package controller

import (
	"context"
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
		r := NewGitSyncReconciler(client, scheme.Scheme)

		//err := client.Create(ctx, gitSync)
		//assert.NoError(t, err)
		//time.Sleep(20 * time.Second)
		//gitSyncs := &apiv1.GitSyncList{}
		//err = client.List(ctx, gitSyncs)
		//assert.NoError(t, err)
		//fmt.Println("gitSyncs", gitSyncs)
		//namespacedName := k8stypes.NamespacedName{Namespace: gitSync.Namespace, Name: gitSync.Name}
		//_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: namespacedName})
		//assert.NoError(t, err)

		_, err := r.reconcile(ctx, gitSync)
		assert.NoError(t, err)
		processorAsInterface, found := r.gitSyncProcessors.Load(gitSync.String())
		assert.True(t, found)
		//_ = processorAsInterface.(*apiv1.GitSync) // just make sure type is right
		assert.NotPanics(t, func() { _ = processorAsInterface.(*git.GitSyncProcessor) })

		assert.Equal(t, apiv1.GitSyncPhaseRunning, gitSync.Status.Phase)
		assert.Equal(t, string(apiv1.GitSyncConditionConfigured), gitSync.Status.Conditions[0].Type)
		assert.Equal(t, metav1.ConditionTrue, gitSync.Status.Conditions[0].Status)
	})

}
