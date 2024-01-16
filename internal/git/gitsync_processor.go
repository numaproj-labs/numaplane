package git

import (
	"fmt"
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

type GitSyncProcessor struct {
	gitSync     v1.GitSync
	channels    map[string]chan Message
	k8Client    client.Client
	clusterName string
}

func isValidTag(tag string) bool {
	match, _ := regexp.MatchString(`^[vV]?\d+\.\d+\.\d+$`, tag)
	return match
}
func isCommitHash(hash string) bool {
	match, _ := regexp.MatchString("^[0-9a-fA-F]{40}$", hash)
	return match
}

// reference can be a branch, a tag, or a commit hash
func cloneRepository(repoUrl string, reference string) (*git.Repository, error) {
	var referenceName plumbing.ReferenceName
	if isValidTag(reference) {
		referenceName = plumbing.ReferenceName(fmt.Sprintf("refs/tags/%s", reference))
	} else if isCommitHash(reference) {
		referenceName = plumbing.NewHashReference("", plumbing.NewHash(reference)).Name()
	} else {

		referenceName = plumbing.ReferenceName(reference)
	}
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:           repoUrl,
		SingleBranch:  true,
		ReferenceName: referenceName,
	})
	if err != nil {
		return nil, fmt.Errorf("error cloning the repository url %s", err)
	}
	return r, nil
}

func watchRepo(repo *v1.RepositoryPath) {
	_, err := cloneRepository(repo.RepoUrl, repo.TargetRevision)
	if err != nil {
		log.Fatalf("error cloning the repository %s", err.Error())
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
