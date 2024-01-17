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

func checkRevision(repoPath *v1.RepositoryPath) {
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL: repoPath.RepoUrl,
	})
	if err != nil {
		log.Fatalf("error cloning the repository %s", err.Error())
	}
	// TargetRevision can be a branch, a tag, or a commit hash
	hash, err := r.ResolveRevision(plumbing.Revision(repoPath.TargetRevision))
	if err != nil {
		log.Fatalf("error resolving revision %s", err.Error())
	}
	w, err := r.Worktree()
	if err != nil {
		log.Fatalf("error getting repository worktree %s", err.Error())
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	})
	if err != nil {
		log.Fatalf("error checking out to the revision %s", err.Error())
	}

	if isCommitSHA(repoPath.TargetRevision) {
		log.Println("No monitoring")
	} else {
		log.Println("monitoring")
	}
}

func NewGitSyncProcessor(gitSync *v1.GitSync, k8client client.Client, clusterName string) (*GitSyncProcessor, error) {
	channels := make(map[string]chan Message)
	for _, repo := range gitSync.Spec.RepositoryPaths {
		gitCh := make(chan Message, messageChanLength)
		channels[repo.Name] = gitCh
		go checkRevision(&repo)
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
