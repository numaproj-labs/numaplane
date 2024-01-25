package git

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/go-git/go-git/v5/plumbing/format/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/numaproj-labs/numaplane/api/v1"
	"github.com/numaproj-labs/numaplane/internal/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/shared/logging"
)

const (
	messageChanLength = 5
	timeInterval      = 3
)

type Message struct {
	Updates bool
	Err     error
}

var commitSHARegex = regexp.MustCompile("^[0-9A-Fa-f]{40}$")

// isCommitSHA returns whether a string is a 40 character SHA-1
func isCommitSHA(sha string) bool {
	return commitSHARegex.MatchString(sha)
}

// isRootDir returns whether this given path represents root directory of a repo,
// We consider empty string as the root.
func isRootDir(path string) bool {
	return len(path) == 0
}

type GitSyncProcessor struct {
	gitSync     v1.GitSync
	channels    map[string]chan Message
	k8Client    client.Client
	clusterName string
}

// stores the Patched files  and their data

type PatchedResource struct {
	Before map[string]string
	After  map[string]string
}

// Kubernetes Yaml Resource structs

type KubernetesResource struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   MetaData `yaml:"metadata"`
}
type MetaData struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

func watchRepo(ctx context.Context, restConfig *rest.Config, gitSync *v1.GitSync, repo *v1.RepositoryPath, namespace string) error {
	logger := logging.FromContext(ctx)

	// create kubernetes client
	k8sClient, err := kubernetes.NewClient(restConfig, logger)
	if err != nil {
		logger.Errorw("cannot create kubernetes client", "err", err)
		return err
	}

	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:          repo.RepoUrl,
		SingleBranch: false,
	})
	if err != nil {
		logger.Errorw("error cloning the repository", "err", err, "repo", repo.RepoUrl)
		return err
	}

	// Fetch all remote branches
	remote, err := r.Remote("origin")
	if err != nil {
		return err
	}
	opts := &git.FetchOptions{
		RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
	}
	if err = remote.Fetch(opts); err != nil {
		return err
	}

	// The revision can be a branch, a tag, or a commit hash
	h, err := r.ResolveRevision(plumbing.Revision(repo.TargetRevision))
	if err != nil {
		logger.Errorw("error resolving the revision", "revision", repo.TargetRevision, "err", err, "repo", repo.RepoUrl)
		return err
	}
	// TODO save the commit hash in the gitSync status

	// Retrieving the commit object matching the hash.
	commit, err := r.CommitObject(*h)
	if err != nil {
		logger.Errorw("error checkout the commit", "hash", h.String(), "err", err)
		return err
	}

	// Retrieve the tree from the commit.
	tree, err := commit.Tree()
	if err != nil {
		logger.Errorw("error get the commit tree", "err", err)
		return err
	}

	if !isRootDir(repo.Path) {
		// Locate the tree with the given path.
		tree, err = tree.Tree(repo.Path)
		if err != nil {
			logger.Errorw("error locate the path", "err", err)
			return err
		}
	}

	// Read all the files under the path and apply each one respectively.
	err = tree.Files().ForEach(func(f *object.File) error {
		logger.Debugw("read file", "file_name", f.Name)
		//TODO: this currently assumes that one file contains just one manifest - modify for multiple
		manifest, err := f.Contents()
		if err != nil {
			logger.Errorw("cannot get file content", "filename", f.Name, "err", err)
			return err
		}

		// Apply manifest in cluster
		return k8sClient.ApplyResource([]byte(manifest), namespace)
	})
	if err != nil {
		return err
	}

	if isCommitSHA(repo.TargetRevision) {
		// TODO: no monitoring
		logger.Debug("no monitoring")

	} else {
		ticker := time.NewTicker(timeInterval * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				patchedResources, err := CheckForRepoUpdates(r, repo, &gitSync.Status, ctx)
				if err != nil {
					return err
				}

				for key, afterValue := range patchedResources.After {
					namespace := strings.Split(key, "/")[0]
					if beforeValue, ok := patchedResources.Before[key]; ok {
						if beforeValue != afterValue {
							err := k8sClient.ApplyResource([]byte(afterValue), namespace)
							if err != nil {
								return err
							}
						}
					} else {
						// Newly Added resource
						err := k8sClient.ApplyResource([]byte(afterValue), namespace)
						if err != nil {
							return err
						}
					}
				}
				// deleted resources
				for key, beforeValue := range patchedResources.Before {
					resource, err := yamlUnmarshal(beforeValue)
					if err != nil {
						return err
					}
					if _, ok := patchedResources.After[key]; !ok {
						err := k8sClient.DeleteResource(resource.Kind, resource.Metadata.Name, resource.Metadata.Namespace, metav1.DeleteOptions{})
						if err != nil {
							return err
						}
					}
				}

			case <-ctx.Done():
				logger.Debug("Context canceled, stopping updates check")
				return nil
			}
		}
	}
	return nil
}

