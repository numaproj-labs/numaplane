package controller

import (
	"context"
	"testing"

	k8 "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	apiv1 "github.com/numaproj-labs/numaplane/api/v1alpha1"
	"github.com/numaproj-labs/numaplane/internal/controller/config"

	"github.com/numaproj-labs/numaplane/internal/git"
	mocksClient "github.com/numaproj-labs/numaplane/internal/kubernetes/mocks"
)

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
			RepositoryPath: apiv1.RepositoryPath{
				Name:           "my-controller",
				RepoUrl:        "https://github.com/numaproj-labs/numaplane-control-manifests.git",
				Path:           "./numaflowController/",
				TargetRevision: "main",
			},
			Destination: apiv1.Destination{
				Cluster:   "staging-usw2-k8s",
				Namespace: "team-a-namespace",
			},
		},
	}
)

func init() {
	_ = apiv1.AddToScheme(scheme.Scheme)
	_ = appv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
}

// test reconciliation of GitSync as it progresses through creation, update, and deletion
func Test_GitSyncLifecycle(t *testing.T) {
	t.Run("GitSync lifecycle", func(t *testing.T) {
		gitSync := defaultGitSync.DeepCopy()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		client := mocksClient.NewMockClient(ctrl)
		key := k8.ObjectKey{
			Namespace: "team-a-namespace",
			Name:      "secret",
		}
		client.EXPECT().Get(context.Background(), key, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(ctx context.Context, key k8.ObjectKey, obj k8.Object, opts ...k8.GetOption) error {
			s := obj.(*corev1.Secret)
			s.Data = map[string][]byte{"username": []byte("admin"), "password": []byte("secret")}
			return nil
		})

		cm := config.GetConfigManagerInstance()
		configM := cm.GetConfig()
		configM.ClusterName = "staging-usw2-k8s"
		gitCred := &config.GitCredential{
			HTTPCredential: &config.HTTPCredential{
				Username: "test",
				Password: config.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "secret"},
					Key:                  "secret",
					Optional:             nil,
				},
			},
			SSHCredential: nil,
			TLS:           nil,
		}
		mp := make(map[string]*config.GitCredential)
		mp["https://github.com/numaproj-labs/numaplane-control-manifests.git"] = gitCred
		configM.RepoCredentials = mp

		r, err := NewGitSyncReconciler(client, scheme.Scheme, cm)
		assert.Nil(t, err)
		assert.NotNil(t, r)

		// reconcile the newly created GitSync
		reconcile(t, r, gitSync)
		verifyRunning(t, r, gitSync)

		// update the spec
		gitSync.Spec.RepositoryPath.Path = gitSync.Spec.RepositoryPath.Path + "xyz"
		reconcile(t, r, gitSync)
		verifyRunning(t, r, gitSync)

		// mark the GitSync for deletion
		now := metav1.Now()
		gitSync.DeletionTimestamp = &now
		reconcile(t, r, gitSync)
		verifyDeleted(t, r, gitSync)

	})

}

// Test the changing of destinations in the GitSync
// GitSync should be added to our GitSyncProcessor map if our cluster matches one of the clusters, but removed if it's not
func Test_GitSyncDestinationChanges(t *testing.T) {
	t.Run("GitSync destination test", func(t *testing.T) {
		gitSync := defaultGitSync.DeepCopy()
		gitSync.Spec.Destination = apiv1.Destination{ // doesn't include our cluster
			Cluster:   "staging-use2-k8s",
			Namespace: "team-a-namespace",
		}

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		client := mocksClient.NewMockClient(ctrl)

		key := k8.ObjectKey{
			Namespace: "team-a-namespace",
			Name:      "secret",
		}

		client.EXPECT().Get(context.Background(), key, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(ctx context.Context, key k8.ObjectKey, obj k8.Object, opts ...k8.GetOption) error {
			s := obj.(*corev1.Secret)
			s.Data = map[string][]byte{"username": []byte("admin"), "password": []byte("secret")}
			return nil
		}).AnyTimes()

		cm := config.GetConfigManagerInstance()
		configM := cm.GetConfig()
		configM.ClusterName = "staging-usw2-k8s"

		gitCred := &config.GitCredential{
			HTTPCredential: &config.HTTPCredential{
				Username: "admin",
				Password: config.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "secret"},
					Key:                  "secret",
					Optional:             nil,
				},
			},
			SSHCredential: nil,
			TLS:           nil,
		}
		mp := make(map[string]*config.GitCredential)
		mp["https://github.com/numaproj-labs/numaplane-control-manifests.git"] = gitCred
		configM.RepoCredentials = mp

		r, err := NewGitSyncReconciler(client, scheme.Scheme, cm)
		assert.Nil(t, err)
		assert.NotNil(t, r)

		// our cluster is not one of the destinations, so it shouldn't end up in the map
		reconcile(t, r, gitSync)
		verifyNotApplicable(t, r, gitSync)

		// now update the spec so that it is one of the destinations
		gitSync = defaultGitSync.DeepCopy()
		reconcile(t, r, gitSync)
		verifyRunning(t, r, gitSync)
	})
}

func reconcile(t *testing.T, r *GitSyncReconciler, gitSync *apiv1.GitSync) {
	err := r.reconcile(context.Background(), gitSync)
	assert.NoError(t, err)
}

// check that a GitSync is Running
func verifyRunning(t *testing.T, r *GitSyncReconciler, gitSync *apiv1.GitSync) {
	// verify in map
	processorAsInterface, found := r.gitSyncProcessors.Load(gitSync.String())
	assert.True(t, found)
	assert.NotPanics(t, func() { _ = processorAsInterface.(*git.GitSyncProcessor) })

	// verify phase and Conditions
	assert.Equal(t, apiv1.GitSyncPhaseRunning, gitSync.Status.Phase)
	assert.Equal(t, string(apiv1.GitSyncConditionConfigured), gitSync.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, gitSync.Status.Conditions[0].Status)
}

// check that a GitSync is deemed Not-Applicable
func verifyNotApplicable(t *testing.T, r *GitSyncReconciler, gitSync *apiv1.GitSync) {
	// verify not in map
	_, found := r.gitSyncProcessors.Load(gitSync.String())
	assert.False(t, found)

	// verify phase and Conditions
	assert.Equal(t, apiv1.GitSyncPhaseNA, gitSync.Status.Phase)
	assert.Equal(t, string(apiv1.GitSyncConditionConfigured), gitSync.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, gitSync.Status.Conditions[0].Status)
}

// check that a GitSync is in a Deleted state
func verifyDeleted(t *testing.T, r *GitSyncReconciler, gitSync *apiv1.GitSync) {
	// verify not in map
	_, found := r.gitSyncProcessors.Load(gitSync.String())
	assert.False(t, found)
}
