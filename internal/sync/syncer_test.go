package sync

import (
	"context"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/numaproj-labs/numaplane/internal/metrics"
)

type MockKubectl struct {
	kube.Kubectl

	DeletedResources []kube.ResourceKey
	CreatedResources []*unstructured.Unstructured
}

func (m *MockKubectl) CreateResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, obj *unstructured.Unstructured, createOptions metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	m.CreatedResources = append(m.CreatedResources, obj)
	return m.Kubectl.CreateResource(ctx, config, gvk, name, namespace, obj, createOptions, subresources...)
}

func (m *MockKubectl) DeleteResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, deleteOptions metav1.DeleteOptions) error {
	m.DeletedResources = append(m.DeletedResources, kube.NewResourceKey(gvk.Group, gvk.Kind, namespace, name))
	return m.Kubectl.DeleteResource(ctx, config, gvk, name, namespace, deleteOptions)
}

func newFakeSyncer() *Syncer {
	client := fake.NewClientBuilder().Build()
	metric, err := metrics.NewMetricsServer()
	if err != nil {
		panic(err)
	}

	//stateCache := newFakeLivStateCache(t)
	kubectl := &MockKubectl{Kubectl: &kubetest.MockKubectlCmd{}}
	syncer := NewSyncer(
		client,
		&rest.Config{Host: "https://localhost:6443"},
		metric,
		kubectl,
	)
	return syncer
}

func Test_BasicOperations(t *testing.T) {
	s := newFakeSyncer()
	assert.NotNil(t, s)
	s.StartWatching("key1")
	assert.True(t, s.Contains("key1"))
	assert.Equal(t, 1, s.Length())
	s.StopWatching("key1")
	assert.False(t, s.Contains("key1"))
}
