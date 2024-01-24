package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Client interface {
	// ApplyResource will create if resource is not already in cluster or update the resource if it's already in cluster then
	// do patch of annotation with key as "kubectl.kubernetes.io/last-applied-configuration".
	ApplyResource([]byte, string) error

	// DeleteResource will delete the resource from cluster by using kind, name and namespace with delete options.
	DeleteResource(string, string, string, metav1.DeleteOptions) error
}
