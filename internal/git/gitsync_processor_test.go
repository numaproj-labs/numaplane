package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"

	v1 "github.com/numaproj-labs/numaplane/api/v1alpha1"
)

func init() {
	_ = v1.AddToScheme(scheme.Scheme)
	_ = appv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
}

func Test_cloneRepo(t *testing.T) {
	testCases := []struct {
		name   string
		repo   v1.RepositoryPath
		hasErr bool
	}{
		{
			name: "valid repo",
			repo: v1.RepositoryPath{
				RepoUrl: "https://github.com/numaproj-labs/numaplane.git",
			},
			hasErr: false,
		},
		{
			name: "invalid repo",
			repo: v1.RepositoryPath{
				RepoUrl: "https://invalid_repo.git",
			},
			hasErr: true,
		},
	}

	t.Parallel()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := cloneRepo(&tc.repo)
			if tc.hasErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, r)
			}
		})
	}
}

// TODO: re-enable after https://github.com/numaproj-labs/numaplane/issues/56
//const (
//	testGitSyncName = "test-gitsync"
//	testNamespace   = "test-ns"
//)
//
//func newGitSync(repo v1.RepositoryPath) *v1.GitSync {
//	return &v1.GitSync{
//		TypeMeta: metav1.TypeMeta{},
//		ObjectMeta: metav1.ObjectMeta{
//			Namespace: testNamespace,
//			Name:      testGitSyncName,
//		},
//		Spec: v1.GitSyncSpec{
//			RepositoryPaths: []v1.RepositoryPath{
//				repo,
//			},
//		},
//		Status: v1.GitSyncStatus{},
//	}
//}
//
//func Test_watchRepo(t *testing.T) {
//	testCases := []struct {
//		name    string
//		gitSync *v1.GitSync
//		hasErr  bool
//	}{
//		{
//			name: "`main` as a TargetRevision",
//			gitSync: newGitSync(v1.RepositoryPath{
//				RepoUrl:        "https://github.com/numaproj-labs/numaplane-control-manifests.git",
//				Path:           "staging-usw2-k8s",
//				TargetRevision: "main",
//			}),
//			hasErr: false,
//		},
//		{
//			name: "tag name as a TargetRevision",
//			gitSync: newGitSync(v1.RepositoryPath{
//				RepoUrl:        "https://github.com/numaproj-labs/numaplane-control-manifests.git",
//				Path:           "staging-usw2-k8s",
//				TargetRevision: "v0.0.1",
//			}),
//			hasErr: false,
//		},
//		{
//			name: "commit hash as a TargetRevision",
//			gitSync: newGitSync(v1.RepositoryPath{
//				RepoUrl:        "https://github.com/numaproj-labs/numaplane-control-manifests.git",
//				Path:           "staging-usw2-k8s",
//				TargetRevision: "7b68200947f2d2624797e56edf02c6d848bc48d1",
//			}),
//			hasErr: false,
//		},
//		{
//			name: "remote branch name as a TargetRevision",
//			gitSync: newGitSync(v1.RepositoryPath{
//				RepoUrl:        "https://github.com/numaproj-labs/numaplane-control-manifests.git",
//				Path:           "staging-usw2-k8s",
//				TargetRevision: "refs/remotes/origin/pipeline",
//			}),
//			hasErr: false,
//		},
//		{
//			name: "local branch name as a TargetRevision",
//			gitSync: newGitSync(v1.RepositoryPath{
//				RepoUrl:        "https://github.com/numaproj-labs/numaplane-control-manifests.git",
//				Path:           "staging-usw2-k8s",
//				TargetRevision: "pipeline",
//			}),
//			hasErr: false,
//		},
//		{
//			name: "root path",
//			gitSync: newGitSync(v1.RepositoryPath{
//				RepoUrl:        "https://github.com/numaproj-labs/numaplane-control-manifests.git",
//				Path:           "",
//				TargetRevision: "pipeline",
//			}),
//			hasErr: false,
//		},
//		{
//			name: "unresolvable TargetRevision",
//			gitSync: newGitSync(v1.RepositoryPath{
//				RepoUrl:        "https://github.com/numaproj-labs/numaplane.git",
//				Path:           "config/samples",
//				TargetRevision: "unresolvable",
//			}),
//			hasErr: true,
//		},
//		{
//			name: "invalid path",
//			gitSync: newGitSync(v1.RepositoryPath{
//				RepoUrl:        "https://github.com/numaproj-labs/numaplane.git",
//				Path:           "invalid_path",
//				TargetRevision: "main",
//			}),
//			hasErr: true,
//		},
//	}
//	t.Parallel()
//	for _, tc := range testCases {
//
//		t.Run(tc.name, func(t *testing.T) {
//			repo := &tc.gitSync.Spec.RepositoryPaths[0]
//			r, err := cloneRepo(repo)
//			assert.Nil(t, err)
//
//			objects := []client.Object{
//				tc.gitSync,
//			}
//			client := fake.NewClientBuilder().WithObjects(objects...).WithStatusSubresource(tc.gitSync).Build()
//			err = watchRepo(context.Background(), r, tc.gitSync, utils.GetTestRestConfig(), client, repo, "")
//			if tc.hasErr {
//				assert.NotNil(t, err)
//			} else {
//				assert.Nil(t, err)
//			}
//		})
//	}
//}
