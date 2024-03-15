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
	kubeUtil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	"github.com/numaproj-labs/numaplane/internal/common"
	controllerConfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/git"
	"github.com/numaproj-labs/numaplane/internal/util/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/util/logging"
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
func NewSyncer(client client.Client, config *rest.Config, rawConfig *rest.Config, kubectl kubeUtil.Kubectl, opts ...Option) *Syncer {
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
	log := logging.FromContext(ctx).Named("synchronizer")
	log.Info("Starting synchronizer...")
	keyCh := make(chan string)
	ctx, cancel := context.WithCancel(logging.WithLogger(ctx, log))
	defer cancel()

	s.stateCache.Init()

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
	log := logging.FromContext(ctx)
	log.Infof("Started synchronizer worker %v", id)
	for {
		select {
		case <-ctx.Done():
			log.Infof("Stopped synchronizer worker %v", id)
			return
		case key := <-keyCh:
			if err := s.runOnce(ctx, key, id); err != nil {
				log.Errorw("Failed to execute a task", zap.String("gitSyncKey", key), zap.Error(err))
			}
		}
	}
}

// Function runOnce implements the logic of each synchronization.
func (s *Syncer) runOnce(ctx context.Context, key string, worker int) error {
	log := logging.FromContext(ctx).With("worker", fmt.Sprint(worker)).With("gitSyncKey", key)
	log.Debugf("Working on key: %s.", key)
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
			log.Info("No corresponding GitSync found, stopped watching.")
			return nil
		}
		return fmt.Errorf("failed to query GitSync object of key %q, %w", key, err)
	}
	if !gitSync.GetDeletionTimestamp().IsZero() {
		s.StopWatching(key)
		log.Debug("GitSync object being deleted.")
		return nil
	}

	globalConfig, err := controllerConfig.GetConfigManagerInstance().GetConfig()
	if err != nil {
		log.Errorw("error getting  the  global config", "err", err)
	}
	repo, err := git.CloneRepo(ctx, s.client, gitSync, globalConfig)
	if err != nil {
		return fmt.Errorf("failed to clone the repo of key %q, %w", key, err)
	}
	manifests, err := git.GetLatestManifests(ctx, repo, s.client, gitSync)
	if err != nil {
		return fmt.Errorf("failed to get the manifest of key %q, %w", key, err)
	}
	uns, err := toUnstructuredAndApplyAnnotation(manifests, gitSyncName)
	if err != nil {
		return fmt.Errorf("failed to parse the manifest of key %q, %w", key, err)
	}
	synced := s.sync(gitSync, uns, log)
	if synced {
		log.Info("GitSync object is successfully synced.")
	}
	// TODO: commit the status

	return err
}

type resourceInfoProviderStub struct {
}

func (r *resourceInfoProviderStub) IsNamespaced(_ schema.GroupKind) (bool, error) {
	return false, nil
}

func (s *Syncer) sync(gitSync *v1alpha1.GitSync, targetObjs []*unstructured.Unstructured, logger *zap.SugaredLogger) bool {
	logEntry := log.WithFields(log.Fields{"gitsync": gitSync})

	reconciliationResult, modified, err := s.compareState(gitSync, targetObjs)
	if err != nil {
		return false
	}

	// If the live state match the target state, then skip the syncing.
	if !modified {
		logger.Info("GitSync object is successfully already synced, skip the syncing.")
		return true
	}

	opts := []gitopsSync.SyncOpt{
		gitopsSync.WithLogr(logging.NewLogrusLogger(logEntry)),
		gitopsSync.WithOperationSettings(false, true, false, false),
		gitopsSync.WithManifestValidation(true),
		gitopsSync.WithPruneLast(true),
		gitopsSync.WithReplace(false),
		gitopsSync.WithServerSideApply(true),
		gitopsSync.WithServerSideApplyManager(common.SSAManager),
	}

	cluster, err := s.stateCache.GetClusterCache()
	if err != nil {
		return false
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
		return false
	}

	syncCtx.Sync()

	phase, _, _ := syncCtx.GetState()
	return phase.Successful()
}

func (s *Syncer) compareState(gitSync *v1alpha1.GitSync, targetObjs []*unstructured.Unstructured) (gitopsSync.ReconciliationResult, bool, error) {
	var infoProvider kubeUtil.ResourceInfoProvider
	infoProvider, err := s.stateCache.GetClusterCache()
	if err != nil {
		infoProvider = &resourceInfoProviderStub{}
	}
	liveObjByKey, err := s.stateCache.GetManagedLiveObjs(gitSync, targetObjs)
	if err != nil {
		return gitopsSync.ReconciliationResult{}, false, err
	}
	reconciliationResult := gitopsSync.Reconcile(targetObjs, liveObjByKey, gitSync.Spec.Destination.Namespace, infoProvider)

	// Ignore `status` field for all comparison.
	// TODO: make it configurable
	overrides := map[string]ResourceOverride{
		"*/*": {
			IgnoreDifferences: OverrideIgnoreDiff{JSONPointers: []string{"/status"}}},
	}

	logEntry := log.WithFields(log.Fields{"gitsync": gitSync})

	diffOpts := []diff.Option{
		diff.WithLogr(logging.NewLogrusLogger(logEntry)),
	}

	modified, err := StateDiffs(reconciliationResult.Target, reconciliationResult.Live, overrides, diffOpts)
	if err != nil {
		return reconciliationResult, false, err
	}

	return reconciliationResult, modified.Modified, nil
}

func toUnstructuredAndApplyAnnotation(manifests []string, gitSyncName string) ([]*unstructured.Unstructured, error) {
	uns := make([]*unstructured.Unstructured, 0)
	for _, m := range manifests {
		obj := make(map[string]interface{})
		err := yaml.Unmarshal([]byte(m), &obj)
		if err != nil {
			return nil, err
		}
		target := &unstructured.Unstructured{Object: obj}
		err = kubernetes.SetGitSyncInstanceAnnotation(target, common.AnnotationKeyGitSyncInstance, gitSyncName)
		if err != nil {
			return nil, err
		}
		uns = append(uns, target)
	}
	return uns, nil
}
