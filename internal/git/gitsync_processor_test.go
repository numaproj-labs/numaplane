package git

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	v1 "github.com/numaproj-labs/numaplane/api/v1"
	"github.com/numaproj-labs/numaplane/tests/utils"
)

const (
	fileNameToBeWatched = "k8.yaml"
	path                = "config"
	remoteRepo          = "temp/remote"
	localRepo           = "temp/local"
	defaultNameSpace    = "numaflowtest"
)

var kubernetesYamlString = `apiVersion: v1
kind: Service
metadata:
  name: my-nginx-svc
  labels:
    app: nginx
spec:
  type: LoadBalancer
  ports:
  - port: 80
  selector:
    app: nginx
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-nginx
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
`

func Test_watchRepo(t *testing.T) {
	testCases := []struct {
		name     string
		repo     v1.RepositoryPath
		hasError bool
	}{
		// TODO: Need to have kubernetes fake client for enabling these test cases.
		//{
		//	name: "`main` as a TargetRevision",
		//	repo: v1.RepositoryPath{
		//		RepoUrl:        "https://github.com/numaproj-labs/numaplane.git",
		//		Path:           "config/samples",
		//		TargetRevision: "main",
		//	},
		//	hasError: false,
		//},
		//{
		//	name: "tag name as a TargetRevision",
		//	repo: v1.RepositoryPath{
		//		RepoUrl:        "https://github.com/go-git/go-git.git",
		//		Path:           ".github",
		//		TargetRevision: "v5.5.1",
		//	},
		//	hasError: false,
		//},
		//{
		//	name: "commit hash as a TargetRevision",
		//	repo: v1.RepositoryPath{
		//		RepoUrl:        "https://github.com/numaproj-labs/numaplane.git",
		//		Path:           "config/samples",
		//		TargetRevision: "8ed04cc87c3ab8f5d7d459f82e587da5a9192d20",
		//	},
		//	hasError: false,
		//},
		//{
		//	name: "remote branch name as a TargetRevision",
		//	repo: v1.RepositoryPath{
		//		RepoUrl:        "https://github.com/git-fixtures/basic.git",
		//		Path:           "go",
		//		TargetRevision: "refs/remotes/origin/branch",
		//	},
		//	hasError: false,
		//},
		//{
		//	name: "local branch name as a TargetRevision",
		//	repo: v1.RepositoryPath{
		//		RepoUrl:        "https://github.com/git-fixtures/basic.git",
		//		Path:           "go",
		//		TargetRevision: "branch",
		//	},
		//	hasError: false,
		//},
		//{
		//	name: "root path",
		//	repo: v1.RepositoryPath{
		//		RepoUrl:        "https://github.com/git-fixtures/basic.git",
		//		Path:           "",
		//		TargetRevision: "branch",
		//	},
		//	hasError: false,
		//},
		{
			name: "invalid repo",
			repo: v1.RepositoryPath{
				RepoUrl:        "https://invalid_repo.git",
				Path:           "path",
				TargetRevision: "main",
			},
			hasError: true,
		},
		{
			name: "unresolvable TargetRevision",
			repo: v1.RepositoryPath{
				RepoUrl:        "https://github.com/numaproj-labs/numaplane.git",
				Path:           "config/samples",
				TargetRevision: "unresolvable",
			},
			hasError: true,
		},
		{
			name: "invalid path",
			repo: v1.RepositoryPath{
				RepoUrl:        "https://github.com/numaproj-labs/numaplane.git",
				Path:           "invalid_path",
				TargetRevision: "main",
			},
			hasError: true,
		},
	}
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	gitSync := v1.GitSync{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       v1.GitSyncSpec{},
		Status:     v1.GitSyncStatus{},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := watchRepo(ctx, utils.GetTestRestConfig(), &gitSync, &tc.repo, "")
			if tc.hasError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func updateFileInBranch(repo *git.Repository, branchName, fileName, content string, author *object.Signature, tagName string) error {
	branchRef, err := repo.Reference(plumbing.NewBranchReferenceName(branchName), true)
	if err != nil {
		return fmt.Errorf("could not find branch: %v", err.Error())
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("could not get working tree: %v", err.Error())
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: branchRef.Name(),
		Force:  true,
	})
	if err != nil {
		return fmt.Errorf("could not checkout branch: %v", err)
	}

	filePath := w.Filesystem.Join(localRepo, fileName)

	// Read the existing content from the file
	existingContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not read existing file: %v", err)
	}
	combinedContent := append(existingContent, []byte(content)...)
	err = os.WriteFile(filePath, combinedContent, 0644)
	if err != nil {
		return fmt.Errorf("could not write to file: %v", err)
	}

	_, err = w.Add(fileName)
	if err != nil {
		return fmt.Errorf("could not stage file: %v", err)
	}

	commit, err := w.Commit("Update file "+fileName, &git.CommitOptions{
		Author: author,
	})
	if err != nil {
		return fmt.Errorf("could not commit changes: %v", err)
	}

	_, err = repo.CreateTag(tagName, commit, &git.CreateTagOptions{
		Message: "Tag for " + fileName,
		Tagger:  author,
	})
	if err != nil {
		return fmt.Errorf("could not create tag: %v", err)
	}

	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(branchRef.Name() + ":" + branchRef.Name()),
			config.RefSpec("refs/tags/" + tagName + ":refs/tags/" + tagName),
		},
	})
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			return fmt.Errorf("already up-to-date %s", err.Error())
		} else {
			return fmt.Errorf("could not push to remote: %v", err)
		}
	}
	return nil
}

