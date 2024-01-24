package git

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	v1 "github.com/numaproj-labs/numaplane/api/v1"
	"github.com/numaproj-labs/numaplane/tests/utils"
)

/*

const (
	fileNameToBeWatched = "k8.yaml"
	path                = "config"
)

*/

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

/*
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

		filePath := w.Filesystem.Join("temp", fileName)

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
			Auth: &http.BasicAuth{
				Username: "shubhamdixit863",
				Password: "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",
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
		r, err := git.PlainClone("temp", false, &git.CloneOptions{
			URL:          "https://github.com/shubhamdixit863/testingrepo",
			SingleBranch: true,
		})
		if err != nil {
			return nil, "", err
		}

		// The revision can be a branch, a tag, or a commit hash
		h, err := r.ResolveRevision("main")
		if err != nil {
			return nil, "", err
		}
		return r, h.String(), nil
	}
*/
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

/*
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
		Email: "shubhamdixit863@gmail.com",
		When:  time.Now(),
	}
	err = updateFileInBranch(r, "main", fmt.Sprintf("%s/%s", path, fileNameToBeWatched), "test 12-2-2 test", signature, tag)
	assert.Nil(t, err)

	path := &v1.RepositoryPath{
		Name:           "test",
		RepoUrl:        "https://github.com/shubhamdixit863/testingrepo",
		Path:           "config",
		TargetRevision: "main",
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

	_, err = CheckForRepoUpdates(r, path, status, context.Background())
	assert.Nil(t, err)

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
		Email: "shubhamdixit863@gmail.com",
		When:  time.Now(),
	}
	err = updateFileInBranch(r, "main", fmt.Sprintf("%s/%s", path, fileNameToBeWatched), "test 12-2-2 test", signature, tag)
	assert.Nil(t, err)

	path := &v1.RepositoryPath{
		Name:           "test",
		RepoUrl:        "https://github.com/shubhamdixit863/testingrepo",
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

	_, err = CheckForRepoUpdates(r, path, status, context.Background())
	assert.Nil(t, err)

	err = os.RemoveAll("temp")
	assert.Nil(t, err)

}


*/

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
			expected: map[string]string{"numaflow-frontend": `apiVersion: v1
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
          cpu: "500m"`, "numaflow-my-service": `apiVersion: v1
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
				err := populateResourceMap([]byte(re), resourceMap)
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
			expected: "production-frontend",
		},
		{
			name: "Service without namespace",
			yaml: `apiVersion: v1
kind: Service
metadata:
  name: my-service`,
			expected: "default-my-service",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resourceName, err := getResourceName(tc.yaml)
			assert.Nil(t, err)
			assert.Equal(t, tc.expected, resourceName, "Extracted resource name should match expected")
		})
	}
}
