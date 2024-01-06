/*
Copyright 2023 The Numaproj Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package git

import (
	"errors"
	"log"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/numaproj-labs/numaplane/api/v1"
)

type Message struct {
	Updates bool
	Err     error
}

type GitSyncProcessor struct {
	gitSync  v1.GitSync
	k8Client client.Client
	channels []chan Message
}

func checkForUpdates(r *git.Repository) (bool, error) {
	err := r.Fetch(&git.FetchOptions{
		RemoteName: "origin",
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return false, err
	}
	// Get local and remote references
	localRef, err := r.Head()
	if err != nil {
		return false, err
	}
	remoteRef, err := r.Reference(plumbing.NewRemoteReferenceName("origin", "master"), true)
	if err != nil {
		return false, err
	}
	// Compare the references
	if localRef.Hash() != remoteRef.Hash() {
		return true, nil
	}
	return false, nil
}

func WatchRepo(repoPath v1.RepositoryPath, ch chan Message) {
	// clone the repository
	repo, err := git.PlainClone(repoPath.Path, false, &git.CloneOptions{
		URL: repoPath.RepoUrl,
	})
	if err != nil {
		log.Fatalf("error in cloning the repository %s", err)
	}

	// start monitoring the repository
	ticker := time.NewTicker(3 * time.Minute)
	for range ticker.C {
		updates, err := checkForUpdates(repo)
		if err != nil {
			log.Printf("error checking for updates in the github repo %s", err)
			// send the updates over channel
			ch <- Message{Updates: updates, Err: err}
		}

	}
}

func NewGitSyncProcessor(gitSync *v1.GitSync, k8client client.Client) (*GitSyncProcessor, error) {
	var channels []chan Message
	for _, repo := range gitSync.Spec.RepositoryPaths {
		gitCh := make(chan Message)
		channels = append(channels, gitCh)
		go WatchRepo(repo, gitCh)
	}
	return &GitSyncProcessor{
		gitSync:  *gitSync,
		k8Client: k8client,
		channels: channels,
	}, nil

}

func (processor *GitSyncProcessor) Create(gitSync v1.GitSync) error {
	// creating   a new k8 resource
	return nil
}

func (processor *GitSyncProcessor) Update(gitSync v1.GitSync) error {
	for _, ch := range processor.channels {
		select {
		case msg := <-ch:
			if msg.Err != nil {
				log.Printf("Error watching repository: %s", msg.Err)
			} else if msg.Updates {
				//  apply  any necessary changes to the Kubernetes resources.
			}
		default:
		}
	}
	return nil
}

func (processor *GitSyncProcessor) Shutdown() error {
	for _, ch := range processor.channels {
		close(ch)
	}
	return nil
}
