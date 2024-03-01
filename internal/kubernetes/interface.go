package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Generate a new client using the kubernetes controller.

//go:generate mockgen -destination=mocks/mock_client.go -package=mocks . Client
type Client interface {
	// ApplyResource will create if resource is not already in cluster or update the resource if it's already in cluster then
	// do patch of annotation with key as "kubectl.kubernetes.io/last-applied-configuration".
	ApplyResource([]byte, string) error

	// DeleteResource will delete the resource from cluster by using kind, name and namespace with delete options.
	DeleteResource(string, string, string, metav1.DeleteOptions) error

	// Get the resource from kubernetes cluster of the specified resource.
	Get(ctx context.Context, key k8sClient.ObjectKey, obj k8sClient.Object, opts ...k8sClient.GetOption) error

	// Update the resource of kubernetes cluster.
	Update(ctx context.Context, obj k8sClient.Object, opts ...k8sClient.UpdateOption) error

	// GetSecret Gets a Kubernetes Secret
	GetSecret(ctx context.Context, namespace, secretName string) (*corev1.Secret, error)

	// StatusUpdate will update the status of kubernetes resource.
	StatusUpdate(ctx context.Context, obj k8sClient.Object, opts ...k8sClient.SubResourceUpdateOption) error
}
