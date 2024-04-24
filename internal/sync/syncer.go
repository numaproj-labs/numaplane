package sync

import (
	"container/list"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/argoproj/gitops-engine/pkg/diff"
	gitopsSync "github.com/argoproj/gitops-engine/pkg/sync"
	gitopsSyncCommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	kubeUtil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/numaproj-labs/numaplane/internal/common"
	controllerConfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/git"
	"github.com/numaproj-labs/numaplane/internal/util/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/util/logger"
	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
)

type Syncer struct {
	client     client.Client
	config     *rest.Config
	rawConfig  *rest.Config
	kubectl    kubeUtil.Kubectl
	gitSyncMap map[string]*list.Element
	// List of the GitSync namespaced name, format is "namespace/name"
	gitSyncList *list.List
	lock        *sync.RWMutex
	options     *options
	stateCache  LiveStateCache
}

// KeyOfGitSync returns the unique key of a gitsync
func KeyOfGitSync(gitSync *v1alpha1.GitSync) string {
	return fmt.Sprintf("%s/%s", gitSync.Namespace, gitSync.Name)
}

// NewSyncer returns a Synchronizer instance.
func NewSyncer(
	client client.Client,
	config *rest.Config,
	rawConfig *rest.Config,
	kubectl kubeUtil.Kubectl,
	opts ...Option,
) *Syncer {
	watcherOpts := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(watcherOpts)
		}
	}
	stateCache := NewLiveStateCache(config)
	w := &Syncer{
		client:      client,
		config:      config,
		rawConfig:   rawConfig,
		kubectl:     kubectl,
		options:     watcherOpts,
		gitSyncMap:  make(map[string]*list.Element),
		gitSyncList: list.New(),
		lock:        new(sync.RWMutex),
		stateCache:  stateCache,
	}
	return w
}

// Contains returns if the synchronizer contains the key (namespace/name).
func (s *Syncer) Contains(key string) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	_, ok := s.gitSyncMap[key]
	return ok
}

// Length returns how many GitSync objects are watched
func (s *Syncer) Length() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.gitSyncList.Len()
}

// StartWatching put a key (namespace/name) into the synchronizer
func (s *Syncer) StartWatching(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.gitSyncMap[key]; !ok {
		s.gitSyncMap[key] = s.gitSyncList.PushBack(key)
	}
}

// StopWatching stops watching on the key (namespace/name)
func (s *Syncer) StopWatching(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if e, ok := s.gitSyncMap[key]; ok {
		_ = s.gitSyncList.Remove(e)
		delete(s.gitSyncMap, key)
	}
}

// Start function starts the synchronizer worker group.
// Each worker keeps picking up tasks (which contains GitSync keys) to sync the resources.
func (s *Syncer) Start(ctx context.Context) error {
	numaLogger := logger.FromContext(ctx).WithName("synchronizer")
	numaLogger.Info("Starting synchronizer...")

	keyCh := make(chan string)
	ctx, cancel := context.WithCancel(logger.WithLogger(ctx, numaLogger))
	defer cancel()

	err := s.stateCache.Init(&numaLogger)
	if err != nil {
		return err
	}

	// Worker group
	for i := 1; i <= s.options.workers; i++ {
		go s.run(ctx, i, keyCh)
	}

	// Function assign() moves an element in the list from the front to the back,
	// and send to the channel so that it can be picked up by a worker.
	assign := func() {
		s.lock.Lock()
		defer s.lock.Unlock()
		if s.gitSyncList.Len() == 0 {
			return
		}
		e := s.gitSyncList.Front()
		if key, ok := e.Value.(string); ok {
			s.gitSyncList.MoveToBack(e)
			keyCh <- key
		}
	}

	// Following for loop keeps calling assign() function to assign watching tasks to the workers.
	// It makes sure each element in the list will be assigned at most every N milliseconds.
	for {
		select {
		case <-ctx.Done():
			log.Info("Shutting down synchronizer job assigner.")
			return nil
		default:
			assign()
		}
		// Make sure each of the keys will be assigned at most every N milliseconds.
		time.Sleep(time.Millisecond * time.Duration(func() int {
			l := s.Length()
			if l == 0 {
				return s.options.taskInterval
			}
			result := s.options.taskInterval / l
			if result > 0 {
				return result
			}
			return 1
		}()))
	}
}

