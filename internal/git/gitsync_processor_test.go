package git

import (
	"context"
	"testing"

	v1 "github.com/numaproj-labs/numaplane/api/v1"
	"github.com/stretchr/testify/assert"
)

func Test_watchRepo(t *testing.T) {
	testCases := []struct {
		name     string
		repo     v1.RepositoryPath
		hasError bool
	}{
		{
			name: "`main` as a TargetRevision",
			repo: v1.RepositoryPath{
				RepoUrl:        "https://github.com/numaproj-labs/numaplane.git",
				Path:           "config/samples",
				TargetRevision: "main",
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
			name: "commit hash as a TargetRevision",
			repo: v1.RepositoryPath{
				RepoUrl:        "https://github.com/numaproj-labs/numaplane.git",
				Path:           "config/samples",
				TargetRevision: "8ed04cc87c3ab8f5d7d459f82e587da5a9192d20",
			},
			hasError: false,
		},
		{
			name: "remote branch name as a TargetRevision",
			repo: v1.RepositoryPath{
				RepoUrl:        "https://github.com/git-fixtures/basic.git",
				Path:           "go",
				TargetRevision: "refs/remotes/origin/branch",
			},
			hasError: false,
		},
		{
			name: "local branch name as a TargetRevision",
			repo: v1.RepositoryPath{
				RepoUrl:        "https://github.com/git-fixtures/basic.git",
				Path:           "go",
				TargetRevision: "branch",
			},
			hasError: false,
		},
		{
			name: "root path",
			repo: v1.RepositoryPath{
				RepoUrl:        "https://github.com/git-fixtures/basic.git",
				Path:           "",
				TargetRevision: "branch",
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
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := watchRepo(context.Background(), &tc.repo, "")
			if tc.hasError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
