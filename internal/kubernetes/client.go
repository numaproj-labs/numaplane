package kubernetes

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubectl/pkg/util"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
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
	kubeClient    k8sClient.Client
	dynamicClient dynamic.Interface
	config        *rest.Config
	mapper        *restmapper.DeferredDiscoveryRESTMapper
	log           *zap.SugaredLogger
}

// apply will do create/patch of manifest
func (c *client) apply(u *unstructured.Unstructured, namespaceOverride string) error {
	gvk := u.GroupVersionKind()
	restMapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	// TODO: Make this as reusable REST Client rather than needing generating new client everytime.
	restClient, err := newRestClient(*c.config, gvk.GroupVersion())
	if err != nil {
		return err
	}

	helper := resource.NewHelper(restClient, restMapping)

	// Override namespace of manifest
	if err := SetNamespaceIfScoped(namespaceOverride, u, helper); err != nil {
		return err
	}

	resourceInfo := &resource.Info{
		Client:          restClient,
		Mapping:         restMapping,
		Namespace:       u.GetNamespace(),
		Name:            u.GetName(),
		Object:          u,
		ResourceVersion: restMapping.Resource.Version,
	}

	patcher := newPatcher(resourceInfo, helper)

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

// Delete resource from cluster using kind, name and namespace.
func (c *client) deleteResourceByKindAndNameAndNamespace(kind, name, namespace string, do metav1.DeleteOptions) error {
	gvk, err := c.mapper.KindFor(schema.GroupVersionResource{Resource: kind})
	if err != nil {
		return err
	}

	restMapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	restClient, err := newRestClient(*c.config, gvk.GroupVersion())
	if err != nil {
		return err
	}
	helper := resource.NewHelper(restClient, restMapping)

	// delete resource based on namespaced or non-namespaced.
	if helper.NamespaceScoped {
		return c.dynamicClient.Resource(restMapping.Resource).Namespace(namespace).Delete(context.Background(), name, do)
	}
	return c.dynamicClient.Resource(restMapping.Resource).Delete(context.Background(), name, do)
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

func SetNamespaceIfScoped(namespaceOverride string, u *unstructured.Unstructured, helper *resource.Helper) error {
	if helper.NamespaceScoped {
		if namespaceOverride == "" {
			return fmt.Errorf("failed to set namespace, override namespace is empty")
		} else {
			u.SetNamespace(namespaceOverride)
		}
	}

	return nil
}

// Get the resource in kubernetes cluster
func (c *client) Get(ctx context.Context, key k8sClient.ObjectKey, obj k8sClient.Object, opts ...k8sClient.GetOption) error {
	return c.kubeClient.Get(ctx, key, obj, opts...)
}

// Update the resource in kubernetes cluster
func (c *client) Update(ctx context.Context, obj k8sClient.Object, opts ...k8sClient.UpdateOption) error {
	return c.kubeClient.Update(ctx, obj, opts...)
}

// StatusUpdate will update the status of kubernetes resource
func (c *client) StatusUpdate(ctx context.Context, obj k8sClient.Object, opts ...k8sClient.SubResourceUpdateOption) error {
	return c.kubeClient.Status().Update(ctx, obj, opts...)
}
