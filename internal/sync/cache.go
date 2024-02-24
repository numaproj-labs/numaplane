package sync

import (
	"errors"
	"fmt"
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/cespare/xxhash/v2"
	"github.com/numaproj-labs/numaplane/internal/kubernates"
	"github.com/numaproj-labs/numaplane/internal/shared"
	"go.uber.org/zap"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	"golang.org/x/sync/semaphore"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
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
	onObjectUpdated    ObjectUpdatedHandler

	cluster       clustercache.ClusterCache
	cacheSettings cacheSettings
	lock          sync.RWMutex
}

func NewLiveStateCache(
	onObjectUpdated ObjectUpdatedHandler,
) LiveStateCache {
	return &liveStateCache{
		onObjectUpdated: onObjectUpdated,
	}
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
		clustercache.SetPopulateResourceInfoHandler(c.PopulateResourceInfo),
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

	if settings.ignoreResourceUpdatesEnabled && shouldHashManifest("", gvk) {
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

// getGitSyncName retrieve application name base on tracking method
func getGitSyncName(un *unstructured.Unstructured) string {
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

// shouldHashManifest validates if the API resource needs to be hashed.
// If there's an app name from resource tracking, or if this is itself an app, we should generate a hash.
// Otherwise, the hashing should be skipped to save CPU time.
func shouldHashManifest(gitSyncName string, gvk schema.GroupVersionKind) bool {
	// Only hash if the resource belongs to an app.
	// Best      - Only hash for resources that are part of an app or their dependencies
	// (current) - Only hash for resources that are part of an app + all apps that might be from an ApplicationSet
	// Orphan    - If orphan is enabled, hash should be made on all resource of that namespace and a config to disable it
	// Worst     - Hash all resources watched by Argo
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

// Invalidate cache and executes callback that optionally might update cache settings
func (c *liveStateCache) invalidate(cacheSettings cacheSettings) {
	c.logger.Info("invalidating live state cache")
	c.lock.Lock()
	c.cacheSettings = cacheSettings
	clusterCache := c.cluster
	c.lock.Unlock()

	clusterCache.Invalidate(clustercache.SetSettings(cacheSettings.clusterSettings))

	c.logger.Info("live state cache invalidated")
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

type NoopNormalizer struct {
}

func (n *NoopNormalizer) Normalize(un *unstructured.Unstructured) error {
	return nil
}

// NewNoopNormalizer returns normalizer that does not apply any resource modifications
func NewNoopNormalizer() diff.Normalizer {
	return &NoopNormalizer{}
}
