package git

import (
	"fmt"
	"log"

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
}

func checkRevision(r *git.Repository, revision string) (string, error) {
	hash, err := r.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return "", err
	}

	// Check if it's a  tag
	refs, err := r.References()
	if err != nil {
		return "", err
	}
	found := false
	refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsTag() && ref.Name().Short() == revision {
			found = true
			return nil
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found {
		return "tag", nil
	}
	// Check if it's a branch
	iter, err := r.Branches()
	if err != nil {
		return "", err
	}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Hash() == *hash {
			return nil
		}
		return fmt.Errorf("not a branch")
	})
	if err == nil {
		return "branch", nil
	}
	// If it's neither a tag nor a branch, it must be a commit hash
	return "commit-hash", nil
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

func watchRepo(repo *v1.RepositoryPath) {
	r, err := cloneRepository(repo.RepoUrl)
	if err != nil {
		log.Fatalf("error cloning the repository %s", err.Error())
	}
	// check the TargetRevision is hash ,branch or tag
	revision, err := checkRevision(r, repo.TargetRevision)
	if err != nil {
		log.Fatalf("error in checking revision %s", err.Error())
		return
	}

	switch revision {
	case "branch":
		// monitor the branch
	case "tag":
	// monitor the tag for any change in commit

	case "commit-hash":
		// no monitoring

	}

}

func NewGitSyncProcessor(gitSync *v1.GitSync, k8client client.Client, clusterName string) (*GitSyncProcessor, error) {
	channels := make(map[string]chan Message)
	for _, repo := range gitSync.Spec.RepositoryPaths {
		gitCh := make(chan Message, messageChanLength)
		channels[repo.Name] = gitCh
		go watchRepo(&repo)
	}
	return &GitSyncProcessor{
		gitSync:     *gitSync,
		k8Client:    k8client,
		channels:    channels,
		clusterName: clusterName,
	}, nil

}

func (processor *GitSyncProcessor) Update(gitSync *v1.GitSync) error {
	return nil
}

func (processor *GitSyncProcessor) Shutdown() error {
	return nil
}
