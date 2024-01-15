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

func cloneRepository(repoUrl string, reference string) (*git.Repository, error) {
	if isValidTag(reference) {
		reference = fmt.Sprintf("refs/tags/%s", reference)
	}
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:           repoUrl,
		SingleBranch:  true,
		ReferenceName: plumbing.ReferenceName(reference),
	})
	if err != nil {
		return nil, fmt.Errorf("error cloning the repository url %s", err)
	}
	return r, nil
}

func watchRepo(repoUrl string, reference string) {
	_, err := cloneRepository(repoUrl, reference)
	if err != nil {
		log.Fatalf("error cloning the repository %s", err.Error())
	}
}

func NewGitSyncProcessor(gitSync *v1.GitSync, k8client client.Client, clusterName string) (*GitSyncProcessor, error) {
	channels := make(map[string]chan Message)
	for _, repo := range gitSync.Spec.RepositoryPaths {
		gitCh := make(chan Message, messageChanLength)
		channels[repo.Name] = gitCh
		go watchRepo(repo.RepoUrl, repo.TargetRevision)
	}
	return &GitSyncProcessor{
		gitSync:     *gitSync,
		k8Client:    k8client,
		channels:    channels,
		clusterName: clusterName,
	}, nil

}
