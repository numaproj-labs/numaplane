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
	"testing"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type When struct {
	t          *testing.T
	restConfig *rest.Config
	kubeClient kubernetes.Interface
	gitSync    *v1alpha1.GitSync

	// is this needed if we port forward only git pod?
	portForwarderStopChannels map[string]chan struct{}
}

func (w *When) CreateGitSync() *When {

	w.t.Helper()
	if w.gitSync == nil {
		w.t.Fatal("No GitSync to create")
	}
	w.t.Log("Creating GitSync", w.gitSync.Name)
	// ctx := context.Background()
	// i, err := w.kubeClient.CoreV1().RESTClient().Post()

	return w
}
func (w *When) DeleteGitSync() *When {

	return w
}

func (w *When) Given() *Given {
	return &Given{
		t:          w.t,
		gitSync:    w.gitSync,
		restConfig: w.restConfig,
		kubeClient: w.kubeClient,
	}
}

func (w *When) Expect() *Expect {
	return &Expect{
		t:          w.t,
		gitSync:    w.gitSync,
		restConfig: w.restConfig,
		kubeClient: w.kubeClient,
	}
}
