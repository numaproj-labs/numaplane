package controller

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/numaproj-labs/numaplane/api/v1"
	"github.com/numaproj-labs/numaplane/internal/git"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// what to test:
// newly created GitSync should: have finalizer, correct status, be added to map
// updated GitSync should still have the finalizer, correct status, and still be in map
// updated GitSync with cluster changes in both directions
// deleted GitSync should get deleted (i.e. finalizer removed) and no longer be in map

const (
	testGitSyncName = "test-gitsync"
	testNamespace   = "test-ns"
)

var (
	defaultGitSync = &apiv1.GitSync{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testGitSyncName,
		},
		Spec: apiv1.GitSyncSpec{
			RepositoryPaths: []apiv1.RepositoryPath{
				{
					Name:           "my-controller",
					RepoUrl:        "https://github.com/myrepo.git",
					Path:           "./numaflowController/",
					TargetRevision: "main",
				},
				{
					Name:           "my-pipelines",
					RepoUrl:        "https://github.com/myrepo.git",
					Path:           "./pipelines/",
					TargetRevision: "main",
				},
			},
			Destinations: []apiv1.Destination{
				{
					Cluster:   "staging-usw2-k8s",
					Namespace: "team-a-namespace",
				},
				{
					Cluster:   "staging-use2-k8s",
					Namespace: "team-a-namespace",
				},
			},
		},
	}
)

func init() {
	_ = apiv1.AddToScheme(scheme.Scheme)
	_ = appv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
}

func Test_NewGitSync(t *testing.T) {
	t.Run("new GitSync", func(t *testing.T) {
		gitSync := defaultGitSync.DeepCopy()
		ctx := context.TODO()
		client := fake.NewClientBuilder().Build()
		os.Setenv("CLUSTER_NAME", "staging-usw2-k8s")
		r, err := NewGitSyncReconciler(client, scheme.Scheme)
		assert.Nil(t, err)
		assert.NotNil(t, r)

		// reconcile it twice: first reconcile should be a new add, and the subsequent one should keep everything the same
		for i := 0; i < 2; i++ {
			_, err := r.reconcile(ctx, gitSync)
			assert.NoError(t, err)
			processorAsInterface, found := r.gitSyncProcessors.Load(gitSync.String())
			assert.True(t, found)
			assert.NotPanics(t, func() { _ = processorAsInterface.(*git.GitSyncProcessor) })

			assert.Equal(t, apiv1.GitSyncPhaseRunning, gitSync.Status.Phase)
			assert.Equal(t, string(apiv1.GitSyncConditionConfigured), gitSync.Status.Conditions[0].Type)
			assert.Equal(t, metav1.ConditionTrue, gitSync.Status.Conditions[0].Status)

		}
	})

}

// GitSync should be added to our GitSyncProcessor map if our cluster matches one of the clusters, but removed if it's not
func Test_GitSyncCluster(t *testing.T) {
	t.Run("GitSync cluster test", func(t *testing.T) {
		gitSync := defaultGitSync.DeepCopy()
		gitSync.Spec.Destinations = []apiv1.Destination{ // doesn't include our cluster
			{
				Cluster:   "staging-use2-k8s",
				Namespace: "team-a-namespace",
			},
		}
		ctx := context.TODO()
		client := fake.NewClientBuilder().Build()
		os.Setenv("CLUSTER_NAME", "staging-usw2-k8s")
		r, err := NewGitSyncReconciler(client, scheme.Scheme)
		assert.Nil(t, err)
		assert.NotNil(t, r)

		// our cluster is not one of the destinations, so it shouldn't end up in the map
		_, err = r.reconcile(ctx, gitSync)
		assert.NoError(t, err)
		_, found := r.gitSyncProcessors.Load(gitSync.String())
		assert.False(t, found)

		// now update the spec so that it is one of the destinations
		gitSync = defaultGitSync.DeepCopy()
		_, err = r.reconcile(ctx, gitSync)
		assert.NoError(t, err)
		processorAsInterface, found := r.gitSyncProcessors.Load(gitSync.String())
		assert.True(t, found)
		assert.NotPanics(t, func() { _ = processorAsInterface.(*git.GitSyncProcessor) })
	})
}