// Function run() defines each worker's job.
// It waits for keys in the channel, and starts a synchronization job
func (s *Syncer) run(ctx context.Context, id int, keyCh <-chan string) {
	numaLogger := logger.FromContext(ctx)
	numaLogger.Infof("Started synchronizer worker %v", id)
	for {
		select {
		case <-ctx.Done():
			numaLogger.Infof("Stopped synchronizer worker %v", id)
			return
		case key := <-keyCh:
			if err := s.runOnce(ctx, key, id); err != nil {
				numaLogger.Error(err, "Failed to execute a task", "gitSyncKey", key)
			}
		}
	}
}

// Function runOnce implements the logic of each synchronization.
func (s *Syncer) runOnce(ctx context.Context, key string, worker int) error {
	numaLogger := logger.FromContext(ctx).WithValues("worker", worker, "gitSyncKey", key)
	numaLogger.Debugf("Working on key: %s.", key)
	strs := strings.Split(key, "/")
	if len(strs) != 2 {
		return fmt.Errorf("invalid key %q", key)
	}
	namespace := strs[0]
	gitSyncName := strs[1]
	gitSync := &v1alpha1.GitSync{}
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: gitSyncName}, gitSync); err != nil {
		if apierrors.IsNotFound(err) {
			s.StopWatching(key)
			numaLogger.Info("No corresponding GitSync found, stopped watching.")
			return nil
		}
		return fmt.Errorf("failed to query GitSync object of key %q, %w", key, err)
	}
	if !gitSync.GetDeletionTimestamp().IsZero() {
		numaLogger.Debug("GitSync object being deleted.")
		err := ProcessGitSyncDeletion(ctx, gitSync, s)
		if err != nil {
			return err
		}
		s.StopWatching(key)
		return nil
	}

	globalConfig, err := controllerConfig.GetConfigManagerInstance().GetConfig()
	if err != nil {
		numaLogger.Error(err, "error getting the global config")
	}

	fmt.Printf("deletethis: setting log level to %d\n", globalConfig.LogLevel)
	numaLogger.SetLevel(globalConfig.LogLevel)

	repo, err := git.CloneRepo(ctx, s.client, gitSync, globalConfig)
	if err != nil {
		return fmt.Errorf("failed to clone the repo of key %q, %w", key, err)
	}
	commitHash, manifests, err := git.GetLatestManifests(ctx, repo, s.client, gitSync)
	if err != nil {
		return fmt.Errorf("failed to get the manifest of key %q, %w", key, err)
	}
	uns, err := applyAnnotationAndNamespace(manifests, gitSyncName, gitSync.Spec.Destination.Namespace)
	if err != nil {
		return fmt.Errorf("failed to parse the manifest of key %q, %w", key, err)
	}

	synced := false
	syncState, syncMessage := s.sync(gitSync, uns, &numaLogger)
	if syncState.Successful() {
		synced = true
		numaLogger.Info("GitSync object is successfully synced.")
	}

	namespacedName := types.NamespacedName{
		Namespace: gitSync.Namespace,
		Name:      gitSync.Name,
	}

	if !gitSync.GetDeletionTimestamp().IsZero() {
		log.Debug("GitSync object being deleted.")
		err := ProcessGitSyncDeletion(ctx, gitSync, s)
		if err != nil {
			return err
		}
		s.StopWatching(key)
		return nil
	} else {
		return updateCommitStatus(ctx, s.client, &numaLogger, namespacedName, commitHash, synced, syncMessage)
	}
}

type resourceInfoProviderStub struct {
}

func (r *resourceInfoProviderStub) IsNamespaced(_ schema.GroupKind) (bool, error) {
	return false, nil
}

