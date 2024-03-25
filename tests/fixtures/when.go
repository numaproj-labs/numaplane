/*
Copyright 2023.

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

package fixtures

import (
	"context"
	"fmt"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
	planepkg "github.com/numaproj-labs/numaplane/pkg/client/clientset/versioned/typed/numaplane/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type When struct {
	t             *testing.T
	restConfig    *rest.Config
	kubeClient    kubernetes.Interface
	gitSync       *v1alpha1.GitSync
	gitSyncClient planepkg.GitSyncInterface
}

func (w *When) CreateGitSyncAndWait() *When {

	w.t.Helper()
	if w.gitSync == nil {
		w.t.Fatal("No GitSync to create")
	}
	w.t.Log("Creating GitSync", w.gitSync.Name)
	ctx := context.Background()
	i, err := w.gitSyncClient.Create(ctx, w.gitSync, metav1.CreateOptions{})
	if err != nil {
		w.t.Fatal(err)
	} else {
		w.gitSync = i
	}
	// wait for GitSync to run
	if err := WaitForGitSyncRunning(ctx, w.gitSyncClient, w.gitSync.Name, defaultTimeout); err != nil {
		w.t.Fatal(err)
	}
	return w

}
func (w *When) DeleteGitSync() *When {

	w.t.Helper()
	if w.gitSync == nil {
		w.t.Fatal("No gitsync to delete")
	}
	w.t.Log("Deleting gitsync", w.gitSync.Name)
	ctx := context.Background()
	if err := w.gitSyncClient.Delete(ctx, w.gitSync.Name, metav1.DeleteOptions{}); err != nil {
		w.t.Fatal(err)
	}

	return w
}

// make git push to Git server pod
func (w *When) PushToGitRepo(repoPath string, files []string) *When {

	// open path to git server
	// an example path would be http://localhost:8080/git/repo1.git
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		w.t.Fatal(err)
	}

	// open worktree
	wt, err := repo.Worktree()
	if err != nil {
		w.t.Fatal(err)
	}

	// iterate over files to be added and committed
	for _, f := range files {
		_, err = wt.Add(f)
		if err != nil {
			w.t.Fatal(err)
		}
	}

	_, err = wt.Commit("Committing to git server", &git.CommitOptions{})
	if err != nil {
		w.t.Fatal(err)
	}

	// git push to remote
	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
	})
	if err != nil {
		w.t.Fatal(err)
	}

	return w
}

func (w *When) Given() *Given {
	return &Given{
		t:             w.t,
		gitSync:       w.gitSync,
		restConfig:    w.restConfig,
		kubeClient:    w.kubeClient,
		gitSyncClient: w.gitSyncClient,
	}
}

func (w *When) Expect() *Expect {
	return &Expect{
		t:             w.t,
		gitSync:       w.gitSync,
		restConfig:    w.restConfig,
		kubeClient:    w.kubeClient,
		gitSyncClient: w.gitSyncClient,
	}
}

func WaitForGitSyncRunning(ctx context.Context, gitSyncClient planepkg.GitSyncInterface, gitSyncName string, timeout time.Duration) error {
	fieldSelector := "metadata.name=" + gitSyncName
	opts := metav1.ListOptions{FieldSelector: fieldSelector}
	watch, err := gitSyncClient.Watch(ctx, opts)
	if err != nil {
		return err
	}
	defer watch.Stop()
	timeoutCh := make(chan bool, 1)
	go func() {
		time.Sleep(timeout)
		timeoutCh <- true
	}()
	for {
		select {
		case event := <-watch.ResultChan():
			i, ok := event.Object.(*v1alpha1.GitSync)
			if ok {
				if i.Status.Phase == v1alpha1.GitSyncPhaseRunning {
					return nil
				}
			} else {
				return fmt.Errorf("not gitsync")
			}
		case <-timeoutCh:
			return fmt.Errorf("timeout after %v waiting for GitSync running", timeout)
		}
	}
}
