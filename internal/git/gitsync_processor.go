package git

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
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
	gitSync     v1alpha1.GitSync
	channels    map[string]chan Message
	kubeClient  kubernetes.Client
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

func cloneRepo(repo *v1alpha1.RepositoryPath) (*git.Repository, error) {
	return git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL: repo.RepoUrl,
	})
}

func watchRepo(ctx context.Context, r *git.Repository, gitSync *v1alpha1.GitSync, kubeClient kubernetes.Client, repo *v1alpha1.RepositoryPath, namespace string) error {
	logger := logging.FromContext(ctx).With("GitSync name", gitSync.Name,
		"RepositoryPath Name", repo.Name)
	var lastCommitHash string

	// Fetch all remote branches
	err := fetchUpdates(r)
	if err != nil {
		return err
	}

	// The revision can be a branch, a tag, or a commit hash
	hash, err := getLatestCommitHash(r, repo.TargetRevision)
	if err != nil {
		logger.Errorw("error resolving the revision", "revision", repo.TargetRevision, "err", err, "repo", repo.RepoUrl)
		return err
	}
	namespacedName := types.NamespacedName{
		Namespace: gitSync.Namespace,
		Name:      gitSync.Name,
	}
	gitSync = &v1alpha1.GitSync{}
	if err = kubeClient.Get(ctx, namespacedName, gitSync); err != nil {
		// if we aren't able to do a Get, then either it's been deleted in the past, or something else went wrong
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			logger.Errorw("Unable to get GitSync", "err", err)
			return err
		}
	}
	val, ok := gitSync.Status.CommitStatus[repo.Name]
	// Only create the resources for the first time if not created yet.
	// Otherwise, monitoring with intervals.
	if !ok {
		// Retrieving the commit object matching the hash.
		tree, err := getCommitTreeAtPath(r, repo.Path, *hash)
		if err != nil {
			return err
		}
		// Read all the files under the path and apply each one respectively.
		err = tree.Files().ForEach(func(f *object.File) error {
			logger.Debugw("read file", "file_name", f.Name)
			// TODO: this currently assumes that one file contains just one manifest - modify for multiple
			manifest, err := f.Contents()
			if err != nil {
				logger.Errorw("cannot get file content", "filename", f.Name, "err", err)
				return err
			}

			// Apply manifest in cluster
			return kubeClient.ApplyResource([]byte(manifest), namespace)
		})
		if err != nil {
			return err
		}

		err = updateCommitStatus(ctx, kubeClient, namespacedName, hash.String(), repo, logger)
		if err != nil {
			return err
		}

		lastCommitHash = hash.String()
	} else {
		lastCommitHash = gitSync.Status.CommitStatus[repo.Name].Hash
	}
	// no monitoring if targetRevision is a specific commit hash
	if isCommitSHA(repo.TargetRevision) {
		if val.Hash != hash.String() {
			logger.Errorw(
				"synced commit hash doesn't match desired one",
				"synced commit hash", val.Hash,
				"desired commit hash", hash.String())
			return err
		}
	} else {
		ticker := time.NewTicker(timeInterval * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				logger.Debug("checking for new updates")
				patchedResources, recentHash, err := CheckForRepoUpdates(ctx, r, repo, lastCommitHash, namespace)
				if err != nil {
					return err
				}
				// recentHash would be an empty string if there is no update in the  repository
				if len(recentHash) > 0 {
					lastCommitHash = recentHash
					err = ApplyPatchToResources(patchedResources, kubeClient)
					if err != nil {
						return err
					}
					err = updateCommitStatus(ctx, kubeClient, namespacedName, recentHash, repo, logger)
					if err != nil {
						return err
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

// ApplyPatchToResources applies changes to Kubernetes resources based on the provided PatchedResource.
// It uses a Kubernetes client to apply changes to each resource and handles creation and deletion of resources as needed.

func ApplyPatchToResources(patchedResources PatchedResource, kubeClient kubernetes.Client) error {
	for key, afterValue := range patchedResources.After {
		namespace := strings.Split(key, "/")[0]
		if beforeValue, ok := patchedResources.Before[key]; ok {
			if beforeValue != afterValue {
				err := kubeClient.ApplyResource([]byte(afterValue), namespace)
				if err != nil {
					return errors.New("failed to apply resource: " + err.Error())
				}
			}
		} else {
			// Handle newly added resources
			err := kubeClient.ApplyResource([]byte(afterValue), namespace)
			if err != nil {
				return errors.New("failed to apply new resource: " + err.Error())
			}
		}
	}

	// Handle deleted resources
	for key, beforeValue := range patchedResources.Before {
		resource, err := yamlUnmarshal(beforeValue)
		if err != nil {
			return errors.New("failed to unmarshal YAML: " + err.Error())
		}
		if _, ok := patchedResources.After[key]; !ok {
			err := kubeClient.DeleteResource(resource.Kind, resource.Metadata.Name, resource.Metadata.Namespace, metav1.DeleteOptions{})
			if err != nil {
				return errors.New("failed to delete resource: " + err.Error())
			}
		}
	}
	return nil
}

// this check for file changes in the repo by comparing the old commit  hash with new commit hash and returns the recent hash with PatchedResources map

func CheckForRepoUpdates(ctx context.Context, r *git.Repository, repo *v1alpha1.RepositoryPath, lastCommitHash string, defaultNameSpace string) (PatchedResource, string, error) {
	var patchedResources PatchedResource
	var recentHash string
	logger := logging.FromContext(ctx).With("RepositoryPath Name", repo.Name, "Repository Url", repo.RepoUrl)
	if err := fetchUpdates(r); err != nil {
		logger.Errorw("error checking for updates in the github repo", "err", err)
		return patchedResources, recentHash, err
	}
	remoteRef, err := getLatestCommitHash(r, repo.TargetRevision)
	if err != nil {
		logger.Errorw("failed to get latest commits in the github repo", "err", err)
		return patchedResources, recentHash, err
	}
	if remoteRef.String() != lastCommitHash {
		recentHash = remoteRef.String()
		lastTreeForThePath, err := getCommitTreeAtPath(r, repo.Path, plumbing.NewHash(lastCommitHash))
		if err != nil {
			logger.Errorw("failed to get last commit", "err", err)
			return patchedResources, recentHash, err
		}

		recentTreeForThePath, err := getCommitTreeAtPath(r, repo.Path, *remoteRef)

		if err != nil {
			logger.Errorw("failed to get recent commit", "err", err)
			return patchedResources, recentHash, err
		}
		// get the diff between the previous and current
		difference, err := lastTreeForThePath.Patch(recentTreeForThePath)
		if err != nil {
			logger.Errorw("failed to take diff of commit", "err", err)
			return patchedResources, recentHash, err
		}

		beforeMap := make(map[string]string)
		afterMap := make(map[string]string)
		//if file exists in both [from] and [to] its modified ,if it exists in [to] its newly added and if it only exists in [from] its deleted
		for _, filePatch := range difference.FilePatches() {
			from, to := filePatch.Files()
			if from != nil {
				initialContent, err := getBlobFileContents(r, from)
				if err != nil {
					logger.Errorw("failed to get  initial content", "err", err)
					return patchedResources, recentHash, err
				}
				err = populateResourceMap(initialContent, beforeMap, defaultNameSpace)
				if err != nil {
					logger.Errorw("failed to populate resource map", "err", err)
					return PatchedResource{}, recentHash, err
				}
			}
			if to != nil {
				finalContent, err := getBlobFileContents(r, to)
				if err != nil {
					logger.Errorw("failed to get  final content", "err", err)

					return patchedResources, recentHash, err
				}
				err = populateResourceMap(finalContent, afterMap, defaultNameSpace)
				if err != nil {
					logger.Errorw("failed to populate resource map", "err", err)
					return PatchedResource{}, recentHash, err
				}
			}
		}
		patchedResources.Before = beforeMap
		patchedResources.After = afterMap
	}
	return patchedResources, recentHash, nil
}

// populateResourceMap fills the resourceMap with resource names as keys and their string representations as values.
func populateResourceMap(content []byte, resourceMap map[string]string, defaultNameSpace string) error {
	// split the string by ---
	resources := strings.Split(string(content), "---")
	for _, v := range resources {
		name, err := getResourceName(v, defaultNameSpace)
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
func getResourceName(yamlContent string, defaultNameSpace string) (string, error) {
	resource, err := yamlUnmarshal(yamlContent)
	if err != nil {
		return "", err
	}
	namespace := resource.Metadata.Namespace
	if namespace == "" {
		namespace = defaultNameSpace
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
	if !isRootDir(path) {
		commitTreeForPath, err := commitTree.Tree(path)
		if err != nil {
			return nil, err
		}
		return commitTreeForPath, nil
	}
	return commitTree, nil
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

// fetchUpdates fetches all the remote branches, returning nil if already up-to-date or an error otherwise.
func fetchUpdates(repo *git.Repository) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return err
	}
	opts := &git.FetchOptions{
		RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
	}
	if err = remote.Fetch(opts); err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}
	return nil
}

func updateCommitStatus(
	ctx context.Context, kubeClient kubernetes.Client,
	namespacedName types.NamespacedName,
	hash string,
	repo *v1alpha1.RepositoryPath,
	logger *zap.SugaredLogger,
) error {

	commitStatus := v1alpha1.CommitStatus{
		Hash:     hash,
		Synced:   true,
		SyncTime: metav1.NewTime(time.Now()),
	}

	gitSync := &v1alpha1.GitSync{}
	if err := kubeClient.Get(ctx, namespacedName, gitSync); err != nil {
		// if we aren't able to do a Get, then either it's been deleted in the past, or something else went wrong
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			logger.Errorw("Unable to get GitSync", "err", err)
			return err
		}
	}
	if gitSync.Status.CommitStatus == nil {
		gitSync.Status.CommitStatus = make(map[string]v1alpha1.CommitStatus)
	}
	gitSync.Status.CommitStatus[repo.Name] = commitStatus
	// It's Ok to fail here as upon errors the whole process will be retried
	// until a CommitStatus is persisted.
	if err := kubeClient.StatusUpdate(ctx, gitSync); err != nil {
		logger.Errorw("Error Updating GitSync Status", "err", err)
		return err
	}
	return nil
}

// getLatestCommitHash retrieves the latest commit hash of a given branch or tag
func getLatestCommitHash(repo *git.Repository, refName string) (*plumbing.Hash, error) {
	commitHash, err := repo.ResolveRevision(plumbing.Revision(refName))
	if err != nil {
		return nil, err
	}
	return commitHash, err
}

func NewGitSyncProcessor(ctx context.Context, gitSync *v1alpha1.GitSync, kubeClient kubernetes.Client, clusterName string) (*GitSyncProcessor, error) {
	logger := logging.FromContext(ctx)
	channels := make(map[string]chan Message)
	namespace := gitSync.Spec.GetDestinationNamespace(clusterName)
	processor := &GitSyncProcessor{
		gitSync:     *gitSync,
		kubeClient:  kubeClient,
		channels:    channels,
		clusterName: clusterName,
	}
	for _, repo := range gitSync.Spec.RepositoryPaths {
		gitCh := make(chan Message, messageChanLength)
		channels[repo.Name] = gitCh
		go func(repo *v1alpha1.RepositoryPath) {

			r, err := cloneRepo(repo)
			if err != nil {
				logger.Errorw("error cloning the repo", "err", err)
			} else {
				for ok := true; ok; {
					err = watchRepo(ctx, r, gitSync, kubeClient, repo, namespace)
					// TODO: terminate the goroutine on fatal errors from 'watchRepo'
					if err != nil {
						logger.Errorw("error watching the repo", "err", err)
					}
				}
			}
		}(&repo)
	}

	return processor, nil
}

func (processor *GitSyncProcessor) Update(gitSync *v1alpha1.GitSync) error {
	return nil
}

func (processor *GitSyncProcessor) Shutdown() error {
	return nil
}
