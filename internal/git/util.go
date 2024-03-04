package git

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	"github.com/numaproj-labs/numaplane/internal/kustomize"
	"github.com/numaproj-labs/numaplane/internal/shared/logging"

	controllerConfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	gitShared "github.com/numaproj-labs/numaplane/internal/shared/git"
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
	return cloneRepo(ctx, gitSync, client, cloneOptions, namespace)
}

// GetLatestManifests gets the latest manifests from the Git repository.
// TODO: fetches updates from the remote repository, checks the latest
//
//	commit hash against the stored hash in the GitSync object.
func GetLatestManifests(
	ctx context.Context,
	r *git.Repository,
	client k8sClient.Client,
	gitSync *v1alpha1.GitSync,
	namespace string,
) ([]string, error) {
	logger := logging.FromContext(ctx).With("GitSync name", gitSync.Name, "repo", gitSync.Spec.RepoUrl)
	manifests := make([]string, 0)

	// Fetch all remote branches
	err := fetchUpdates(ctx, client, gitSync, r, namespace)
	if err != nil {
		return manifests, err
	}

	// The revision can be a branch, a tag, or a commit hash
	hash, err := getLatestCommitHash(r, gitSync.Spec.TargetRevision)
	if err != nil {
		logger.Errorw("error resolving the revision", "revision", gitSync.Spec.TargetRevision, "err", err, "repo", gitSync.Spec.RepoUrl)
		return manifests, err
	}

	// Only create the resources for the first time if not created yet.
	// Otherwise, monitoring with intervals.

	// kustomizePath will be the path of clone repository + the path where kustomize file is present.
	localRepoPath := getLocalRepoPath(gitSync.GetName())
	kustomizePath := localRepoPath + "/" + gitSync.Spec.Path
	k := kustomize.NewKustomizeApp(kustomizePath, gitSync.Spec.RepoUrl, os.Getenv("KUSTOMIZE_BINARY_PATH"))

	if kustomize.IsKustomizationRepository(kustomizePath) {
		m, err := k.Build(nil)
		if err != nil {
			logger.Errorw("cannot build kustomize yaml", "err", err)
			return manifests, err
		}
		manifests = append(manifests, m...)
	} else {
		// Retrieving the commit object matching the hash.
		tree, err := getCommitTreeAtPath(r, gitSync.Spec.Path, *hash)
		if err != nil {
			return manifests, err
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
			manifests = append(manifests, manifest)
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

// fetchUpdates fetches all the remote branches and updates the local changes, returning nil if already up-to-date or an error otherwise.
func fetchUpdates(ctx context.Context,
	client k8sClient.Client,
	gitSync *v1alpha1.GitSync, repo *git.Repository, namespace string) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return err
	}

	globalConfig, err := controllerConfig.GetConfigManagerInstance().GetConfig()
	if err != nil {
		return err
	}

	credentials := gitShared.FindCredByUrl(gitSync.Spec.RepoUrl, globalConfig)

	fetchOptions, err := gitShared.GetRepoFetchOptions(ctx, credentials, client, namespace, gitSync.Spec.RepoUrl)
	if err != nil {
		return err
	}

	err = remote.Fetch(fetchOptions)
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
	client k8sClient.Client,
	options *git.CloneOptions,
	namespace string,
) (*git.Repository, error) {
	path := getLocalRepoPath(gitSync.GetName())

	r, err := git.PlainCloneContext(ctx, path, false, options)
	if err != nil && errors.Is(err, git.ErrRepositoryAlreadyExists) {
		// If the repository is already present in local, then pull the latest changes and update it.
		existingRepo, openErr := git.PlainOpen(path)
		if openErr != nil {
			return r, fmt.Errorf("failed to open existing repo, err: %v", openErr)
		}
		if fetchErr := fetchUpdates(ctx, client, gitSync, existingRepo, namespace); fetchErr != nil {
			return existingRepo, fetchErr
		}
		return existingRepo, nil
	}

	return r, err
}