func getCommitHashAndRepo() (*git.Repository, string, error) {
	_, err := git.Init(filesystem.NewStorage(osfs.New(remoteRepo), cache.NewObjectLRUDefault()), nil)
	if err != nil {
		log.Println("error initializing remote", err)
		return nil, "", err
	}

	r, err := git.PlainInit(localRepo, false)
	if err != nil {
		log.Println("error initializing local", err)

		return nil, "", err
	}

	absolutePath, err := filepath.Abs(remoteRepo)
	if err != nil {
		log.Println("error getting absolute path ", err)

		return nil, "", err
	}

	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{fmt.Sprintf("file://%s", absolutePath)},
	})
	if err != nil {
		log.Println("error creating remote", err)

		return nil, "", err
	}

	w, err := r.Worktree()
	if err != nil {
		log.Println("error getting work tree", err)
		return nil, "", err
	}

	directory := filepath.Join(localRepo, "config")
	filename := filepath.Join(directory, "k8.yaml")

	err = os.MkdirAll(directory, 0755)
	if err != nil {
		log.Println("error creating directory", err)
		return nil, "", err
	}
	err = os.WriteFile(filename, []byte(""), 0644)
	if err != nil {
		log.Println(err)

		return nil, "", err
	}

	// Add the file and commit
	_, err = w.Add("config/k8.yaml")
	if err != nil {
		log.Println("error adding file", err)

		return nil, "", err
	}
	commit, err := w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		log.Println(err)

		return nil, "", err
	}

	log.Println(commit)
	// push to main branch in remote
	branch := "master"
	branchRe := fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)
	err = r.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(branchRe),
		},
	})
	if err != nil {
		log.Println("error pushing to remote", err)

		return nil, "", err
	}

	h, err := r.ResolveRevision("master")
	if err != nil {
		log.Println("reference resolving issue", err)

		return nil, "", err
	}

	return r, h.String(), nil
}

func TestGetLatestCommit(t *testing.T) {
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:          "https://github.com/shubhamdixit863/testingrepo",
		SingleBranch: true,
	})
	assert.Nil(t, err)
	commit, err := getLatestCommit(r, "main")
	assert.Nil(t, err)
	log.Println(commit.String())
	assert.Equal(t, 40, len(commit.String()))
}

func TestCheckForRepoUpdatesBranch(t *testing.T) {
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	randomFloat := rng.Float64()
	roundedFloat := math.Round(randomFloat*100) / 100
	tag := fmt.Sprintf("v%f", roundedFloat)
	r, lastCommitHash, err := getCommitHashAndRepo()
	assert.Nil(t, err)

	signature := &object.Signature{
		Name:  "test",
		Email: "test@test.com",
		When:  time.Now(),
	}
	err = updateFileInBranch(r, "master", fmt.Sprintf("%s/%s", path, fileNameToBeWatched), kubernetesYamlString, signature, tag)
	assert.Nil(t, err)
	absolutePath, err := filepath.Abs(remoteRepo)
	assert.Nil(t, err)
	path := &v1.RepositoryPath{
		Name:           "test",
		RepoUrl:        fmt.Sprintf("file:///%s", absolutePath),
		Path:           "config",
		TargetRevision: "master",
	}

	commitStatus := make(map[string]v1.CommitStatus)
	commitStatus["test"] = v1.CommitStatus{
		Hash:     lastCommitHash,
		Synced:   true,
		SyncTime: metav1.Time{},
		Error:    "",
	}
	status := &v1.GitSyncStatus{
		Phase:        "",
		Conditions:   nil,
		Message:      "",
		CommitStatus: commitStatus,
	}

	patchedContent, err := CheckForRepoUpdates(context.Background(), r, path, status, defaultNameSpace)
	assert.Nil(t, err)
	assert.Equal(t, kubernetesYamlString, fmt.Sprintf("%s---%s", patchedContent.After[fmt.Sprintf("%s/my-nginx-svc", defaultNameSpace)], patchedContent.After[fmt.Sprintf("%s/my-nginx", defaultNameSpace)]))

	err = os.RemoveAll("temp")
	assert.Nil(t, err)

}