// sync compares the live state of the watched resources to target state defined in git
// for the given GitSync. If it doesn't match, syncing the state to the live objects. Otherwise,
// skip the syncing.
func (s *Syncer) sync(
	gitSync *v1alpha1.GitSync,
	targetObjs []*unstructured.Unstructured,
	numaLogger *logger.NumaLogger,
) (gitopsSyncCommon.OperationPhase, string) {
	reconciliationResult, diffResults, err := s.compareState(gitSync, targetObjs)
	if err != nil {
		numaLogger.Error(err, "Error on comparing git sync state")
		return gitopsSyncCommon.OperationError, err.Error()
	}

	opts := []gitopsSync.SyncOpt{
		gitopsSync.WithLogr(*numaLogger.LogrLogger),
		gitopsSync.WithOperationSettings(false, true, false, false),
		gitopsSync.WithManifestValidation(true),
		gitopsSync.WithPruneLast(false),
		gitopsSync.WithResourceModificationChecker(true, diffResults),
		gitopsSync.WithReplace(false),
		gitopsSync.WithServerSideApply(true),
		gitopsSync.WithServerSideApplyManager(common.SSAManager),
	}

	cluster, err := s.stateCache.GetClusterCache()
	if err != nil {
		numaLogger.Error(err, "Error on getting the cluster cache")
		return gitopsSyncCommon.OperationError, err.Error()
	}
	openAPISchema := cluster.GetOpenAPISchema()

	syncCtx, cleanup, err := gitopsSync.NewSyncContext(
		gitSync.Spec.TargetRevision,
		reconciliationResult,
		s.config,
		s.rawConfig,
		s.kubectl,
		gitSync.Spec.Destination.Namespace,
		openAPISchema,
		opts...,
	)
	defer cleanup()
	if err != nil {
		numaLogger.Error(err, "Error on creating syncing context")
		return gitopsSyncCommon.OperationError, err.Error()
	}

	syncCtx.Sync()

	phase, message, _ := syncCtx.GetState()
	return phase, message
}

func (s *Syncer) compareState(gitSync *v1alpha1.GitSync, targetObjs []*unstructured.Unstructured) (gitopsSync.ReconciliationResult, *diff.DiffResultList, error) {
	var infoProvider kubeUtil.ResourceInfoProvider
	clusterCache, err := s.stateCache.GetClusterCache()
	infoProvider = clusterCache
	if err != nil {
		infoProvider = &resourceInfoProviderStub{}
	}
	liveObjByKey, err := s.stateCache.GetManagedLiveObjs(gitSync, targetObjs)
	if err != nil {
		return gitopsSync.ReconciliationResult{}, nil, err
	}
	reconciliationResult := gitopsSync.Reconcile(targetObjs, liveObjByKey, gitSync.Spec.Destination.Namespace, infoProvider)

	// Ignore `status` field for all comparison.
	// TODO: make it configurable
	overrides := map[string]ResourceOverride{
		"*/*": {
			IgnoreDifferences: OverrideIgnoreDiff{JSONPointers: []string{"/status"}}},
	}

	resourceOps, cleanup, err := s.getResourceOperations()
	if err != nil {
		log.Errorf("CompareAppState error getting resource operations: %s", err)
	}
	defer cleanup()

	diffOpts := []diff.Option{
		diff.WithLogr(*logger.New().WithValues("gitsync", fmt.Sprintf("%s/%s", gitSync.Namespace, gitSync.Name)).LogrLogger),
		diff.WithServerSideDiff(true),
		diff.WithServerSideDryRunner(diff.NewK8sServerSideDryRunner(resourceOps)),
		diff.WithManager(common.SSAManager),
		diff.WithGVKParser(clusterCache.GetGVKParser()),
	}

	diffResults, err := StateDiffs(reconciliationResult.Target, reconciliationResult.Live, overrides, diffOpts)
	if err != nil {
		return reconciliationResult, nil, err
	}

	return reconciliationResult, diffResults, nil
}

