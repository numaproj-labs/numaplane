package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/cespare/xxhash/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"k8s.io/apimachinery/pkg/runtime"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	controllerConfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/helm"
	"github.com/numaproj-labs/numaplane/internal/kustomize"
	gitShared "github.com/numaproj-labs/numaplane/internal/util/git"
	kubernetesshared "github.com/numaproj-labs/numaplane/internal/util/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/util/logging"
)

func CloneRepo(
	ctx context.Context,
	client k8sClient.Client,
	gitSync *v1alpha1.GitSync,
	namespace string,
	globalConfig controllerConfig.GlobalConfig,
) (*git.Repository, error) {
	gitCredentials := gitShared.FindCredByUrl(gitSync.Spec.RepoUrl, globalConfig)
	cloneOptions, err := gitShared.GetRepoCloneOptions(ctx, gitCredentials, client, namespace, gitSync.Spec.RepoUrl)
	if err != nil {
		return nil, fmt.Errorf("error getting  the  clone options: %v", err)
	}
	return cloneRepo(ctx, gitSync, cloneOptions)
}

// GetLatestManifests gets the latest manifests from the Git repository.
// TODO: fetches updates from the remote repository, checks the latest
//
//	commit hash against the stored hash in the GitSync object.
func GetLatestManifests(
	ctx context.Context,
	r *git.Repository,
	gitSync *v1alpha1.GitSync,

) ([]string, error) {
	logger := logging.FromContext(ctx).With("GitSync name", gitSync.Name, "repo", gitSync.Spec.RepoUrl)
	manifests := make([]string, 0)

	// Fetch all remote branches
	err := fetchUpdates(r)
	if err != nil {
		return manifests, err
	}

	// The revision can be a branch, a tag, or a commit hash
	hash, err := getLatestCommitHash(r, gitSync.Spec.TargetRevision)
	if err != nil {
		logger.Errorw("error resolving the revision", "revision", gitSync.Spec.TargetRevision, "err", err, "repo", gitSync.Spec.RepoUrl)
		return manifests, err
	}

	localRepoPath := getLocalRepoPath(gitSync)

	// repoPath will be the path of clone repository + the path while needs to be deployed.
	deployablePath := localRepoPath + "/" + gitSync.Spec.Path
	// validate the deployablePath path for its existence.
	if _, err := os.Stat(deployablePath); err != nil {
		return manifests, fmt.Errorf("invalid deployable path, err: %v", err)
	}

	// initialize the kustomize with KUSTOMIZE_BINARY_PATH if defined.
	k := kustomize.NewKustomizeApp(deployablePath, gitSync.Spec.RepoUrl, os.Getenv("KUSTOMIZE_BINARY_PATH"))

	// initialize the helm with HELM_BINARY_PATH if defined.
	h := helm.New(deployablePath, os.Getenv("HELM_BINARY_PATH"))

	sourceType, err := gitSync.Spec.ExplicitType()
	if err != nil {
		return manifests, err
	}

	switch sourceType {
	case v1alpha1.SourceTypeKustomize:
		generatedManifest, err := k.Build(nil)
		if err != nil {
			logger.Errorw("can not build kustomize yaml", "err", err)
			return manifests, err
		}
		manifestData, err := SplitYAMLToString([]byte(generatedManifest))
		if err != nil {
			return manifests, fmt.Errorf("can not parse kustomize manifest, err: %v", err)
		}
		manifests = append(manifests, manifestData...)
	case v1alpha1.SourceTypeHelm:
		generatedManifest, err := h.Build(gitSync.Name, gitSync.Spec.Destination.Namespace, gitSync.Spec.Helm.Parameters, gitSync.Spec.Helm.ValueFiles)
		if err != nil {
			return manifests, fmt.Errorf("cannot build helm manifest, err: %v", err)
		}
		manifestData, err := SplitYAMLToString([]byte(generatedManifest))
		if err != nil {
			return manifests, fmt.Errorf("can not parse helm manifest, err: %v", err)
		}
		manifests = append(manifests, manifestData...)
	case v1alpha1.SourceTypeRaw:
		// Retrieving the commit object matching the hash.
		tree, err := getCommitTreeAtPath(r, gitSync.Spec.Path, *hash)
		if err != nil {
			return manifests, err
		}
		// Read all the files under the path and apply each one respectively.
		err = tree.Files().ForEach(func(f *object.File) error {
			logger.Debugw("read file", "file_name", f.Name)
			if kubernetesshared.IsValidKubernetesManifestFile(f.Name) {
				manifest, err := f.Contents()
				if err != nil {
					logger.Errorw("cannot get file content", "filename", f.Name, "err", err)
					return err
				}
				manifestData, err := SplitYAMLToString([]byte(manifest))
				if err != nil {
					return fmt.Errorf("can not parse file data, err: %v", err)
				}
				manifests = append(manifests, manifestData...)
			}

			return nil
		})
		if err != nil {
			return manifests, err
		}
	}
	return manifests, nil
}

// getLatestCommitHash retrieves the latest commit hash of a given branch or tag
func getLatestCommitHash(repo *git.Repository, refName string) (*plumbing.Hash, error) {
	commitHash, err := repo.ResolveRevision(plumbing.Revision(refName))
	if err != nil {
		return nil, err
	}
	return commitHash, err
}

// getCommitTreeAtPath retrieves a specific tree (or subtree) located at a given path within a specific commit in a Git repository
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

// isRootDir returns whether this given path represents root directory of a repo,
// We consider empty string as the root.
func isRootDir(path string) bool {
	return len(path) == 0
}

// getLocalRepoPath will return the local path where repo will be cloned,
// by default it will use /tmp as base directory
// unless LOCAL_REPO_PATH env is set.
func getLocalRepoPath(gitSync *v1alpha1.GitSync) string {
	baseDir := os.Getenv("LOCAL_REPO_PATH")
	repoUrl := strconv.FormatUint(xxhash.Sum64([]byte(gitSync.Spec.RepoUrl)), 16)
	if baseDir != "" {
		return fmt.Sprintf("%s/%s/%s", baseDir, gitSync.Name, repoUrl)
	} else {
		return fmt.Sprintf("/tmp/%s/%s", gitSync.Name, repoUrl)
	}
}

// fetchUpdates fetches all the remote branches and updates the local changes,
// returning nil if already up-to-date or an error otherwise.
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

func cloneRepo(
	ctx context.Context,
	gitSync *v1alpha1.GitSync,
	options *git.CloneOptions,
) (*git.Repository, error) {
	path := getLocalRepoPath(gitSync)

	r, err := git.PlainCloneContext(ctx, path, false, options)
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

// SplitYAMLToString splits a YAML file into strings. Returns list of yamls
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
func SplitYAMLToString(yamlData []byte) ([]string, error) {
	d := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlData), 4096)
	var objs []string
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %v", err)
		}
		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}
		objs = append(objs, string(ext.Raw))
	}
	return objs, nil
}
