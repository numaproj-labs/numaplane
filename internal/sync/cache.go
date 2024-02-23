package sync

import (
	"context"
	"errors"
	"fmt"
	"github.com/numaproj-labs/numaplane/internal/kubernates"
	"github.com/numaproj-labs/numaplane/internal/shared"
	"go.uber.org/zap"
	"net"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"golang.org/x/sync/semaphore"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
)

// GitOps engine cluster cache tuning options
var (
	// clusterCacheResyncDuration controls the duration of cluster cache refresh.
	// NOTE: this differs from gitops-engine default of 24h
	clusterCacheResyncDuration = 12 * time.Hour

	// clusterCacheWatchResyncDuration controls the maximum duration that group/kind watches are allowed to run
	// for before relisting & restarting the watch
	clusterCacheWatchResyncDuration = 10 * time.Minute

	// clusterSyncRetryTimeoutDuration controls the sync retry duration when cluster sync error happens
	clusterSyncRetryTimeoutDuration = 10 * time.Second

	// The default limit of 50 is chosen based on experiments.
	clusterCacheListSemaphoreSize int64 = 50

	// clusterCacheListPageSize is the page size when performing K8s list requests.
	// 500 is equal to kubectl's size
	clusterCacheListPageSize int64 = 500

	// clusterCacheListPageBufferSize is the number of pages to buffer when performing K8s list requests
	clusterCacheListPageBufferSize int32 = 1

	// clusterCacheRetryLimit sets a retry limit for failed requests during cluster cache sync
	// If set to 1, retries are disabled.
	clusterCacheAttemptLimit int32 = 1

	// clusterCacheRetryUseBackoff specifies whether to use a backoff strategy on cluster cache sync, if retry is enabled
	clusterCacheRetryUseBackoff bool = false
)

type ResourceInfo struct {
	GitSyncName string

	Health       *health.HealthStatus
	manifestHash string
}

type LiveStateCache interface {
	// GetManagedLiveObjs Returns state of live nodes which correspond for target nodes of specified gitsync.
	GetManagedLiveObjs(gitsync *v1alpha1.GitSync, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error)
	// Run Starts watching resources of each controlled cluster.
	Run(ctx context.Context) error
	// Init must be executed before cache can be used
	Init() error
}

type cacheSettings struct {
	clusterSettings clustercache.Settings

	// ignoreResourceUpdates is a flag to enable resource-ignore rules.
	ignoreResourceUpdatesEnabled bool
}

type ObjectUpdatedHandler = func(managedByGitSync map[string]bool, ref v1.ObjectReference)

type liveStateCache struct {
	clusterCacheConfig *rest.Config
	logger             *zap.SugaredLogger
	appInformer        cache.SharedIndexInformer
	onObjectUpdated    ObjectUpdatedHandler

	cluster       clustercache.ClusterCache
	cacheSettings cacheSettings
	lock          sync.RWMutex
}

func (c *liveStateCache) getCluster() (clustercache.ClusterCache, error) {
	c.lock.RLock()
	cacheSettings := c.cacheSettings
	c.lock.RUnlock()

	if c.cluster != nil {
		return c.cluster, nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.cluster != nil {
		return c.cluster, nil
	}

	clusterCacheConfig := c.clusterCacheConfig

	// TODO: check if we need set managed namespace,
	//       also enable logging within clusterCache
	clusterCacheOpts := []clustercache.UpdateSettingsFunc{
		clustercache.SetListSemaphore(semaphore.NewWeighted(clusterCacheListSemaphoreSize)),
		clustercache.SetListPageSize(clusterCacheListPageSize),
		clustercache.SetListPageBufferSize(clusterCacheListPageBufferSize),
		clustercache.SetWatchResyncTimeout(clusterCacheWatchResyncDuration),
		clustercache.SetClusterSyncRetryTimeout(clusterSyncRetryTimeoutDuration),
		clustercache.SetResyncTimeout(clusterCacheResyncDuration),
		clustercache.SetSettings(cacheSettings.clusterSettings),
		clustercache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (interface{}, bool) {
			res := &ResourceInfo{}
			c.lock.RLock()
			cacheSettings := c.cacheSettings
			c.lock.RUnlock()

			res.Health, _ = health.GetResourceHealth(un, cacheSettings.clusterSettings.ResourceHealthOverride)

			gvk := un.GroupVersionKind()

			if cacheSettings.ignoreResourceUpdatesEnabled && shouldHashManifest("", gvk) {
				hash, err := generateManifestHash(un)
				if err != nil {
					c.logger.Errorf("Failed to generate manifest hash: %v", err)
				} else {
					res.manifestHash = hash
				}
			}

			return res, true
		}),
		clustercache.SetRetryOptions(clusterCacheAttemptLimit, clusterCacheRetryUseBackoff, isRetryableError),
	}

	clusterCache := clustercache.NewClusterCache(clusterCacheConfig, clusterCacheOpts...)

	_ = clusterCache.OnResourceUpdated(func(newRes *clustercache.Resource, oldRes *clustercache.Resource, namespaceResources map[kube.ResourceKey]*clustercache.Resource) {
		c.lock.RLock()
		cacheSettings := c.cacheSettings
		c.lock.RUnlock()

		if cacheSettings.ignoreResourceUpdatesEnabled && oldRes != nil && newRes != nil && skipResourceUpdate(resInfo(oldRes), resInfo(newRes)) {
			return
		}

		// TODO: when switch to change event triggering for sync tasking, notify
		//       the GitSync managing the resources that there are changes happen
	})

	c.cluster = clusterCache

	return clusterCache, nil
}

