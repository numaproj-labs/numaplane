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
	"os"
	"strings"
	"testing"

	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
	planepkg "github.com/numaproj-labs/numaplane/pkg/client/clientset/versioned/typed/numaplane/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

type Given struct {
	t             *testing.T
	restConfig    *rest.Config
	kubeClient    kubernetes.Interface
	gitSyncClient planepkg.GitSyncInterface
	gitSync       *v1alpha1.GitSync
}

// create GitSync using raw YAML or @filename
func (g *Given) GitSync(text string) *Given {
	g.t.Helper()
	g.gitSync = &v1alpha1.GitSync{}
	g.readResource(text, g.gitSync)
	l := g.gitSync.GetLabels()
	if l == nil {
		l = map[string]string{}
	}
	l[Label] = LabelValue
	g.gitSync.SetLabels(l)
	return g
}

func (g *Given) WithGitSync(gs *v1alpha1.GitSync) *Given {
	g.t.Helper()
	g.gitSync = gs
	l := g.gitSync.GetLabels()
	if l == nil {
		l = map[string]string{}
	}
	l[Label] = LabelValue
	g.gitSync.SetLabels(l)
	return g
}

// helper func to read and unmarshal GitSync YAML into object
func (g *Given) readResource(text string, v metav1.Object) {
	g.t.Helper()
	var file string
	if strings.HasPrefix(text, "@") {
		file = strings.TrimPrefix(text, "@")
	} else {
		f, err := os.CreateTemp("", "numaplane-e2e")
		if err != nil {
			g.t.Fatal(err)
		}
		_, err = f.Write([]byte(text))
		if err != nil {
			g.t.Fatal(err)
		}
		err = f.Close()
		if err != nil {
			g.t.Fatal(err)
		}
		file = f.Name()
	}

	f, err := os.ReadFile(file)
	if err != nil {
		g.t.Fatal(err)
	}
	err = yaml.Unmarshal(f, v)
	if err != nil {
		g.t.Fatal(err)
	}
}

func (g *Given) When() *When {
	return &When{
		t:             g.t,
		gitSync:       g.gitSync,
		restConfig:    g.restConfig,
		kubeClient:    g.kubeClient,
		gitSyncClient: g.gitSyncClient,
	}
}
