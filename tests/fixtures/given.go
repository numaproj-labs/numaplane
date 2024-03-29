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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
	planepkg "github.com/numaproj-labs/numaplane/pkg/client/clientset/versioned/typed/numaplane/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

var (
	auth = &http.BasicAuth{
		Username: "root",
		Password: "root",
	}
	tempPath = "./tmp"
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
	g.addE2ELabel()
	return g
}

func (g *Given) WithGitSync(gs *v1alpha1.GitSync) *Given {
	g.t.Helper()
	g.gitSync = gs
	g.addE2ELabel()
	return g
}

func (g *Given) addE2ELabel() {
	l := g.gitSync.GetLabels()
	if l == nil {
		l = map[string]string{}
	}
	l[E2ELabel] = E2ELabelValue
	g.gitSync.SetLabels(l)
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

// initialize git repo with all Numaflow files
func (g *Given) CloneGitRepo() *Given {

	ctx := context.Background()

	// open path to git server
	// an example path would be http://localhost:8080/git/repo1.git
	repo, err := g.cloneRepo(ctx)
	if err != nil {
		g.t.Fatal(err)
	}

	tmpPath := filepath.Join("tmp", g.gitSync.Spec.Path)
	dataPath := filepath.Join("testdata", g.gitSync.Spec.Path)

	// create dir for gitsync path
	_ = os.Mkdir(tmpPath, 0777)

	dir, err := os.ReadDir(dataPath)
	if err != nil {
		g.t.Fatal(err)
	}

	// assumes no nested directory for now
	// TODO: account for nested directories
	for _, file := range dir {
		name := file.Name()
		err := CopyFile(filepath.Join(dataPath, name), filepath.Join(tmpPath, name))
		if err != nil {
			g.t.Fatal(err)
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		g.t.Fatal(err)
	}

	_, err = wt.Add(g.gitSync.Spec.Path)
	if err != nil {
		g.t.Fatal(err)
	}

	_, err = wt.Commit("Initial commit", &git.CommitOptions{})
	if err != nil {
		g.t.Fatal(err)
	}

	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       auth,
	})
	if err != nil {
		g.t.Fatal(err)
	}

	return g
}

// clone repository unless it's already been cloned
func (g *Given) cloneRepo(ctx context.Context) (*git.Repository, error) {

	cloneOpts := git.CloneOptions{URL: g.gitSync.Spec.RepoUrl, Auth: auth}

	repo, err := git.PlainCloneContext(ctx, tempPath, false, &cloneOpts)
	if err != nil && errors.Is(err, git.ErrRepositoryAlreadyExists) {
		existingRepo, openErr := git.PlainOpen(tempPath)
		if openErr != nil {
			return repo, fmt.Errorf("failed to open existing repo: %v", openErr)
		}
		return existingRepo, nil
	}

	return repo, nil

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
