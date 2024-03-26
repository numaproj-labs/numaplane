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
	"testing"
	"time"

	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
	planepkg "github.com/numaproj-labs/numaplane/pkg/client/clientset/versioned/typed/numaplane/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Expect struct {
	t             *testing.T
	restConfig    *rest.Config
	kubeClient    kubernetes.Interface
	gitSync       *v1alpha1.GitSync
	gitSyncClient planepkg.GitSyncInterface
}

// check that resources are created
func (e *Expect) ResourcesExist(resourceType string, resources []string) *Expect {

	e.t.Helper()
	ctx := context.Background()
	for _, r := range resources {
		result := e.kubeClient.CoreV1().RESTClient().Get().
			Timeout(defaultTimeout).
			Namespace(TargetNamespace).
			Resource(resourceType).
			Name(r).
			Do(ctx)
		if result.Error() != nil {
			e.t.Fatalf("Resource %s does not exist", r)
		}
	}

	return e
}

// check that resources are deleted
func (e *Expect) ResourcesDontExist(resourceType string, resources []string) *Expect {

	e.t.Helper()
	for _, resource := range resources {
		if !e.isDeleted(resourceType, resource) {
			e.t.Fatalf("Resource %s not deleted", resource)
		}
	}

	return e
}

func (e *Expect) When() *When {
	return &When{
		t:             e.t,
		gitSync:       e.gitSync,
		restConfig:    e.restConfig,
		kubeClient:    e.kubeClient,
		gitSyncClient: e.gitSyncClient,
	}
}

func (e *Expect) isDeleted(resourceType, resource string) bool {

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		result := e.kubeClient.CoreV1().RESTClient().Get().
			Namespace(TargetNamespace).
			Resource(resourceType).
			Name(resource).
			Do(ctx)
		if result.Error() != nil {
			if errors.IsNotFound(result.Error()) {
				return true
			} else {
				return false
			}
		}
		time.Sleep(2 * time.Second)
	}

}
