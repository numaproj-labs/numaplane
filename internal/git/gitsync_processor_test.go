package git

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	v1 "github.com/numaproj-labs/numaplane/api/v1"
)

const (
	fileNameToBeWatched = "k8.yaml"
	path                = "config"
)

func Test_watchRepo(t *testing.T) {
	testCases := []struct {
		name     string
		repo     v1.RepositoryPath
		hasError bool
	}{
		{
			name: "branch name as a TargetRevision",
			repo: v1.RepositoryPath{
				RepoUrl:        "https://github.com/numaproj-labs/numaplane.git",
				Path:           "config/samples",
				TargetRevision: "main",
			},
			hasError: false,
		},
		{
			name: "commit hash as a TargetRevision",
			repo: v1.RepositoryPath{
				RepoUrl:        "https://github.com/numaproj-labs/numaplane.git",
				Path:           "config/samples",
				TargetRevision: "8ed04cc87c3ab8f5d7d459f82e587da5a9192d20",
			},
			hasError: false,
		},
		{
			name: "tag name as a TargetRevision",
			repo: v1.RepositoryPath{
				RepoUrl:        "https://github.com/go-git/go-git.git",
				Path:           ".github",
				TargetRevision: "v5.5.1",
			},
			hasError: false,
		},
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
	gitsyncPr := GitSyncProcessor{
		gitSync:     v1.GitSync{},
		channels:    nil,
		k8Client:    nil,
		clusterName: "",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := gitsyncPr.watchRepo(ctx, &tc.repo, "")
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
			Username: os.Getenv("username"),
			Password: os.Getenv("password"),
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

	r, lastCommitHash, err := getCommitHashAndRepo()
	assert.Nil(t, err)

	signature := &object.Signature{
		Name:  "test",
		Email: "shubhamdixit863@gmail.com",
		When:  time.Now(),
	}
	err = updateFileInBranch(r, "main", fmt.Sprintf("%s/%s", path, fileNameToBeWatched), "test 12-2-2 test", signature, "")
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

	updates, err := CheckForRepoUpdates(r, path, status, context.Background())
	assert.Nil(t, err)
	bytes, ok := updates.ModifiedFiles[fileNameToBeWatched]
	assert.True(t, ok)
	assert.NotEmpty(t, bytes, "File content should not be empty")
	assert.Greater(t, len(bytes), 0, "Length of file content should be greater than 0")
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

	updates, err := CheckForRepoUpdates(r, path, status, context.Background())
	assert.Nil(t, err)
	bytes, ok := updates.ModifiedFiles[fileNameToBeWatched]
	assert.True(t, ok)
	assert.NotEmpty(t, bytes, "File content should not be empty")
	assert.Greater(t, len(bytes), 0, "Length of file content should be greater than 0")
	err = os.RemoveAll("temp")
	assert.Nil(t, err)

}
