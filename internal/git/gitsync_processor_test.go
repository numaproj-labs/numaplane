package git

import (
	"context"
	"testing"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	_ = v1alpha1.AddToScheme(scheme.Scheme)
	_ = appv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
}

func Test_cloneRepo(t *testing.T) {
	testCases := []struct {
		name   string
		repo   v1alpha1.RepositoryPath
		hasErr bool
	}{
		{
			name: "valid repo",
			repo: v1alpha1.RepositoryPath{
				Name:           "numaplane",
				RepoUrl:        "https://github.com/numaproj-labs/numaplane.git",
				TargetRevision: "main",
			},
			hasErr: false,
		},
	}

	t.Parallel()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			localRepoPath := getLocalRepoPath("gitsync-test-example")
			r, err := cloneRepo(context.Background(), localRepoPath, &tc.repo)
			if tc.hasErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, r)
			}
		})
	}
}