// this check for file changes in the repo by comparing the old commit  hash with new commit hash

func CheckForRepoUpdates(r *git.Repository, repo *v1.RepositoryPath, status *v1.GitSyncStatus, ctx context.Context) (PatchedResource, error) {
	var patchedResources PatchedResource
	logger := logging.FromContext(ctx)
	if err := fetchUpdates(r); err != nil {
		logger.Errorw("error checking for updates in the github repo", "err", err, "repo", repo.RepoUrl)
		return patchedResources, err
	}
	remoteRef, err := getLatestCommit(r, repo.TargetRevision)
	if err != nil {
		logger.Errorw("failed to get latest commits in the github repo", "err", err, "repo", repo.RepoUrl)
		return patchedResources, err
	}
	lastCommitStatus := status.CommitStatus[repo.Name]
	if remoteRef.String() != lastCommitStatus.Hash {

		status.CommitStatus[repo.Name] = v1.CommitStatus{
			Hash:     remoteRef.String(),
			Synced:   true,
			SyncTime: metav1.Time{Time: time.Now()},
			Error:    "",
		}

		lastTreeForThePath, err := getCommitTreeAtPath(r, repo.Path, plumbing.NewHash(lastCommitStatus.Hash))
		if err != nil {
			logger.Errorw("failed to  get last commit", "err", err, "repo", repo.RepoUrl)
			return patchedResources, err
		}

		recentTreeForThePath, err := getCommitTreeAtPath(r, repo.Path, *remoteRef)

		if err != nil {
			logger.Errorw("failed to  recent commit", "err", err, "repo", repo.RepoUrl)
			return patchedResources, err
		}

		patch, err := lastTreeForThePath.Patch(recentTreeForThePath)
		if err != nil {
			logger.Errorw("failed to patch commit", "err", err, "repo", repo.RepoUrl)
			return patchedResources, err
		}

		beforeMap := make(map[string]string)
		afterMap := make(map[string]string)
		//if file exists in both [from] and [to] its modified ,if it exists in [to] its newly added and if it only exists in [from] its deleted
		for _, filePatch := range patch.FilePatches() {
			from, to := filePatch.Files()
			if from != nil {
				initialContent, err := getBlobFileContents(r, from)
				if err != nil {
					logger.Errorw("failed to get  initial content", "err", err.Error(), "repo", repo.RepoUrl)
					return patchedResources, err
				}
				err = populateResourceMap(initialContent, beforeMap)
				if err != nil {
					logger.Errorw("failed to populate resource map", "err", err.Error(), "repo", repo.RepoUrl)
					return PatchedResource{}, err
				}
			}
			if to != nil {
				finalContent, err := getBlobFileContents(r, to)
				if err != nil {
					logger.Errorw("failed to get  final content", "err", err.Error(), "repo", repo.RepoUrl)

					return patchedResources, err
				}
				err = populateResourceMap(finalContent, afterMap)
				if err != nil {
					logger.Errorw("failed to populate resource map", "err", err.Error(), "repo", repo.RepoUrl)
					return PatchedResource{}, err
				}
			}
		}

		patchedResources.Before = beforeMap
		patchedResources.After = afterMap
	}

	return patchedResources, nil
}

