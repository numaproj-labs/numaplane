package git

import (
	"log"
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

func isCommitSHA(revision string) bool {
	match, _ := regexp.MatchString("^[0-9a-fA-F]{40}$", revision)
	return match
}

type GitSyncProcessor struct {
	gitSync     v1.GitSync
	channels    map[string]chan Message
	k8Client    client.Client
	clusterName string
}

func cloneRepo(repoPath *v1.RepositoryPath) {
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL: repoPath.RepoUrl,
	})
	if err != nil {
		log.Fatalf("error cloning the repository %s", err.Error())
	}
	// TargetRevision can be a branch, a tag, or a commit hash
	err = checkRevision(r, repoPath.TargetRevision)
	if err != nil {
		log.Fatalf("error  checking revision for the repository %s", err.Error())

	}
}

func checkRevision(r *git.Repository, revision string) error {
	hash, err := r.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return err
	}
	w, err := r.Worktree()
	if err != nil {
		return err
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	})
	if err != nil {
		return err
	}

	if isCommitSHA(revision) {
		log.Println("No monitoring")
	} else {
		log.Println("monitoring")
	}
	return nil
}

func NewGitSyncProcessor(gitSync *v1.GitSync, k8client client.Client, clusterName string) (*GitSyncProcessor, error) {
	channels := make(map[string]chan Message)
	for _, repo := range gitSync.Spec.RepositoryPaths {
		gitCh := make(chan Message, messageChanLength)
		channels[repo.Name] = gitCh
		go cloneRepo(&repo)
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
