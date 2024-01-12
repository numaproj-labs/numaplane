package kubernetes

import (
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubectl/pkg/util"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// Default cache directory.
	cacheDir       = "/var/kube/cache"
	defaultTimeout = 180 * time.Second
)

var (
	ttl = 10 * time.Minute
)

type client struct {
	c      dynamic.Interface
	config *rest.Config
	mapper *restmapper.DeferredDiscoveryRESTMapper
	log    *zap.SugaredLogger
}

type Client interface {
	Apply(*unstructured.Unstructured) error
}

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

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cdc)

	kubeClient := &client{
		c:      dynamicClient,
		config: config,
		mapper: mapper,
	}

	return kubeClient, nil
}

// Apply will do create/patch of manifest
func (c *client) Apply(u *unstructured.Unstructured) error {
	gvk := u.GroupVersionKind()
	restMapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	gv := gvk.GroupVersion()
	c.config.GroupVersion = &gv

	restClient, err := newRestClient(*c.config, gv)
	if err != nil {
		return err
	}

	helper := resource.NewHelper(restClient, restMapping)

	resourceInfo := &resource.Info{
		Client:          restClient,
		Mapping:         restMapping,
		Namespace:       u.GetNamespace(),
		Name:            u.GetName(),
		Object:          u,
		ResourceVersion: restMapping.Resource.Version,
	}

	patcher, err := newPatcher(resourceInfo, helper)
	if err != nil {
		return err
	}

	// Retrieves the modified configuration of the object, then embeds the result as an annotation in the modified configuration
	modified, err := util.GetModifiedConfiguration(resourceInfo.Object, true, unstructured.UnstructuredJSONScheme)
	if err != nil {
		return err
	}

	// retrieves the object from the Namespace and Name fields to check if it already exists.
	if err := resourceInfo.Get(); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		// If object doesn't exist then create the resource in the given namespace.
		obj, err := helper.Create(resourceInfo.Namespace, true, resourceInfo.Object)
		if err != nil {
			return err
		}
		if err := resourceInfo.Refresh(obj, true); err != nil {
			c.log.Warn(err)
		}
	}

	// Patch an OpenAPI resource
	_, patchedObj, err := patcher.Patch(resourceInfo.Object, modified, resourceInfo.Source, resourceInfo.Namespace,
		resourceInfo.Name)
	if err != nil {
		return err
	}

	// Refresh, updates the object with another object
	if err := resourceInfo.Refresh(patchedObj, true); err != nil {
		c.log.Warn(err)
	}

	return nil
}

func newRestClient(restConfig rest.Config, gv schema.GroupVersion) (rest.Interface, error) {
	restConfig.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	restConfig.GroupVersion = &gv
	if len(gv.Group) == 0 {
		restConfig.APIPath = "/api"
	} else {
		restConfig.APIPath = "/apis"
	}

	return rest.RESTClientFor(&restConfig)
}

// overlyCautiousIllegalFileCharacters matches characters that *might* not be supported.  Windows is really restrictive,
// so this is really restrictive
var overlyCautiousIllegalFileCharacters = regexp.MustCompile(`[^(\w/.)]`)

// computeDiscoverCacheDir takes the parentDir and the host and comes up with a "usually non-colliding" name.
func computeDiscoverCacheDir(parentDir, host string) string {
	// strip the optional scheme from host if it's there:
	schemelesHost := strings.Replace(strings.Replace(host, "https://", "", 1), "http://", "", 1)
	// now do a simple collapse of non-AZ09 characters.  Collisions are possible but unlikely.  Even if we do collide the problem is short lived
	safeHost := overlyCautiousIllegalFileCharacters.ReplaceAllString(schemelesHost, "_")
	return filepath.Join(parentDir, safeHost)
}
