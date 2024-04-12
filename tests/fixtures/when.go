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
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
	planepkg "github.com/numaproj-labs/numaplane/pkg/client/clientset/versioned/typed/numaplane/v1alpha1"
)

type When struct {
	t             *testing.T
	restConfig    *rest.Config
	kubeClient    kubernetes.Interface
	gitSync       *v1alpha1.GitSync
	gitSyncClient planepkg.GitSyncInterface
	currentCommit string
}

// create new GitSync
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
	if err := w.waitForGitSyncRunning(ctx, w.gitSyncClient, w.gitSync.Name, defaultTimeout); err != nil {
		w.t.Fatal(err)
	}
	return w
}

// update existing GitSync
func (w *When) UpdateGitSyncAndWait() *When {

	w.t.Helper()
	if w.gitSync == nil {
		w.t.Fatal("No GitSync to create")
	}
	w.t.Log("Updating GitSync", w.gitSync.Name)
	ctx := context.Background()
	i, err := w.gitSyncClient.Update(ctx, w.gitSync, metav1.UpdateOptions{})
	if err != nil {
		w.t.Fatal(err)
	} else {
		w.gitSync = i
	}
	// wait for GitSync to run
	if err := w.waitForGitSyncRunning(ctx, w.gitSyncClient, w.gitSync.Name, defaultTimeout); err != nil {
		w.t.Fatal(err)
	}
	return w
}

// delete existing GitSync
func (w *When) DeleteGitSyncAndWait() *When {

	w.t.Helper()
	if w.gitSync == nil {
		w.t.Fatal("No gitsync to delete")
	}
	w.t.Log("Deleting gitsync", w.gitSync.Name)
	ctx := context.Background()
	if err := w.gitSyncClient.Delete(ctx, w.gitSync.Name, metav1.DeleteOptions{}); err != nil {
		w.t.Fatal(err)
	}

	err := w.waitForGitSyncDeleted()
	if apierr.IsNotFound(err) {
		return w
	} else {
		w.t.Fatalf("Error getting gitSync: %v", err)
	}

	return w
}

// make git push to Git server pod
func (w *When) PushToGitRepo(directory string, fileNames []string, remove bool) *When {

	w.t.Log("Adding files to commit to repo..")

	// open local path to cloned git repo
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		w.t.Fatal(err)
	}

	// open worktree
	wt, err := repo.Worktree()
	if err != nil {
		w.t.Fatal(err)
	}

	// dataPath points to commit directory with edited files
	dataPath := filepath.Join("testdata", directory)
	tmpPath := filepath.Join(localPath, w.gitSync.Spec.Path)

	// iterate over files to be added and committed
	for _, fileName := range fileNames {

		if remove {
			_, err = wt.Remove(filepath.Join(w.gitSync.Spec.Path, fileName))
			if err != nil {
				w.t.Fatal(err)
			}
		} else {
			err := CopyFile(filepath.Join(dataPath, fileName), filepath.Join(tmpPath, fileName))
			if err != nil {
				w.t.Fatal(err)
			}
			_, err = wt.Add(w.gitSync.Spec.Path)
			if err != nil {
				w.t.Fatal(err)
			}
		}

	}

	hash, err := wt.Commit("Committing to git server", &git.CommitOptions{})
	if err != nil {
		w.t.Fatal(err)
	}

	// git push to remote
	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       auth,
	})
	if err != nil {
		w.t.Fatal(err)
	}

	// store commit hash
	w.currentCommit = hash.String()

	w.t.Log("Files successfully pushed to repo")

	return w
}

// kubectl apply resource for self healing test
func (w *When) ModifyResource(apiVersion, resourceType, resource, patch string) *When {

	ctx := context.Background()

	w.t.Log("Patching resource..")

	if apiVersion == "v1" {
		result := w.kubeClient.CoreV1().RESTClient().
			Patch(types.MergePatchType).
			Namespace(TargetNamespace).
			Resource(resourceType).
			Name(resource).
			Body([]byte(patch)).
			Do(ctx)
		if result.Error() != nil {
			w.t.Fatalf("Failed to patch resource %s/%s", resourceType, resource)
		}
	} else {
		result := w.kubeClient.CoreV1().RESTClient().Patch(types.MergePatchType).AbsPath(
			fmt.Sprintf("/apis/%s/namespaces/%s/%s/%s",
				apiVersion,
				TargetNamespace,
				resourceType,
				resource)).Body([]byte(patch)).Do(ctx)
		if result.Error() != nil {
			w.t.Fatalf("Failed to patch resource %s/%s", resourceType, resource)
		}
	}

	w.t.Log("Resource successfully patched")

	return w
}

func (w *When) Wait(timeout time.Duration) *When {
	w.t.Helper()
	w.t.Log("Waiting for", timeout.String())
	time.Sleep(timeout)
	w.t.Log("Done waiting")
	return w
}

func (w *When) Given() *Given {
	return &Given{
		t:             w.t,
		gitSync:       w.gitSync,
		restConfig:    w.restConfig,
		kubeClient:    w.kubeClient,
		gitSyncClient: w.gitSyncClient,
		currentCommit: w.currentCommit,
	}
}

func (w *When) Expect() *Expect {
	return &Expect{
		t:             w.t,
		gitSync:       w.gitSync,
		restConfig:    w.restConfig,
		kubeClient:    w.kubeClient,
		gitSyncClient: w.gitSyncClient,
		currentCommit: w.currentCommit,
	}
}

func (w *When) waitForGitSyncRunning(ctx context.Context, gitSyncClient planepkg.GitSyncInterface,
	gitSyncName string, timeout time.Duration) error {

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
				// gitSync is about to start
				if i.Status.Phase == v1alpha1.GitSyncPhasePending {
					w.t.Logf("GitSync %s is in pending state", gitSyncName)
					continue
				}
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

func (w *When) waitForGitSyncDeleted() error {

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			w.t.Fatalf("Timeout after %v waiting for gitSync terminating", defaultTimeout)
		default:
		}
		_, err := w.gitSyncClient.Get(ctx, w.gitSync.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
	}

}
