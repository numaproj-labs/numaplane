package git

import (
	"context"
	"regexp"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"k8s.io/client-go/rest"

	v1 "github.com/numaproj-labs/numaplane/api/v1"
	"github.com/numaproj-labs/numaplane/internal/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/shared/logging"
)

const messageChanLength = 5

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
	clusterName string
}

func watchRepo(ctx context.Context, restConfig *rest.Config, repo *v1.RepositoryPath, namespace string) error {
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
		logger.Errorw("error cloning the repository", "err", err)
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
		logger.Errorw("error resolving the revision", "revision", repo.TargetRevision, "err", err)
		return err
	}

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
		// TODO: monitoring with intervals
		logger.Debug("monitoring with intervals")
	}
	return nil
}

func NewGitSyncProcessor(ctx context.Context, gitSync *v1.GitSync, config *rest.Config, clusterName string) (*GitSyncProcessor, error) {
	logger := logging.FromContext(ctx)

	channels := make(map[string]chan Message)
	namespace := gitSync.Spec.GetDestinationNamespace(clusterName)
	processor := &GitSyncProcessor{
		gitSync:     *gitSync,
		channels:    channels,
		clusterName: clusterName,
	}

	for _, repo := range gitSync.Spec.RepositoryPaths {
		gitCh := make(chan Message, messageChanLength)
		channels[repo.Name] = gitCh
		go func(repo *v1.RepositoryPath) {
			err := watchRepo(ctx, config, repo, namespace)
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
