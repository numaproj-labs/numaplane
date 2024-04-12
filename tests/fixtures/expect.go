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

	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
	planepkg "github.com/numaproj-labs/numaplane/pkg/client/clientset/versioned/typed/numaplane/v1alpha1"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Expect struct {
	t             *testing.T
	restConfig    *rest.Config
	kubeClient    kubernetes.Interface
	gitSync       *v1alpha1.GitSync
	gitSyncClient planepkg.GitSyncInterface
	currentCommit string
}

// check that resources are created / exist
func (e *Expect) ResourcesExist(apiVersion, resourceType string, resources []string) *Expect {

	e.t.Helper()
	e.t.Log("Verifying that resources exist..")

	for _, r := range resources {
		if !e.doesExist(apiVersion, resourceType, r) {
			e.t.Fatalf("Resource %s/%s does not exist", resourceType, r)
		}
		e.t.Logf("Resource %s/%s exists", resourceType, r)
	}

	return e
}

// check that resources have been deleted or never existed
func (e *Expect) ResourcesDontExist(apiVersion, resourceType string, resources []string) *Expect {

	e.t.Helper()
	e.t.Log("Verifying that resources no longer exist..")

	for _, r := range resources {
		if !e.doesNotExist(apiVersion, resourceType, r) {
			e.t.Fatalf("Resource %s/%s not deleted", resourceType, r)
		}
		e.t.Logf("Resource %s/%s doesn't exist", resourceType, r)
	}

	return e
}

// verify value of resource spec to determine if change occurred or not as expected
func (e *Expect) VerifyResourceState(apiVersion, resourceType, resource, field, key string, value interface{}) *Expect {

	e.t.Helper()
	e.t.Log("Verifying resource state is as expected..")

	var (
		err  error
		body []byte
	)

	if !e.doesExist(apiVersion, resourceType, resource) {
		e.t.Fatalf("Resource %s/%s does not exist", resourceType, resource)
	}

	ctx := context.Background()

	if apiVersion == "v1" {
		body, err = e.kubeClient.CoreV1().RESTClient().
			Get().
			Namespace(TargetNamespace).
			Resource(resourceType).
			Name(resource).
			DoRaw(ctx)
	} else {
		body, err = e.kubeClient.CoreV1().RESTClient().Get().AbsPath(
			fmt.Sprintf("/apis/%s/namespaces/%s/%s/%s",
				apiVersion,
				TargetNamespace,
				resourceType,
				resource)).DoRaw(ctx)
	}

	if err != nil {
		e.t.Fatalf("Failed to verify resource %s/%s state", resourceType, resource)
	}

	object := make(map[string]interface{})
	err = yaml.Unmarshal(body, object)
	if err != nil {
		e.t.Fatalf("Failed to verify resource %s/%s state", resourceType, resource)
	}

	for k, v := range object[field].(map[interface{}]interface{}) {
		if k == key {
			if v != value {
				e.t.Fatalf("Resource %s/%s state is not as expected", resourceType, resource)
			}
		}
	}

	e.t.Log("Resource state is as expected")

	return e
}

// verify that GitSync's commitStatus is as expected:
// synced = true
// hash is equal to the last commit to the Git server
func (e *Expect) CheckCommitStatus() *Expect {

	e.t.Helper()
	e.t.Log("Verifying GitSync's commitStatus is as expected..")

	// gitSync commit status will be nil if reconciliation has not finished
	commitStatus, err := e.getCommitStatus()
	if err != nil {
		e.t.Fatalf("Can't find GitSync %s", e.gitSync.Name)
	}

	if commitStatus.Hash != e.currentCommit {
		e.t.Fatalf("GitSync %s is not synced to the most recent commit", e.gitSync.Name)
	}

	e.t.Logf("GitSync %s is synced to current commit", e.gitSync.Name)

	return e
}

func (e *Expect) When() *When {
	return &When{
		t:             e.t,
		gitSync:       e.gitSync,
		restConfig:    e.restConfig,
		kubeClient:    e.kubeClient,
		gitSyncClient: e.gitSyncClient,
		currentCommit: e.currentCommit,
	}
}

func (e *Expect) getCommitStatus() (*v1alpha1.CommitStatus, error) {

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			e.t.Logf("Timeout verifying that GitSync %s is synced", e.gitSync.Name)
		default:
		}
		gitSync, err := e.gitSyncClient.Get(ctx, e.gitSync.Name, v1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if gitSync.Status.CommitStatus != nil && gitSync.Status.CommitStatus.Synced {
			return gitSync.Status.CommitStatus, nil
		}
		time.Sleep(2 * time.Second)
	}

}

// doesnt exist
func (e *Expect) doesNotExist(apiVersion, resourceType, resource string) bool {

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			e.t.Logf("Timeout verifying that resource %s deleted", resource)
			return false
		default:
		}
		if err := e.getResource(apiVersion, resourceType, resource, ctx); err != nil {
			if errors.IsNotFound(err) {
				return true
			}
		}
		time.Sleep(2 * time.Second)
	}

}

func (e *Expect) doesExist(apiVersion, resourceType, resource string) bool {

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			e.t.Logf("Timeout verifying that resource %s exists", resource)
			return false
		default:
		}
		if e.getResource(apiVersion, resourceType, resource, ctx) == nil {
			return true
		}
		time.Sleep(2 * time.Second)
	}

}

func (e *Expect) getResource(apiVersion, resourceType, resource string, ctx context.Context) error {

	if apiVersion == "v1" {
		result := e.kubeClient.CoreV1().RESTClient().
			Get().
			Namespace(TargetNamespace).
			Resource(resourceType).
			Name(resource).
			Do(ctx)
		if result.Error() != nil {
			return result.Error()
		}
	} else {
		result := e.kubeClient.CoreV1().RESTClient().Get().AbsPath(
			fmt.Sprintf("/apis/%s/namespaces/%s/%s/%s",
				apiVersion,
				TargetNamespace,
				resourceType,
				resource)).Do(ctx)
		if result.Error() != nil {
			return result.Error()
		}
	}

	return nil
}
