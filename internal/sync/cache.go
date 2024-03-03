package sync

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/cespare/xxhash/v2"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	"github.com/numaproj-labs/numaplane/internal/shared"
	"github.com/numaproj-labs/numaplane/internal/shared/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/shared/logging"
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

// LiveStateCache is a cluster caching that stores resource references and ownership
// references. It also stored custom metadata for resources managed by GitSyncs. It always
// ensures the cache is up-to-date before returning the resources.
type LiveStateCache interface {
	// GetClusterCache returns synced cluster cache
	GetClusterCache() (clustercache.ClusterCache, error)
	// GetManagedLiveObjs returns state of live nodes which correspond to target nodes of specified gitsync.
	GetManagedLiveObjs(gitsync *v1alpha1.GitSync, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error)
	// Init must be executed before cache can be used
	Init()
	// PopulateResourceInfo is called by the cache to update ResourceInfo struct for a managed resource
	PopulateResourceInfo(un *unstructured.Unstructured, isRoot bool) (interface{}, bool)
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
	// TODO: utilize `onObjectUpdated` when switch to change event triggering for sync tasking
	// onObjectUpdated ObjectUpdatedHandler

	cluster       clustercache.ClusterCache
	cacheSettings cacheSettings
	lock          sync.RWMutex
}

func newLiveStateCache(
	cluster clustercache.ClusterCache,
) LiveStateCache {
	return &liveStateCache{
		cluster: cluster,
	}
}

func NewLiveStateCache(
	clusterCacheConfig *rest.Config,
) LiveStateCache {
	return &liveStateCache{
		clusterCacheConfig: clusterCacheConfig,
	}
}

func (c *liveStateCache) getCluster() clustercache.ClusterCache {
	c.lock.RLock()
	if c.cluster != nil {
		return c.cluster
	}
	cacheSettings := c.cacheSettings
	c.lock.RUnlock()

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.cluster != nil {
		return c.cluster
	}

	clusterCacheConfig := c.clusterCacheConfig

	// TODO: check if we need set managed namespace
	clusterCacheOpts := []clustercache.UpdateSettingsFunc{
		clustercache.SetListSemaphore(semaphore.NewWeighted(clusterCacheListSemaphoreSize)),
		clustercache.SetListPageSize(clusterCacheListPageSize),
		clustercache.SetListPageBufferSize(clusterCacheListPageBufferSize),
		clustercache.SetWatchResyncTimeout(clusterCacheWatchResyncDuration),
		clustercache.SetClusterSyncRetryTimeout(clusterSyncRetryTimeoutDuration),
		clustercache.SetResyncTimeout(clusterCacheResyncDuration),
		clustercache.SetSettings(cacheSettings.clusterSettings),
		clustercache.SetPopulateResourceInfoHandler(c.PopulateResourceInfo),
		clustercache.SetRetryOptions(clusterCacheAttemptLimit, clusterCacheRetryUseBackoff, isRetryableError),
		clustercache.SetLogr(logging.NewLogrusLogger(logging.NewWithCurrentConfig())),
	}

	clusterCache := clustercache.NewClusterCache(clusterCacheConfig, clusterCacheOpts...)

	// Register event handler that is executed whenever the resource gets updated in the cache
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

	return clusterCache
}

func (c *liveStateCache) PopulateResourceInfo(un *unstructured.Unstructured, isRoot bool) (interface{}, bool) {
	res := &ResourceInfo{}
	c.lock.RLock()
	settings := c.cacheSettings
	c.lock.RUnlock()

	res.Health, _ = health.GetResourceHealth(un, settings.clusterSettings.ResourceHealthOverride)

	gitSyncName := getGitSyncName(un)
	if isRoot && gitSyncName != "" {
		res.GitSyncName = gitSyncName
	}

	gvk := un.GroupVersionKind()

	if settings.ignoreResourceUpdatesEnabled && shouldHashManifest(gitSyncName, gvk) {
		hash, err := generateManifestHash(un)
		if err != nil {
			c.logger.Errorf("Failed to generate manifest hash: %v", err)
		} else {
			res.manifestHash = hash
		}
	}

	// edge case. we do not label CRDs, so they miss the tracking label we inject. But we still
	// want the full resource to be available in our cache (to diff), so we store all CRDs
	return res, res.GitSyncName != "" || gvk.Kind == kube.CustomResourceDefinitionKind
}

// getGitSyncName gets the GitSync that owns the resource from an annotation in the resource
func getGitSyncName(un *unstructured.Unstructured) string {
	value, err := kubernetes.GetGitSyncInstanceAnnotation(un, shared.AnnotationKeyGitSyncInstance)
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

// shouldHashManifest validates if the API resource needs to be hashed.
// If there's a GitSync name from resource tracking, or if this is itself a GitSync, we should generate a hash.
// Otherwise, the hashing should be skipped to save CPU time.
func shouldHashManifest(gitSyncName string, gvk schema.GroupVersionKind) bool {
	// Only hash if the resource belongs to a GitSync.
	return gitSyncName != "" || (gvk.Group == "numaplane.numaproj.io" && gvk.Kind == "GitSync")
}

func generateManifestHash(un *unstructured.Unstructured) (string, error) {

	// Only use a noop normalizer for now.
	normalizer := NewNoopNormalizer()

	resource := un.DeepCopy()
	err := normalizer.Normalize(resource)
	if err != nil {
		return "", fmt.Errorf("error normalizing resource: %w", err)
	}

	data, err := resource.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("error marshaling resource: %w", err)
	}
	hash := hash(data)
	return hash, nil
}

func hash(data []byte) string {
	return strconv.FormatUint(xxhash.Sum64(data), 16)
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
	clusterCache := c.getCluster()
	err := clusterCache.EnsureSynced()
	if err != nil {
		return nil, fmt.Errorf("error synchronizing cache state : %w", err)
	}
	return clusterCache, nil
}

func (c *liveStateCache) Init() {
	setting := c.loadCacheSettings()
	c.cacheSettings = *setting
}

func (c *liveStateCache) loadCacheSettings() *cacheSettings {
	ignoreResourceUpdatesEnabled := false
	resourcesFilter := &NoopResourceFilter{}
	clusterSettings := clustercache.Settings{
		ResourcesFilter: resourcesFilter,
	}

	return &cacheSettings{clusterSettings, ignoreResourceUpdatesEnabled}
}

func (c *liveStateCache) GetClusterCache() (clustercache.ClusterCache, error) {
	return c.getSyncedCluster()
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

type NoopNormalizer struct {
}

func (n *NoopNormalizer) Normalize(un *unstructured.Unstructured) error {
	return nil
}

// NewNoopNormalizer returns normalizer that does not apply any resource modifications
func NewNoopNormalizer() diff.Normalizer {
	return &NoopNormalizer{}
}

// NoopResourceFilter a noop resource filter
// TODO: add more meaningful resource filter
type NoopResourceFilter struct {
}

func (n *NoopResourceFilter) IsExcludedResource(group, kind, cluster string) bool {
	return false
}
