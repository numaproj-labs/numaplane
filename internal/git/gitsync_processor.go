package git

import (
	"context"
	"regexp"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	k8Client    client.Client
	clusterName string
}

func cloneRepo(repo *v1.RepositoryPath) (*git.Repository, error) {
	return git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL: repo.RepoUrl,
	})
}

func watchRepo(ctx context.Context, r *git.Repository, gitSync *v1.GitSync, restConfig *rest.Config, k8Client client.Client, repo *v1.RepositoryPath, namespace string) error {
	logger := logging.FromContext(ctx).With("GitSync name", gitSync.Name,
		"RepositoryPath Name", repo.Name)

	// create kubernetes client
	client, err := kubernetes.NewClient(restConfig, logger)
	if err != nil {
		logger.Errorw("cannot create kubernetes client", "err", err)
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
	if err = remote.Fetch(opts); err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	// The revision can be a branch, a tag, or a commit hash
	hash, err := r.ResolveRevision(plumbing.Revision(repo.TargetRevision))
	if err != nil {
		logger.Errorw("error resolving the revision", "revision", repo.TargetRevision, "err", err)
		return err
	}

	namespacedName := types.NamespacedName{
		Namespace: gitSync.Namespace,
		Name:      gitSync.Name,
	}
	gitSync = &v1.GitSync{}
	if err = k8Client.Get(ctx, namespacedName, gitSync); err != nil {
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
		// TODO: call getCommitTreeAtPath() here instead
		// Retrieving the commit object matching the hash.
		commit, err := r.CommitObject(*hash)
		if err != nil {
			logger.Errorw("error checkout the commit", "hash", hash.String(), "err", err)
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
				logger.Errorw("error locate the path", "repository path", repo.Path, "err", err)
				return err
			}
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
			return client.ApplyResource([]byte(manifest), namespace)
		})
		if err != nil {
			return err
		}

		return updateCommitStatus(ctx, k8Client, namespacedName, hash.String(), repo, logger)
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
		// TODO: monitoring with intervals
		logger.Debug("monitoring with intervals")
	}
	return nil
}

func updateCommitStatus(
	ctx context.Context, k8Client client.Client,
	namespacedName types.NamespacedName,
	hash string,
	repo *v1.RepositoryPath,
	logger *zap.SugaredLogger,
) error {

	commitStatus := v1.CommitStatus{
		Hash:     hash,
		Synced:   true,
		SyncTime: metav1.NewTime(time.Now()),
	}

	gitSync := &v1.GitSync{}
	if err := k8Client.Get(ctx, namespacedName, gitSync); err != nil {
		// if we aren't able to do a Get, then either it's been deleted in the past, or something else went wrong
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			logger.Errorw("Unable to get GitSync", "err", err)
			return err
		}
	}
	if gitSync.Status.CommitStatus == nil {
		gitSync.Status.CommitStatus = make(map[string]v1.CommitStatus)
	}
	gitSync.Status.CommitStatus[repo.Name] = commitStatus
	// It's Ok to fail here as upon errors the whole process will be retried
	// until a CommitStatus is persisted.
	if err := k8Client.Status().Update(ctx, gitSync); err != nil {
		logger.Errorw("Error Updating GitSync Status", "err", err)
		return err
	}
	return nil
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
			r, err := cloneRepo(repo)
			if err != nil {
				logger.Errorw("error cloning the repo", "err", err)
			} else {
				for ok := true; ok; {
					err = watchRepo(ctx, r, gitSync, config, k8client, repo, namespace)
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

func (processor *GitSyncProcessor) Update(gitSync *v1.GitSync) error {
	return nil
}

func (processor *GitSyncProcessor) Shutdown() error {
	return nil
}
