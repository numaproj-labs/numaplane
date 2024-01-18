package git

import (
	"fmt"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/numaproj-labs/numaplane/internal/kubernetes"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"regexp"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/numaproj-labs/numaplane/api/v1"
)

const messageChanLength = 5

type Message struct {
	Updates bool
	Err     error
}

type GitSyncProcessor struct {
	gitSync     v1.GitSync
	channels    map[string]chan Message
	k8Client    client.Client
	clusterName string
	log         *zap.SugaredLogger
}

var (
	commitSHARegex = regexp.MustCompile("^[0-9A-Fa-f]{40}$")
)

// IsCommitSHA returns whether or not a string is a 40 character SHA-1
func IsCommitSHA(sha string) bool {
	return commitSHARegex.MatchString(sha)
}

// reference can be a branch, a tag, or a commit hash
func cloneRepository(repoUrl string) (*git.Repository, error) {
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:          repoUrl,
		SingleBranch: true,
	})
	if err != nil {
		return nil, fmt.Errorf("error cloning the repository url %s", err)
	}
	return r, nil
}

func (processor *GitSyncProcessor) watchRepo(repo *v1.RepositoryPath, namespace string) error {
	logger := processor.log
	r, err := cloneRepository(repo.RepoUrl)
	if err != nil {
		logger.Errorw("error cloning the repository", "err", err)
		return err
	}
	h, err := r.ResolveRevision(plumbing.Revision(repo.TargetRevision))
	if err != nil {
		logger.Errorw("error resolve the revision", "revision", repo.TargetRevision, "err", err)
		return err
	}

	// ... retrieving the commit object
	commit, err := r.CommitObject(*h)
	if err != nil {
		logger.Errorw("error checkout the commit", "hash", h.String(), "err", err)
		return err
	}

	// ... retrieve the tree from the commit
	tree, err := commit.Tree()
	if err != nil {
		logger.Errorw("error get the commit tree", "err", err)
		return err
	}

	// Locate the tree with the given path
	tree, err = tree.Tree(repo.Path)
	if err != nil {
		logger.Errorw("error locate the path", "err", err)
		return err
	}

	// Read all the files under the path and apply each one respectively.
	return tree.Files().ForEach(func(f *object.File) error {
		logger.Infow("read file", "file_hash", f.Hash, "file_name", f.Name)
		resource, err := f.Contents()
		if err != nil {
			logger.Errorw("cannot get file content", "filename", f.Name, "err", err)
			return err
		}

		// Apply to the change.
		config, err := rest.InClusterConfig()
		if err != nil {
			logger.Errorw("cannot init the cluster config", "err", err)
			return err
		}
		client, err := kubernetes.NewClient(config)
		if err != nil {
			logger.Errorw("cannot new client", "err", err)
			return err
		}
		return client.ApplyResource([]byte(resource), namespace)
	})

	if IsCommitSHA(repo.TargetRevision) {
		// no monitoring
	} else {
		// monitor with intervals
	}
	return nil
}

func NewGitSyncProcessor(gitSync *v1.GitSync, k8client client.Client, clusterName string, log *zap.SugaredLogger) (*GitSyncProcessor, error) {
	channels := make(map[string]chan Message)
	processor := &GitSyncProcessor{
		gitSync:     *gitSync,
		k8Client:    k8client,
		channels:    channels,
		clusterName: clusterName,
		log:         log,
	}
	namespace, err := gitSync.Spec.GetDestinationNamespace(clusterName)
	if err != nil {
		log.Errorw("cannot get the destination namespace", "err", err)
		return nil, err
	}
	for _, repo := range gitSync.Spec.RepositoryPaths {
		gitCh := make(chan Message, messageChanLength)
		channels[repo.Name] = gitCh

		go func(repo *v1.RepositoryPath) {
			err := processor.watchRepo(repo, namespace)
			if err != nil {
				// TODO: Retry on non-fatal errors
				log.Errorw("error watch the repo", "err", err)
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
