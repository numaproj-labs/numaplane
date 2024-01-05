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

	// gitSyncProcessors maps namespaced name of each GitSync CRD to GitSyncProcessor
	gitSyncProcessors map[string]*git.GitSyncProcessor
}

const (
	finalizerName = "numaplane-controller"

	clusterName = "staging-usw2-k8s" // TODO: just for testing - add to a ConfigMap?
)

func NewGitSyncReconciler(c client.Client, s *runtime.Scheme) *GitSyncReconciler {
	return &GitSyncReconciler{
		Client:            c,
		Scheme:            s,
		gitSyncProcessors: make(map[string]*git.GitSyncProcessor),
	}
}

//+kubebuilder:rbac:groups=numaplane.numaproj.io.github.com.numaproj-labs,resources=gitsyncs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=numaplane.numaproj.io.github.com.numaproj-labs,resources=gitsyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=numaplane.numaproj.io.github.com.numaproj-labs,resources=gitsyncs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GitSync object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *GitSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logging.FromContext(ctx)

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

	gitSyncOrig := gitSync
	gitSync = gitSync.DeepCopy()

	// We will have a GitSyncProcessor object for every GitSync that contains our cluster
	// Therefore, we Create or Update it if our cluster is in the GitSync
	// We Delete it if either the GitSync is deleted or our cluster is removed from it

	forThisCluster := gitSync.Spec.ContainsClusterDestination(clusterName)
	_, forThisClusterPrev := r.gitSyncProcessors[gitSync.String()]
	shouldDelete := !gitSync.DeletionTimestamp.IsZero() || (forThisClusterPrev && !forThisCluster)
	shouldApply := gitSync.DeletionTimestamp.IsZero() && forThisCluster

	logger.Debugw("GitSync values: ", "forThisCluster", forThisCluster, "forThisClusterPrev", forThisClusterPrev, "deletionTimestamp", gitSync.DeletionTimestamp,
		"shouldDelete", shouldDelete, "shouldApply", shouldApply)

	if shouldDelete {
		logger.Infow("Received request to delete GitSync", "GitSync", gitSync)
		err := r.deleteGitSync(ctx, gitSync)
		if err != nil {
			logger.Errorw("GitSync Deletion error", "err", err, "GitSync", gitSync)
			return ctrl.Result{}, err
		}

	} else if shouldApply {

		logger.Debugw("Received request to create or update GitSync", "GitSync", gitSync)

		//  if it doesn't exist in our map, we create it and add it to the map (this applies either to a new CRD just created, or in the case that this app has restarted)
		//  if it already exists in our map, we call Update()

		// first validate it
		err := r.validate(gitSync)
		if err != nil {
			logger.Errorw("Validation failed", "err", err, "GitSync", gitSync)
			// TODO: update Conditions/Phase as needed
			return ctrl.Result{}, err
		}

		processor, found := r.gitSyncProcessors[req.NamespacedName.String()]
		if !found {
			err := r.addGitSync(ctx, gitSync)
			if err != nil {
				logger.Errorw("Error creating GitSync", "err", err, "GitSync", gitSync)
				return ctrl.Result{}, err
			}
		} else {
			logger.Infow("Updating existing GitSync", "GitSync", gitSync)
			processor.Update(gitSync)
		}

		// TODO: update Conditions and Phase

	} else {
		return ctrl.Result{}, nil
	}

	if needsUpdate(gitSyncOrig, gitSync) {
		// Update with a DeepCopy because .Status will be cleaned up.
		//gitSyncCopied := gitSync.DeepCopy()
		gitSyncStatus := gitSync.Status // TODO: test if this in fact gets wiped and re-added below
		if err := r.Client.Update(ctx, gitSync /*gitSyncCopied*/); err != nil {
			logger.Errorw("Error Updating GitSync", "err", err, "GitSync", gitSync)
			return ctrl.Result{}, err
		}
		gitSync.Status = gitSyncStatus
	}
	if !shouldDelete { // would've already been deleted
		// TODO: make thread safe?
		if err := r.Client.Status().Update(ctx, gitSync); err != nil {
			logger.Errorw("Error Updating GitSync Status", "err", err, "GitSync", gitSync)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *GitSyncReconciler) addGitSync(ctx context.Context, gitSync *apiv1.GitSync) error {
	logger := logging.FromContext(ctx)

	// this is either a new CRD just created, or otherwise the app may have restarted
	logger.Info("GitSync not found, so adding", "GitSync", gitSync)

	if !controllerutil.ContainsFinalizer(gitSync, finalizerName) { // TODO: make sure this is locked
		controllerutil.AddFinalizer(gitSync, finalizerName)
	}

	processor, err := git.NewGitSyncProcessor(gitSync, r.Client)
	if err != nil {
		logger.Error(err, "Error creating GitSyncProcessor", "GitSync", gitSync)
	}
	r.gitSyncProcessors[gitSync.String()] = processor

	return nil
}

/*func (r *GitSyncReconciler) updateGitSync(gitSync *apiv1.GitSync) error {

}*/

func (r *GitSyncReconciler) deleteGitSync(ctx context.Context, gitSync *apiv1.GitSync) error {
	logger := logging.FromContext(ctx)

	if controllerutil.ContainsFinalizer(gitSync, finalizerName) {

		// find it in the map and call Shutdown(), and then delete it from the map
		processor, found := r.gitSyncProcessors[gitSync.String()]
		if !found {
			// todo: should this be a warning? Is it reasonable to get here?
			logger.Info("Unexpected: GitSync not found in map to delete it", "GitSync", gitSync)
			return nil
		}
		err := processor.Shutdown()
		if err != nil {
			logger.Error(err, "Error shutting down GitSync", "GitSync", gitSync)
			return err
		}
		delete(r.gitSyncProcessors, gitSync.String())
		logger.Info("Deleted GitSync", "GitSync", gitSync)

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
	// TODO: any official guideline?
	if !equality.Semantic.DeepEqual(old.Finalizers, new.Finalizers) {
		return true
	}
	return false
}
