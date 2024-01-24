package kubernetes

import (
	"fmt"
	"path/filepath"

	jsongo "github.com/json-iterator/go"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// NewClient creates a new kubernetes client.
func NewClient(config *rest.Config, logger *zap.SugaredLogger) (Client, error) {
	config.Timeout = defaultTimeout

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	httpCacheDir := filepath.Join(cacheDir, "http")
	discoveryCacheDir := computeDiscoverCacheDir(filepath.Join(cacheDir, "discovery"), config.Host)

	// DiscoveryClient queries API server about the resources
	cdc, err := disk.NewCachedDiscoveryClientForConfig(config, discoveryCacheDir, httpCacheDir, ttl)
	if err != nil {
		return nil, err
	}

	return &client{
		dynamicClient: dynamicClient,
		config:        config,
		mapper:        restmapper.NewDeferredDiscoveryRESTMapper(cdc),
		log:           logger,
	}, nil
}

// DeleteResource will delete the resource from cluster using kind, name and namespace.
func (c *client) DeleteResource(kind, name, namespace string, do metav1.DeleteOptions) error {
	return c.deleteResourceByKindAndNameAndNamespace(kind, name, namespace, do)
}

// ApplyResource converts the manifest byte data to unstructured format and then applies it to the cluster
func (c *client) ApplyResource(data []byte, namespaceOverride string) error {
	obj := make(map[string]interface{})
	if err := yaml.Unmarshal(data, obj); err != nil {
		return fmt.Errorf("failed to unmarshal yaml data, err: %v", err)
	}

	// convert to unstructured data
	unstructuredData, err := ToUnstructured(obj)
	if err != nil {
		return err
	}

	return c.apply(unstructuredData, namespaceOverride)
}

func ToUnstructured(manifest map[string]interface{}) (*unstructured.Unstructured, error) {
	b, err := jsongo.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %v", err)
	}

	obj, _, err := unstructured.UnstructuredJSONScheme.Decode(b, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode %v", err)
	}

	// Convert the runtime.Object to unstructured.Unstructured.
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %v", err)
	}

	return &unstructured.Unstructured{
		Object: m,
	}, nil
}