// getAppName retrieve application name base on tracking method
func getAppName(un *unstructured.Unstructured) string {
	value, err := kubernates.GetGitSyncInstanceAnnotation(un, shared.AnnotationKeyGitSyncInstance)
	if err != nil {
		return ""
	}
	return value
}

func resInfo(r *clustercache.Resource) *ResourceInfo {
	info, ok := r.Info.(*ResourceInfo)
	if !ok || info == nil {
		info = &ResourceInfo{}
	}
	return info
}

// TODO: add validation logic after resource tracking label is available.
// shouldHashManifest validates if the API resource needs to be hashed if belongs to a GitSync CR.
func shouldHashManifest(gitsyncName string, gvk schema.GroupVersionKind) bool {
	return false
}

// TODO: implement Manifest hash generation
func generateManifestHash(un *unstructured.Unstructured) (string, error) {
	return "", nil
}

// isRetryableError is a helper method to see whether an error
// returned from the dynamic client is potentially retryable.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	return kerrors.IsInternalError(err) ||
		kerrors.IsInvalid(err) ||
		kerrors.IsTooManyRequests(err) ||
		kerrors.IsServerTimeout(err) ||
		kerrors.IsServiceUnavailable(err) ||
		kerrors.IsTimeout(err) ||
		kerrors.IsUnexpectedObjectError(err) ||
		kerrors.IsUnexpectedServerError(err) ||
		isResourceQuotaConflictErr(err) ||
		isTransientNetworkErr(err) ||
		isExceededQuotaErr(err) ||
		errors.Is(err, syscall.ECONNRESET)
}

func isExceededQuotaErr(err error) bool {
	return kerrors.IsForbidden(err) && strings.Contains(err.Error(), "exceeded quota")
}

func isResourceQuotaConflictErr(err error) bool {
	return kerrors.IsConflict(err) && strings.Contains(err.Error(), "Operation cannot be fulfilled on resourcequota")
}

func isTransientNetworkErr(err error) bool {
	switch err.(type) {
	case net.Error:
		switch err.(type) {
		case *net.DNSError, *net.OpError, net.UnknownNetworkError:
			return true
		case *url.Error:
			// For a URL error, where it replies "connection closed"
			// retry again.
			return strings.Contains(err.Error(), "Connection closed by foreign host")
		}
	}

	errorString := err.Error()
	if exitErr, ok := err.(*exec.ExitError); ok {
		errorString = fmt.Sprintf("%s %s", errorString, exitErr.Stderr)
	}
	if strings.Contains(errorString, "net/http: TLS handshake timeout") ||
		strings.Contains(errorString, "i/o timeout") ||
		strings.Contains(errorString, "connection timed out") ||
		strings.Contains(errorString, "connection reset by peer") {
		return true
	}
	return false
}

func skipResourceUpdate(oldInfo, newInfo *ResourceInfo) bool {
	if oldInfo == nil || newInfo == nil {
		return false
	}
	isSameHealthStatus := (oldInfo.Health == nil && newInfo.Health == nil) || oldInfo.Health != nil && newInfo.Health != nil && oldInfo.Health.Status == newInfo.Health.Status
	isSameManifest := oldInfo.manifestHash != "" && newInfo.manifestHash != "" && oldInfo.manifestHash == newInfo.manifestHash
	return isSameHealthStatus && isSameManifest
}

func (c *liveStateCache) getSyncedCluster() (clustercache.ClusterCache, error) {
	clusterCache, err := c.getCluster()
	if err != nil {
		return nil, fmt.Errorf("error getting cluster: %w", err)
	}
	err = clusterCache.EnsureSynced()
	if err != nil {
		return nil, fmt.Errorf("error synchronizing cache state : %w", err)
	}
	return clusterCache, nil
}

func (c *liveStateCache) Init() error {
	cacheSettings, err := c.loadCacheSettings()
	if err != nil {
		return fmt.Errorf("error loading cache settings: %w", err)
	}
	c.cacheSettings = *cacheSettings
	return nil
}

func (c *liveStateCache) loadCacheSettings() (*cacheSettings, error) {

	ignoreResourceUpdatesEnabled := false
	clusterSettings := clustercache.Settings{}

	return &cacheSettings{clusterSettings, ignoreResourceUpdatesEnabled}, nil
}

func (c *liveStateCache) GetManagedLiveObjs(gitsync *v1alpha1.GitSync, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	clusterInfo, err := c.getSyncedCluster()
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info for %q: %w", gitsync.Spec.Destination.Cluster, err)
	}
	return clusterInfo.GetManagedLiveObjs(targetObjs, func(r *clustercache.Resource) bool {
		return resInfo(r).GitSyncName == gitsync.Name
	})
}
