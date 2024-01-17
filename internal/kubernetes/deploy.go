package kubernetes

import (
	"fmt"
	"path/filepath"

	jsongo "github.com/json-iterator/go"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// NewClient creates a new kubernetes client.
func NewClient(config *rest.Config) (Client, error) {
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
		c:      dynamicClient,
		config: config,
		mapper: restmapper.NewDeferredDiscoveryRESTMapper(cdc),
	}, nil
}

// ApplyResource converts tha manifest byte data in unstructured format
func (c *client) ApplyResource(data []byte) error {
	obj := make(map[string]interface{})
	if err := yaml.Unmarshal(data, obj); err != nil {
		return fmt.Errorf("failed to unmarshal yaml data, err: %v", err)
	}

	// convert to unstructured data
	unstructuredData, err := ToUnstructured(obj)
	if err != nil {
		return err
	}

	return c.apply(unstructuredData)
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
