package synchronizer

import (
	"container/list"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	"github.com/numaproj-labs/numaplane/internal/shared/logging"
)

type Synchronizer struct {
	client     client.Client
	gitSyncMap map[string]*list.Element
	// List of the GitSync namespaced name, format is "namespace/name"
	gitSyncList *list.List
	lock        *sync.RWMutex
	options     *options
}

// NewSynchronizer returns a Synchronizer instance.
func NewSynchronizer(client client.Client, opts ...Option) *Synchronizer {
	watcherOpts := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(watcherOpts)
		}
	}
	w := &Synchronizer{
		client:      client,
		options:     watcherOpts,
		gitSyncMap:  make(map[string]*list.Element),
		gitSyncList: list.New(),
		lock:        new(sync.RWMutex),
	}
	return w
}

// Contains returns if the synchronizer contains the key (namespace/name).
func (s *Synchronizer) Contains(key string) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	_, ok := s.gitSyncMap[key]
	return ok
}

// Length returns how many GitSync objects are watched
func (s *Synchronizer) Length() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.gitSyncList.Len()
}

// StartWatching put a key (namespace/name) into the synchronizer
func (s *Synchronizer) StartWatching(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.gitSyncMap[key]; !ok {
		s.gitSyncMap[key] = s.gitSyncList.PushBack(key)
	}
}

// StopWatching stops watching on the key (namespace/name)
func (s *Synchronizer) StopWatching(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if e, ok := s.gitSyncMap[key]; ok {
		_ = s.gitSyncList.Remove(e)
		delete(s.gitSyncMap, key)
	}
}

// Start function starts the synchronizer worker group.
// Each worker keeps picking up tasks (which contains GitSync keys) to sync the resources.
func (s *Synchronizer) Start(ctx context.Context) error {
	log := logging.FromContext(ctx).Named("synchronizer")
	log.Info("Starting synchronizer...")
	keyCh := make(chan string)
	ctx, cancel := context.WithCancel(logging.WithLogger(ctx, log))
	defer cancel()
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
	// It makes sure each element in the list will be assigned every N milliseconds.
	for {
		select {
		case <-ctx.Done():
			log.Info("Shutting down synchronizer job assigner.")
			return nil
		default:
			assign()
		}
		// Make sure each of the key will be assigned at most every N milliseconds.
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

// Function run() defines each of the worker's job.
// It waits for keys in the channel, and starts a synchronization job
func (s *Synchronizer) run(ctx context.Context, id int, keyCh <-chan string) {
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
func (s *Synchronizer) runOnce(ctx context.Context, key string, worker int) error {
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

	// TODO: Add the logic to synchronize the manifests for a single GitSync object.

	return nil
}