func TestCheckForRepoUpdatesVersion(t *testing.T) {
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	randomFloat := rng.Float64()
	roundedFloat := math.Round(randomFloat*100) / 100
	tag := fmt.Sprintf("v%f", roundedFloat)
	r, lastCommitHash, err := getCommitHashAndRepo()
	assert.Nil(t, err)

	signature := &object.Signature{
		Name:  "test",
		Email: "test@test.com",
		When:  time.Now(),
	}
	err = updateFileInBranch(r, "master", fmt.Sprintf("%s/%s", path, fileNameToBeWatched), kubernetesYamlString, signature, tag)
	assert.Nil(t, err)
	absolutePath, err := filepath.Abs(remoteRepo)

	assert.Nil(t, err)

	path := &v1.RepositoryPath{
		Name:           "test",
		RepoUrl:        fmt.Sprintf("file:///%s", absolutePath),
		Path:           "config",
		TargetRevision: tag,
	}

	commitStatus := make(map[string]v1.CommitStatus)
	commitStatus["test"] = v1.CommitStatus{
		Hash:     lastCommitHash,
		Synced:   true,
		SyncTime: metav1.Time{},
		Error:    "",
	}
	status := &v1.GitSyncStatus{
		Phase:        "",
		Conditions:   nil,
		Message:      "",
		CommitStatus: commitStatus,
	}

	patchedContent, err := CheckForRepoUpdates(context.Background(), r, path, status, defaultNameSpace)

	assert.Nil(t, err)
	assert.Equal(t, kubernetesYamlString, fmt.Sprintf("%s---%s", patchedContent.After[fmt.Sprintf("%s/my-nginx-svc", defaultNameSpace)], patchedContent.After[fmt.Sprintf("%s/my-nginx", defaultNameSpace)]))
	err = os.RemoveAll("temp")
	assert.Nil(t, err)

}

func TestPopulateResourceMap(t *testing.T) {
	testCases := []struct {
		name      string
		resources []string
		expected  map[string]string
	}{
		{
			name: "Basic test",
			resources: []string{
				`apiVersion: v1
kind: Pod
metadata:
  name: frontend
  namespace: numaflow
spec:
  containers:
    - name: app
      image: images.my-company.example/app:v4
      resources:
        requests:
          memory: "64Mi"
          cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"
    - name: log-aggregator
      image: images.my-company.example/log-aggregator:v6
      resources:
        requests:
          memory: "64Mi"
          cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"`,
				`apiVersion: v1
kind: Service
metadata:
  name: my-service
  namespace: numaflow`,
			},
			expected: map[string]string{"numaflow/frontend": `apiVersion: v1
kind: Pod
metadata:
  name: frontend
  namespace: numaflow
spec:
  containers:
    - name: app
      image: images.my-company.example/app:v4
      resources:
        requests:
          memory: "64Mi"
          cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"
    - name: log-aggregator
      image: images.my-company.example/log-aggregator:v6
      resources:
        requests:
          memory: "64Mi"
          cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"`, "numaflow/my-service": `apiVersion: v1
kind: Service
metadata:
  name: my-service
  namespace: numaflow`},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resourceMap := make(map[string]string)
			for _, re := range tc.resources {
				err := populateResourceMap([]byte(re), resourceMap, defaultNameSpace)
				assert.Nil(t, err)
			}

			assert.Equal(t, resourceMap, tc.expected)
		})
	}
}

func TestGetResourceName(t *testing.T) {
	testCases := []struct {
		name     string
		yaml     string
		expected string
	}{
		{
			name: "Pod with namespace",
			yaml: `apiVersion: v1
kind: Pod
metadata:
  name: frontend
  namespace: production`,
			expected: "production/frontend",
		},
		{
			name: "Service without namespace",
			yaml: `apiVersion: v1
kind: Service
metadata:
  name: my-service`,
			expected: fmt.Sprintf("%s/my-service", defaultNameSpace),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resourceName, err := getResourceName(tc.yaml, defaultNameSpace)
			assert.Nil(t, err)
			assert.Equal(t, tc.expected, resourceName, "Extracted resource name should match expected")
		})
	}
}