// populateResourceMap fills the resourceMap with resource names as keys and their string representations as values.
func populateResourceMap(content []byte, resourceMap map[string]string) error {
	// split the string by ---
	resources := strings.Split(string(content), "---")
	for _, v := range resources {
		name, err := getResourceName(v)
		if err != nil {
			return err
		}
		resourceMap[name] = v
	}
	return nil
}

// unmarshalls yaml into Kubernetes Resource
func yamlUnmarshal(yamlContent string) (KubernetesResource, error) {
	var resource KubernetesResource
	err := yaml.Unmarshal([]byte(yamlContent), &resource)
	if err != nil {
		return KubernetesResource{}, err
	}
	return resource, err
}

// getResourceName extracts the name and namespace of the Kubernetes resource from YAML content.
func getResourceName(yamlContent string) (string, error) {
	resource, err := yamlUnmarshal(yamlContent)
	if err != nil {
		return "", err
	}
	namespace := resource.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}
	return namespace + "/" + resource.Metadata.Name, nil
}

// retrieves a specific tree (or subtree) located at a given path within a specific commit in a Git repository
func getCommitTreeAtPath(r *git.Repository, path string, hash plumbing.Hash) (*object.Tree, error) {
	commit, err := r.CommitObject(hash)
	if err != nil {
		return nil, err
	}
	commitTree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	commitTreeForPath, err := commitTree.Tree(path)
	if err != nil {
		return nil, err
	}
	return commitTreeForPath, nil
}

// gets the file content from the repository with file hash
func getBlobFileContents(r *git.Repository, file diff.File) ([]byte, error) {
	fileBlob, err := r.BlobObject(file.Hash())
	if err != nil {
		return nil, err
	}
	reader, err := fileBlob.Reader()
	if err != nil {
		return nil, err
	}
	fileContent, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return fileContent, nil
}

// fetchUpdates fetches updates from the 'origin' remote, returning nil if already up-to-date or an error otherwise.
func fetchUpdates(repo *git.Repository) error {
	err := repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}
	return nil
}

// getLatestCommit retrieves the latest commit hash of a given branch or tag
func getLatestCommit(repo *git.Repository, refName string) (*plumbing.Hash, error) {
	commitHash, err := repo.ResolveRevision(plumbing.Revision(refName))
	if err != nil {
		return nil, err
	}
	return commitHash, err
}

func NewGitSyncProcessor(ctx context.Context, gitSync *v1.GitSync, k8client client.Client, config *rest.Config, clusterName string) (*GitSyncProcessor, error) {
	logger := logging.FromContext(ctx)
	channels := make(map[string]chan Message)
	namespace := gitSync.Spec.GetDestinationNamespace(clusterName)
	processor := &GitSyncProcessor{
		gitSync:     *gitSync,
		k8Client:    k8client,
		channels:    channels,
		clusterName: clusterName,
	}
	for _, repo := range gitSync.Spec.RepositoryPaths {
		gitCh := make(chan Message, messageChanLength)
		channels[repo.Name] = gitCh
		go func(repo *v1.RepositoryPath) {
			err := watchRepo(ctx, config, nil, repo, namespace) //
			if err != nil {
				// TODO: Retry on non-fatal errors
				logger.Errorw("error watching the repo", "err", err)
			}
		}(&repo)
	}

	return processor, nil
}

func (processor *GitSyncProcessor) Update(gitSync *v1.GitSync) error {
	return nil
}

func (processor *GitSyncProcessor) Shutdown() error {
	return nil
}
