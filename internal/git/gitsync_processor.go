package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-git/go-git/v5/plumbing/transport"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	controllerconfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/kustomize"
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
	channel     chan Message
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

// this gets secret using the kubernetes client
func getSecret(ctx context.Context, kubeClient kubernetes.Client, namespace, secretName string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	key := k8sClient.ObjectKey{
		Namespace: namespace,
		Name:      secretName,
	}
	if err := kubeClient.Get(ctx, key, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func cloneRepo(ctx context.Context, path string, specs *v1alpha1.GitSyncSpec) (*git.Repository, error) {
	endpoint, err := transport.NewEndpoint(specs.RepoUrl)
	if err != nil {
		return nil, err
	}

	r, err := git.PlainCloneContext(ctx, path, false, &git.CloneOptions{
		URL: endpoint.String(),
	})
	if err != nil && errors.Is(err, git.ErrRepositoryAlreadyExists) {
		// If the repository is already present in local, then pull the latest changes and update it.
		existingRepo, openErr := git.PlainOpen(path)
		if openErr != nil {
			return r, fmt.Errorf("failed to open existing repo, err: %v", openErr)
		}
		if fetchErr := fetchUpdates(existingRepo); fetchErr != nil {
			return existingRepo, fetchErr
		}
		return existingRepo, nil
	}

	return r, err
}

// watchRepo monitors a Git repository for changes. It fetches updates from the remote repository,
// checks the latest commit hash against the stored hash in the GitSync object,
// and applies any changes to the Kubernetes cluster. It also periodically checks for updates based on a ticker.
func watchRepo(ctx context.Context, r *git.Repository, gitSync *v1alpha1.GitSync, kubeClient kubernetes.Client, specs *v1alpha1.GitSyncSpec, namespace, localRepoPath string) (string, error) {
	logger := logging.FromContext(ctx).With("GitSync name", gitSync.Name)
	var lastCommitHash string

	// Fetch all remote branches
	err := fetchUpdates(r)
	if err != nil {
		return "", err
	}

	// The revision can be a branch, a tag, or a commit hash
	hash, err := getLatestCommitHash(r, specs.TargetRevision)
	if err != nil {
		logger.Errorw("error resolving the revision", "revision", specs.TargetRevision, "err", err, "repo", specs.RepoUrl)
		return "", err
	}
	namespacedName := types.NamespacedName{
		Namespace: gitSync.Namespace,
		Name:      gitSync.Name,
	}

	gitSync = &v1alpha1.GitSync{}
	if err = kubeClient.Get(ctx, namespacedName, gitSync); err != nil {
		// if we aren't able to do a Get, then either it's been deleted in the past, or something else went wrong
		if apierrors.IsNotFound(err) {
			return hash.String(), nil
		} else {
			logger.Errorw("Unable to get GitSync", "err", err)
			return hash.String(), err
		}
	}

	// kustomizePath will be the path of clone repository + the path where kustomize file is present.
	kustomizePath := localRepoPath + "/" + specs.Path
	k := kustomize.NewKustomizeApp(kustomizePath, specs.RepoUrl, os.Getenv("KUSTOMIZE_BINARY_PATH"))

	sourceType, err := specs.ExplicitType()
	if err != nil {
		return hash.String(), err
	}

	// Only create the resources for the first time if not created yet.
	// Otherwise, monitoring with intervals.
	if gitSync.Status.CommitStatus == nil || gitSync.Status.CommitStatus.Hash == "" {
		err = runInitialSetup(r, sourceType, kubeClient, specs, k, namespace, hash)
		if err != nil {
			return hash.String(), err
		}

		err = updateCommitStatus(ctx, kubeClient, logger, namespacedName, hash.String(), nil)
		if err != nil {
			return hash.String(), err
		}

		lastCommitHash = hash.String()
	} else {
		lastCommitHash = gitSync.Status.CommitStatus.Hash
	}
	// no monitoring if targetRevision is a specific commit hash
	if isCommitSHA(specs.TargetRevision) {
		if lastCommitHash != hash.String() {
			logger.Errorw(
				"synced commit hash doesn't match desired one",
				"synced commit hash", lastCommitHash,
				"desired commit hash", hash.String())
			return hash.String(), err
		}
	} else {
		ticker := time.NewTicker(timeInterval * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				logger.Debug("checking for new updates")
				var (
					recentHash       string
					patchedResources PatchedResource
				)
				switch sourceType {
				case v1alpha1.SourceTypeHelm:
				case v1alpha1.SourceTypeKustomize:
					patchedResources, recentHash, err = CheckRepoUpdatesForKustomize(ctx, r, specs, lastCommitHash, namespace, k)
					if err != nil {
						return hash.String(), err
					}
				case v1alpha1.SourceTypeRaw:
					patchedResources, recentHash, err = CheckForRepoUpdates(ctx, r, specs, lastCommitHash, namespace)
					if err != nil {
						return hash.String(), err
					}
				}

				// recentHash would be an empty string if there is no update in the  repository
				if len(recentHash) > 0 {
					lastCommitHash = recentHash
					err = ApplyPatchToResources(patchedResources, kubeClient)
					if err != nil {
						return recentHash, err
					}
					err = updateCommitStatus(ctx, kubeClient, logger, namespacedName, recentHash, nil)
					if err != nil {
						return recentHash, err
					}

				}
			case <-ctx.Done():
				logger.Debug("Context canceled, stopping updates check")
				return hash.String(), nil
			}
		}
	}

	return hash.String(), nil
}

// runInitialSetup will do the initial configuration of defined repository.
func runInitialSetup(r *git.Repository, sourceType v1alpha1.SourceType, kubeClient kubernetes.Client, specs *v1alpha1.GitSyncSpec, k kustomize.Kustomize, namespace string, hash *plumbing.Hash) error {
	switch sourceType {
	case v1alpha1.SourceTypeHelm:
	case v1alpha1.SourceTypeKustomize:
		manifests, err := k.Build(nil)
		if err != nil {
			return fmt.Errorf("cannot build kustomize yaml, err: %v", err)
		}

		for _, manifest := range manifests {
			err = kubeClient.ApplyResource([]byte(manifest), namespace)
			if err != nil {
				return err
			}
		}
	case v1alpha1.SourceTypeRaw:
		// Retrieving the commit object matching the hash.
		tree, err := getCommitTreeAtPath(r, specs.Path, *hash)
		if err != nil {
			return err
		}
		// Read all the files under the path and apply each one respectively.
		err = tree.Files().ForEach(func(f *object.File) error {
			// TODO: this currently assumes that one file contains just one manifest - modify for multiple
			manifest, err := f.Contents()
			if err != nil {
				return fmt.Errorf("cannot get file content for filename %s, err: %v", f.Name, err)
			}

			// Apply manifest in cluster
			return kubeClient.ApplyResource([]byte(manifest), namespace)
		})
		if err != nil {
			return err
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

// CheckRepoUpdatesForKustomize will calculate for any update required based on commit history.
func CheckRepoUpdatesForKustomize(ctx context.Context, r *git.Repository, specs *v1alpha1.GitSyncSpec, lastCommitHash, defaultNameSpace string, k kustomize.Kustomize) (PatchedResource, string, error) {
	var patchedResources PatchedResource
	var recentHash string

	logger := logging.FromContext(ctx).With("Repository Url", specs.RepoUrl)
	// Build the kustomize manifest with old changes.
	oldManifest, err := k.Build(nil)
	if err != nil {
		logger.Errorw("error generating kustomize manifest", "err", err)
		return patchedResources, recentHash, err
	}

	if err = fetchUpdates(r); err != nil {
		logger.Errorw("error checking for updates in the github repo", "err", err)
		return patchedResources, recentHash, err
	}

	remoteRef, err := getLatestCommitHash(r, specs.TargetRevision)
	if err != nil {
		logger.Errorw("failed to get latest commits in the github repo", "err", err)
		return patchedResources, recentHash, err
	}

	// If remote commit hash and local one aren't equal, it means there are new changes made in remote repository.
	if remoteRef.String() != lastCommitHash {
		recentHash = remoteRef.String()
		beforeMap := make(map[string]string)
		afterMap := make(map[string]string)

		for _, manifest := range oldManifest {
			err = populateResourceMap(manifest, beforeMap, defaultNameSpace)
			if err != nil {
				logger.Errorw("failed to populate resource map", "err", err)
				return patchedResources, recentHash, err
			}
		}

		newManifest, err := k.Build(nil)
		if err != nil {
			logger.Errorw("error generating kustomize manifest", "err", err)
			return patchedResources, recentHash, err
		}
		for _, manifest := range newManifest {
			err = populateResourceMap(manifest, afterMap, defaultNameSpace)
			if err != nil {
				logger.Errorw("failed to populate resource map", "err", err)
				return patchedResources, recentHash, err
			}
		}

		patchedResources.Before = beforeMap
		patchedResources.After = afterMap
	}

	return patchedResources, recentHash, nil
}

// this check for file changes in the repo by comparing the old commit hash with new commit hash
//and returns the recent hash with PatchedResources map

func CheckForRepoUpdates(ctx context.Context, r *git.Repository, specs *v1alpha1.GitSyncSpec, lastCommitHash string, defaultNameSpace string) (PatchedResource, string, error) {
	var patchedResources PatchedResource
	var recentHash string
	logger := logging.FromContext(ctx).With("Repository Url", specs.RepoUrl)
	if err := fetchUpdates(r); err != nil {
		logger.Errorw("error checking for updates in the github repo", "err", err)
		return patchedResources, recentHash, err
	}
	remoteRef, err := getLatestCommitHash(r, specs.TargetRevision)
	if err != nil {
		logger.Errorw("failed to get latest commits in the github repo", "err", err)
		return patchedResources, recentHash, err
	}
	if remoteRef.String() != lastCommitHash {
		recentHash = remoteRef.String()
		lastTreeForThePath, err := getCommitTreeAtPath(r, specs.Path, plumbing.NewHash(lastCommitHash))
		if err != nil {
			logger.Errorw("failed to get last commit", "err", err)
			return patchedResources, recentHash, err
		}

		recentTreeForThePath, err := getCommitTreeAtPath(r, specs.Path, *remoteRef)

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
				err = populateResourceMap(string(initialContent), beforeMap, defaultNameSpace)
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
				err = populateResourceMap(string(finalContent), afterMap, defaultNameSpace)
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
func populateResourceMap(content string, resourceMap map[string]string, defaultNameSpace string) error {
	// split the string by ---
	resources := strings.Split(content, "---")
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

// fetchUpdates fetches all the remote branches and updates the local changes, returning nil if already up-to-date or an error otherwise.
func fetchUpdates(repo *git.Repository) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return err
	}

	err = remote.Fetch(&git.FetchOptions{
		RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
		Force:    true,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}

	// update the local repo with remote changes.
	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	err = worktree.Pull(&git.PullOptions{
		Force:      true,
		RemoteName: "origin",
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}

	return nil
}

// updateCommitStatus will update the commit status in git sync CR also if error happened while syncing the repo then it update the error reason as well.
func updateCommitStatus(ctx context.Context, kubeClient kubernetes.Client, logger *zap.SugaredLogger, namespacedName types.NamespacedName,
	hash string, err error) error {

	commitStatus := v1alpha1.CommitStatus{
		Hash:     hash,
		Synced:   true,
		SyncTime: metav1.NewTime(time.Now()),
	}
	// set error reason with commit status.
	if err != nil {
		commitStatus.Error = err.Error()
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

	gitSync.Status.CommitStatus = &commitStatus
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

// NewGitSyncProcessor creates a new instance of GitSyncProcessor.
// It takes in a context, a GitSync object, a Kubernetes client, and a cluster name as parameters.
// The function clones the Git repository specified in the GitSync object and starts a goroutine to watch the repository for changes.
// The goroutine will continuously monitor the repository and apply any changes to the Kubernetes cluster.
// If any error occurs during the repository watching process, it will be logged and the commit status will be updated with the error.
func NewGitSyncProcessor(ctx context.Context, gitSync *v1alpha1.GitSync, kubeClient kubernetes.Client, clusterName string, repoCred map[string]*controllerconfig.GitCredential) (*GitSyncProcessor, error) {
	logger := logging.FromContext(ctx)

	namespace := gitSync.Spec.GetDestinationNamespace(clusterName)
	specs := gitSync.Spec
	channel := make(chan Message, messageChanLength)
	processor := &GitSyncProcessor{
		gitSync:     *gitSync,
		kubeClient:  kubeClient,
		channel:     channel,
		clusterName: clusterName,
	}
	localRepoPath := getLocalRepoPath(gitSync.GetName())
	go func(specs *v1alpha1.GitSyncSpec) {
		r, err := cloneRepo(ctx, localRepoPath, specs)
		if err != nil {
			logger.Errorw("error cloning the repo", "err", err)
		} else {
			for ok := true; ok; {
				hash, err := watchRepo(ctx, r, gitSync, kubeClient, specs, namespace, localRepoPath)
				// TODO: terminate the goroutine on fatal errors from 'watchRepo'
				if err != nil {
					// update error reason in git sync CR if happened.
					if updateErr := updateCommitStatus(ctx, kubeClient, logger, types.NamespacedName{Namespace: gitSync.Namespace, Name: gitSync.Name}, hash, err); updateErr != nil {
						logger.Errorw("error updating gitSync commit status", "err", updateErr)
					}
					logger.Errorw("error watching the repo", "err", err)
				}
			}
		}
	}(&specs)

	return processor, nil
}

// getLocalRepoPath will return the local path where repo will be cloned, by default it will use /tmp as base directory
// unless LOCAL_REPO_PATH env is set.
func getLocalRepoPath(gitSyncName string) string {
	baseDir := os.Getenv("LOCAL_REPO_PATH")
	if baseDir != "" {
		return fmt.Sprintf("%s/%s", baseDir, gitSyncName)
	} else {
		return fmt.Sprintf("/tmp/%s", gitSyncName)
	}
}

func (processor *GitSyncProcessor) Update(gitSync *v1alpha1.GitSync) error {
	return nil
}

func (processor *GitSyncProcessor) Shutdown() error {
	return nil
}
