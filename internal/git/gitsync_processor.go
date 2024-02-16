package git

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	controllerconfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/shared/logging"
)

const (
	messageChanLength = 5
)

type Message struct {
	Updates bool
	Err     error
}

type GitSyncProcessor struct {
	gitSync     v1alpha1.GitSync
	channel     chan Message
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

func cloneRepo(ctx context.Context, path string, repo *v1alpha1.RepositoryPath) (*git.Repository, error) {
	endpoint, err := transport.NewEndpoint(repo.RepoUrl)
	if err != nil {
		return nil, err
	}

	r, err := git.PlainCloneContext(ctx, path, false, &git.CloneOptions{
		URL: endpoint.String(),
	})
	if err != nil && errors.Is(err, git.ErrRepositoryAlreadyExists) {
		// If repository is already present in local, then pull the latest changes and update it.
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

// NewGitSyncProcessor creates a new instance of GitSyncProcessor.
// It takes in a context, a GitSync object, a Kubernetes client, and a cluster name as parameters.
// The function clones the Git repository specified in the GitSync object and starts a goroutine to watch the repository for changes.
// The goroutine will continuously monitor the repository and apply any changes to the Kubernetes cluster.
// If any error occurs during the repository watching process, it will be logged and the commit status will be updated with the error.
func NewGitSyncProcessor(ctx context.Context, gitSync *v1alpha1.GitSync, clusterName string, repoCred map[string]*controllerconfig.GitCredential) (*GitSyncProcessor, error) {
	logger := logging.FromContext(ctx)

	_ = gitSync.Spec.GetDestinationNamespace(clusterName)
	repo := gitSync.Spec.RepositoryPath
	channel := make(chan Message, messageChanLength)
	processor := &GitSyncProcessor{
		gitSync:     *gitSync,
		channel:     channel,
		clusterName: clusterName,
	}
	localRepoPath := getLocalRepoPath(gitSync.GetName())
	go func(repo *v1alpha1.RepositoryPath) {
		_, err := cloneRepo(ctx, localRepoPath, repo)
		if err != nil {
			logger.Errorw("error cloning the repo", "err", err)
		}
	}(&repo)

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
