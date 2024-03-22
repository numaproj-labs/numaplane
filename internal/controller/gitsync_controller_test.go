package controller

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/numaproj-labs/numaplane/internal/controller/config"
	apiv1 "github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
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
			GitSource: apiv1.GitSource{
				GitLocation: apiv1.GitLocation{
					RepoUrl:        "https://github.com/numaproj-labs/numaplane-control-manifests.git",
					Path:           "staging-usw2-k8s",
					TargetRevision: "main",
				},
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
		client := fake.NewClientBuilder().Build()
		cm := config.GetConfigManagerInstance()
		getwd, err := os.Getwd()
		assert.Nil(t, err, "Failed to get working directory")
		configPath := filepath.Join(getwd, "../../", "tests", "manifests")
		err = cm.LoadConfig(func(err error) {
		}, configPath, "config", "yaml")
		assert.NoError(t, err)
		r, err := NewGitSyncReconciler(client, scheme.Scheme, cm, nil)
		assert.Nil(t, err)
		assert.NotNil(t, r)

		// reconcile the newly created GitSync
		reconcile(t, r, gitSync)
		verifyRunning(t, gitSync)

		// update the spec
		gitSync.Spec.Path = gitSync.Spec.Path + "xyz"
		reconcile(t, r, gitSync)
		verifyRunning(t, gitSync)

		// mark the GitSync for deletion
		now := metav1.Now()
		gitSync.DeletionTimestamp = &now
		reconcile(t, r, gitSync)
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
		client := fake.NewClientBuilder().Build()
		cm := config.GetConfigManagerInstance()
		getwd, err := os.Getwd()
		assert.Nil(t, err, "Failed to get working directory")
		configPath := filepath.Join(getwd, "../../", "tests", "manifests")
		err = cm.LoadConfig(func(err error) {
		}, configPath, "config", "yaml")
		assert.NoError(t, err)
		r, err := NewGitSyncReconciler(client, scheme.Scheme, cm, nil)
		assert.Nil(t, err)
		assert.NotNil(t, r)
		log.Println(cm.GetConfig())

		// our cluster is not one of the destinations, so it shouldn't end up in the map
		reconcile(t, r, gitSync)
		verifyNotApplicable(t, gitSync)

		// now update the spec so that it is one of the destinations
		gitSync = defaultGitSync.DeepCopy()
		reconcile(t, r, gitSync)
		verifyRunning(t, gitSync)
	})
}

func reconcile(t *testing.T, r *GitSyncReconciler, gitSync *apiv1.GitSync) {
	err := r.reconcile(context.Background(), gitSync)
	assert.NoError(t, err)
}

// check that a GitSync is Running
func verifyRunning(t *testing.T, gitSync *apiv1.GitSync) {

	// verify phase and Conditions
	assert.Equal(t, apiv1.GitSyncPhaseRunning, gitSync.Status.Phase)
	assert.Equal(t, string(apiv1.GitSyncConditionConfigured), gitSync.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, gitSync.Status.Conditions[0].Status)
}

// check that a GitSync is deemed Not-Applicable
func verifyNotApplicable(t *testing.T, gitSync *apiv1.GitSync) {

	// verify phase and Conditions
	assert.Equal(t, apiv1.GitSyncPhaseNA, gitSync.Status.Phase)
	assert.Equal(t, string(apiv1.GitSyncConditionConfigured), gitSync.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, gitSync.Status.Conditions[0].Status)
}
