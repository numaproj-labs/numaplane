/*
Copyright 2023 The Numaproj Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"os"
	"sync"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/numaproj-labs/numaplane/internal/git"

	apiv1 "github.com/numaproj-labs/numaplane/api/v1"
	"github.com/numaproj-labs/numaplane/internal/shared/logging"
)

// GitSyncReconciler reconciles a GitSync object
type GitSyncReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// gitSyncLocks maps GitSync namespaced name to Mutex, to prevent processing the same GitSync at the same time
	// note that if other goroutines outside of this struct need to share the lock in the future, it can be moved
	gitSyncLocks sync.Map

	// gitSyncProcessors maps namespaced name of each GitSync CRD to GitSyncProcessor
	gitSyncProcessors sync.Map

	// clusterName is the name of the cluster we're running on - used to determine which GitSyncs to store in memory
	clusterName string
}

const (
	finalizerName = "numaplane-controller"
)

func NewGitSyncReconciler(c client.Client, s *runtime.Scheme) (*GitSyncReconciler, error) {
	clusterName, found := os.LookupEnv("CLUSTER_NAME") // TODO: if we incorporate a ConfigMap later, could include this in it
	if !found {
		return nil, fmt.Errorf("environment variable CLUSTER_NAME not found")
	}
	return &GitSyncReconciler{
		Client:      c,
		Scheme:      s,
		clusterName: clusterName,
	}, nil
}

//+kubebuilder:rbac:groups=numaplane.numaproj.io.github.com.numaproj-labs,resources=gitsyncs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=numaplane.numaproj.io.github.com.numaproj-labs,resources=gitsyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=numaplane.numaproj.io.github.com.numaproj-labs,resources=gitsyncs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *GitSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logging.FromContext(ctx)

	// since this function can be called by multiple goroutines at the same time, lock so we only process a given GitSync one at a time
	lockAsInterface, foundLock := r.gitSyncLocks.Load(req.NamespacedName.String())
	if !foundLock {
		r.gitSyncLocks.Store(req.NamespacedName.String(), &sync.Mutex{})
		lockAsInterface, _ = r.gitSyncLocks.Load(req.NamespacedName.String())
	}
	lock := lockAsInterface.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	// get the GitSync CRD - if not found, it may have been deleted in the past
	gitSync := &apiv1.GitSync{}
	if err := r.Client.Get(ctx, req.NamespacedName, gitSync); err != nil {
		// if we aren't able to do a Get, then either it's been deleted in the past, or something else went wrong
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			logger.Errorw("Unable to get GitSync", "err", err, "request", req)
			return ctrl.Result{}, err
		}
	}

	gitSyncOrig := gitSync // save this off so we can compare it later
	gitSync = gitSync.DeepCopy()

	gitSync.Status.InitConditions()

	result, err := r.reconcile(ctx, gitSync)
	if err != nil {
		return result, err
	}

	// Update the Spec if needed
	if needsUpdate(gitSyncOrig, gitSync) {
		gitSyncStatus := gitSync.Status
		if err := r.Client.Update(ctx, gitSync); err != nil {
			logger.Errorw("Error Updating GitSync", "err", err, "GitSync", gitSync)
			return ctrl.Result{}, err
		}
		// restore the original status, which would've been wiped in the previous call to Update()
		gitSync.Status = gitSyncStatus

	}

	// Update the Status subresource
	if gitSync.DeletionTimestamp.IsZero() { // would've already been deleted
		if err := r.Client.Status().Update(ctx, gitSync); err != nil {
			logger.Errorw("Error Updating GitSync Status", "err", err, "GitSync", gitSync)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcile does the real logic
func (r *GitSyncReconciler) reconcile(ctx context.Context, gitSync *apiv1.GitSync) (ctrl.Result, error) {
	logger := logging.FromContext(ctx)

	// We will have a GitSyncProcessor object for every GitSync that contains our cluster
	// Therefore, we Create or Update it if our cluster is defined as a Destination in the GitSync
	// We Delete it if either the GitSync is deleted or our cluster is removed from it

	forThisCluster := gitSync.DeletionTimestamp.IsZero() && gitSync.Spec.ContainsClusterDestination(r.clusterName)
	_, forThisClusterPrev := r.gitSyncProcessors.Load(gitSync.String())
	shouldDelete := !forThisCluster && forThisClusterPrev
	shouldAdd := forThisCluster && !forThisClusterPrev
	shouldUpdate := forThisCluster && forThisClusterPrev

	logger.Debugw("GitSync values: ", "forThisCluster", forThisCluster, "forThisClusterPrev", forThisClusterPrev, "deletionTimestamp", gitSync.DeletionTimestamp,
		"shouldDelete", shouldDelete, "shoulAdd", shouldAdd, "shouldUpdate", shouldUpdate)

	if shouldDelete {
		logger.Infow("Received request to stop watching GitSync repos", "GitSync", gitSync)
		err := r.deleteGitSyncProcessor(ctx, gitSync)
		if err != nil {
			logger.Errorw("GitSyncProcessor Deletion error", "err", err, "GitSync", gitSync)
			return ctrl.Result{}, err
		}

	} else if shouldAdd {

		logger.Debugw("Received request to watch GitSync repos", "GitSync", gitSync)

		// first validate it
		err := r.validate(gitSync)
		if err != nil {
			logger.Errorw("Validation failed", "err", err, "GitSync", gitSync)
			gitSync.Status.MarkNotConfigured("InvalidSpec", err.Error())
			return ctrl.Result{}, err
		}

		err = r.addGitSyncProcessor(ctx, gitSync)
		if err != nil {
			logger.Errorw("Error creating GitSyncProcessor", "err", err, "GitSync", gitSync)
			gitSync.Status.MarkNotConfigured("CreationFailure", err.Error())
			return ctrl.Result{}, err
		}

		gitSync.Status.MarkConfigured()

	} else if shouldUpdate {
		logger.Debugw("Received request to update GitSync", "GitSync", gitSync)

		// first validate it
		err := r.validate(gitSync)
		if err != nil {
			logger.Errorw("Validation failed", "err", err, "GitSync", gitSync)
			gitSync.Status.MarkNotConfigured("InvalidSpec", err.Error())
			return ctrl.Result{}, err
		}

		processorAsInterface, _ := r.gitSyncProcessors.Load(gitSync.String())
		processor := processorAsInterface.(*git.GitSyncProcessor)
		logger.Infow("Updating existing GitSync", "GitSync", gitSync)
		err = processor.Update(gitSync)
		if err != nil {
			logger.Errorw("Error updating GitSync", "err", err, "GitSync", gitSync)
			gitSync.Status.MarkNotConfigured("UpdateFailure", err.Error())
			return ctrl.Result{}, err
		}

		gitSync.Status.MarkConfigured() // should already be but just in case
	}

	return ctrl.Result{}, nil

}

func (r *GitSyncReconciler) addGitSyncProcessor(ctx context.Context, gitSync *apiv1.GitSync) error {
	logger := logging.FromContext(ctx)

	// this is either a new CRD just created, or otherwise the app may have restarted
	logger.Infow("GitSyncProcessor not found, so adding", "GitSync", gitSync)

	// add Finalizer so we can ensure that we take appropriate action when CRD is deleted
	if !controllerutil.ContainsFinalizer(gitSync, finalizerName) {
		controllerutil.AddFinalizer(gitSync, finalizerName)
	}

	processor, err := git.NewGitSyncProcessor(gitSync, r.Client)
	if err != nil {
		logger.Errorw("Error creating GitSyncProcessor", "err", err, "GitSync", gitSync)
	}
	r.gitSyncProcessors.Store(gitSync.String(), processor)
	logger.Infow("Started watching GitSync repos", "GitSync", gitSync)

	return nil
}

func (r *GitSyncReconciler) deleteGitSyncProcessor(ctx context.Context, gitSync *apiv1.GitSync) error {
	logger := logging.FromContext(ctx)

	if controllerutil.ContainsFinalizer(gitSync, finalizerName) {

		// find it in the map and call Shutdown(), and then delete it from the map
		processorAsInterface, found := r.gitSyncProcessors.Load(gitSync.String())
		if !found {
			logger.Warnw("Unexpected: GitSyncProcessor not found in map to delete it", "GitSync", gitSync)
			return nil
		}
		processor := processorAsInterface.(*git.GitSyncProcessor)
		err := processor.Shutdown()
		if err != nil {
			logger.Errorw("Error shutting down GitSync", "err", err, "GitSync", gitSync)
			return err
		}
		r.gitSyncProcessors.Delete(gitSync.String())
		logger.Infow("Stopped watching GitSync repos", "GitSync", gitSync)

		controllerutil.RemoveFinalizer(gitSync, finalizerName)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GitSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.GitSync{}).
		Complete(r)
}

// TODO: add validation
func (r *GitSyncReconciler) validate(gitSync *apiv1.GitSync) error {
	return nil
}

func needsUpdate(old, new *apiv1.GitSync) bool {

	if old == nil {
		return true
	}
	// check for any fields we might update in the Spec - generally we'd only update a Finalizer or maybe something in the metadata
	// TODO: we would need to update this if we ever add anything else, like a label or annotation - unless there's a generic check that makes sense
	if !equality.Semantic.DeepEqual(old.Finalizers, new.Finalizers) {
		return true
	}
	return false
}