// getResourceOperations will return the kubectl implementation of the ResourceOperations
// interface that provides functionality to manage kubernetes resources. Returns a
// cleanup function that must be called to remove the generated kube config for this
// server.
func (s *Syncer) getResourceOperations() (kube.ResourceOperations, func(), error) {
	clusterCache, err := s.stateCache.GetClusterCache()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting cluster cache: %w", err)
	}

	ops, cleanup, err := s.kubectl.ManageResources(s.config, clusterCache.GetOpenAPISchema())
	if err != nil {
		return nil, nil, fmt.Errorf("error creating kubectl ResourceOperations: %w", err)
	}
	return ops, cleanup, nil
}

func applyAnnotationAndNamespace(manifests []*unstructured.Unstructured, gitSyncName, destinationNamespace string) ([]*unstructured.Unstructured, error) {
	uns := make([]*unstructured.Unstructured, 0)
	for _, m := range manifests {
		err := kubernetes.SetGitSyncInstanceAnnotation(m, common.AnnotationKeyGitSyncInstance, gitSyncName)
		if err != nil {
			return nil, err
		}
		m.SetNamespace(destinationNamespace)
		uns = append(uns, m)
	}
	return uns, nil
}

// updateCommitStatus will update the commit status in git sync CR.
// If an error occurred while syncing the target state, then it
// updates the error reason as well.
func updateCommitStatus(
	ctx context.Context,
	kubeClient client.Client,
	numaLogger *logger.NumaLogger,
	namespacedName types.NamespacedName,
	hash string,
	synced bool,
	errMsg string,
) error {
	commitStatus := v1alpha1.CommitStatus{
		Hash:     hash,
		Synced:   synced,
		SyncTime: metav1.NewTime(time.Now()),
	}

	if !synced {
		commitStatus.Error = errMsg
	}

	gitSync := &v1alpha1.GitSync{}
	if err := kubeClient.Get(ctx, namespacedName, gitSync); err != nil {
		// if we aren't able to do a Get, then either it's been deleted in the past, or something else went wrong
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			numaLogger.Error(err, "Unable to get GitSync", "err")
			return err
		}
	}
	currentCommitStatus := gitSync.Status.CommitStatus
	// Skip the commit status update if the content are the same.
	if currentCommitStatus != nil && currentCommitStatus.Synced == commitStatus.Synced &&
		currentCommitStatus.Hash == commitStatus.Hash &&
		currentCommitStatus.Error == commitStatus.Error {
		return nil
	}
	gitSync.Status.CommitStatus = &commitStatus
	if err := kubeClient.Status().Update(ctx, gitSync); err != nil {
		numaLogger.Error(err, "Error Updating GitSync Status", "err")
		return err
	}
	return nil
}

// GetLiveManagedObjects retrieves live managed objects from the provided cache for the given GitSync configuration.
// It returns a map of Kubernetes resource keys to unstructured objects and an error if any.
func GetLiveManagedObjects(cache LiveStateCache, gitSync *v1alpha1.GitSync) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	var unstructuredObj []*unstructured.Unstructured
	objs, err := cache.GetManagedLiveObjs(gitSync, unstructuredObj)
	if err != nil {
		return nil, err
	}
	return objs, nil
}

// CascadeDeletion performs cascading deletion of live managed objects using GitSync.
// It utilizes the provided context, Kubernetes client, cache, and GitSync configuration.
// Returns an error if the deletion process encounters any issues.
func CascadeDeletion(ctx context.Context, k8sClient client.Client, cache LiveStateCache, gitSync *v1alpha1.GitSync) error {
	numaLogger := logger.FromContext(ctx)
	objects, err := GetLiveManagedObjects(cache, gitSync)
	if err != nil {
		numaLogger.Debug("Live managed objects not found")
		return err
	}
	err = kubernetes.DeleteManagedObjects(ctx, k8sClient, objects)
	if err != nil {
		return err
	}
	return nil
}

func ProcessGitSyncDeletion(ctx context.Context, gitSync *v1alpha1.GitSync, s *Syncer) error {
	config, err := controllerConfig.GetConfigManagerInstance().GetConfig()
	if err != nil {
		return err
	}
	// if cascade deletion is enabled in the config ,Delete the linked resources to the GitSync
	if config.CascadeDeletion {
		return CascadeDeletion(ctx, s.client, s.stateCache, gitSync)
	}
	return nil
}
