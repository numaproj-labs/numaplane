package git

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
)

const (
	testNamespace = "test-ns"
)

func newGitSync(name string, repoUrl string, path string, targetRevision string) *v1alpha1.GitSync {
	return &v1alpha1.GitSync{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
		},
		Spec: v1alpha1.GitSyncSpec{
			RepoUrl:        repoUrl,
			TargetRevision: targetRevision,
			Path:           path,
		},
		Status: v1alpha1.GitSyncStatus{},
	}
}

func Test_cloneRepo(t *testing.T) {
	testCases := []struct {
		name    string
		gitSync *v1alpha1.GitSync
		hasErr  bool
	}{
		{
			name: "valid repo",
			gitSync: newGitSync(
				"validRepo",
				"https://github.com/numaproj-labs/numaplane.git",
				"",
				"main",
			),
			hasErr: false,
		},
	}

	t.Parallel()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			localRepoPath := getLocalRepoPath(tc.gitSync)
			err := os.RemoveAll(localRepoPath)
			assert.Nil(t, err)
			cloneOptions := &git.CloneOptions{
				URL: tc.gitSync.Spec.RepoUrl,
			}
			r, err := cloneRepo(context.Background(), tc.gitSync, cloneOptions)

			if tc.hasErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, r)
			}
		})
	}
}

func Test_GetLatestManifests(t *testing.T) {
	testCases := []struct {
		name    string
		gitSync *v1alpha1.GitSync
		hasErr  bool
	}{
		{
			name: "`main` as a TargetRevision",
			gitSync: newGitSync(
				"main",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"staging-usw2-k8s",
				"main",
			),
			hasErr: false,
		},
		{
			name: "tag name as a TargetRevision",
			gitSync: newGitSync(
				"tagName",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"staging-usw2-k8s",
				"v0.0.1",
			),
			hasErr: false,
		},
		{
			name: "commit hash as a TargetRevision",
			gitSync: newGitSync(
				"commitHash",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"staging-usw2-k8s",
				"7b68200947f2d2624797e56edf02c6d848bc48d1",
			),
			hasErr: false,
		},
		{
			name: "remote branch name as a TargetRevision",
			gitSync: newGitSync(
				"remoteBranch",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"staging-usw2-k8s",
				"refs/remotes/origin/pipeline",
			),
			hasErr: false,
		},
		{
			name: "local branch name as a TargetRevision",
			gitSync: newGitSync(
				"localBranch",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"staging-usw2-k8s",
				"pipeline",
			),
			hasErr: false,
		},
		{
			name: "root path",
			gitSync: newGitSync(
				"rootPath",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"",
				"pipeline",
			),
			hasErr: false,
		},
		{
			name: "unresolvable TargetRevision",
			gitSync: newGitSync(
				"unresolvableTargetRevision",
				"https://github.com/numaproj-labs/numaplane.git",
				"config/samples",
				"unresolvable",
			),
			hasErr: true,
		},
		{
			name: "invalid path",
			gitSync: newGitSync(
				"invalidPath",
				"https://github.com/numaproj-labs/numaplane.git",
				"invalid_path",
				"main",
			),
			hasErr: true,
		},
		{
			name: "manifest from kustomize enabled repo",
			gitSync: &v1alpha1.GitSync{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      "kustomize-manifest",
				},
				Spec: v1alpha1.GitSyncSpec{
					RepoUrl:        "https://github.com/numaproj/numaflow.git",
					TargetRevision: "main",
					Path:           "config/namespace-install",
					Kustomize:      &v1alpha1.KustomizeSource{},
				},
			},
			hasErr: false,
		},
		{
			name: "manifest from helm enabled repo",
			gitSync: &v1alpha1.GitSync{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      "helm-manifest",
				},
				Spec: v1alpha1.GitSyncSpec{
					RepoUrl:        "https://github.com/numaproj/helm-charts.git",
					TargetRevision: "main",
					Path:           "charts/numaflow",
					Helm:           &v1alpha1.HelmSource{},
				},
			},
			hasErr: false,
		},
	}
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {
			localRepoPath := getLocalRepoPath(tc.gitSync)
			err := os.RemoveAll(localRepoPath)
			assert.Nil(t, err)
			cloneOptions := &git.CloneOptions{
				URL: tc.gitSync.Spec.RepoUrl,
			}
			r, cloneErr := cloneRepo(context.Background(), tc.gitSync, cloneOptions)
			assert.Nil(t, cloneErr)

			// To break the continuous check of repo update, added the context timeout.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			_, err = GetLatestManifests(ctx, r, tc.gitSync)
			if tc.hasErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestGetLatestCommitHash(t *testing.T) {
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:          "https://github.com/shubhamdixit863/testingrepo",
		SingleBranch: true,
	})
	assert.Nil(t, err)
	commit, err := getLatestCommitHash(r, "main")
	assert.Nil(t, err)
	assert.Equal(t, 40, len(commit.String()))
}
