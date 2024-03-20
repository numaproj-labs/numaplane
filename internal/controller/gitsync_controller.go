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

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/sync"
	gitshared "github.com/numaproj-labs/numaplane/internal/util/git"
	kubernetesshared "github.com/numaproj-labs/numaplane/internal/util/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/util/logging"
	apiv1 "github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
)

// GitSyncReconciler reconciles a GitSync object
type GitSyncReconciler struct {
	ConfigManager *config.ConfigManager

	client client.Client
	scheme *runtime.Scheme
	syncer *sync.Syncer

	// clusterName is the name of the cluster we're running on - used to determine which GitSyncs to store in memory
	clusterName string
}

const (
	finalizerName = "numaplane-controller"
)

func NewGitSyncReconciler(
	client client.Client,
	s *runtime.Scheme,
	configManager *config.ConfigManager,
	syncer *sync.Syncer,
) (*GitSyncReconciler, error) {
	getConfig, err := configManager.GetConfig()
	if err != nil {
		return nil, err
	}
	return &GitSyncReconciler{
		client:        client,
		scheme:        s,
		clusterName:   getConfig.ClusterName,
		ConfigManager: configManager,
		syncer:        syncer,
	}, nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Note this method is called concurrently by multiple goroutines, whenever there is a change to the GitSync CRD
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *GitSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logging.FromContext(ctx)

	// get the GitSync CR - if not found, it may have been deleted in the past
	gitSync := &apiv1.GitSync{}
	if err := r.client.Get(ctx, req.NamespacedName, gitSync); err != nil {
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

	err := r.reconcile(ctx, gitSync)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update the Spec if needed
	if needsUpdate(gitSyncOrig, gitSync) {
		gitSyncStatus := gitSync.Status
		if err := r.client.Update(ctx, gitSync); err != nil {
			logger.Errorw("Error Updating GitSync", "err", err, "GitSync", gitSync)
			return ctrl.Result{}, err
		}
		// restore the original status, which would've been wiped in the previous call to Update()
		gitSync.Status = gitSyncStatus
	}

	// Update the Status subresource
	if gitSync.DeletionTimestamp.IsZero() { // would've already been deleted
		if err := r.client.Status().Update(ctx, gitSync); err != nil {
			logger.Errorw("Error Updating GitSync Status", "err", err, "GitSync", gitSync)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcile does the real logic
func (r *GitSyncReconciler) reconcile(ctx context.Context, gitSync *apiv1.GitSync) error {
	logger := logging.FromContext(ctx)

	if !gitSync.Spec.ContainsClusterDestination(r.clusterName) {
		gitSync.Status.MarkNotApplicable("ClusterMismatch", "This cluster isn't a destination")
		return nil
	}

	gitSyncKey := sync.KeyOfGitSync(gitSync)
	if !gitSync.DeletionTimestamp.IsZero() {
		logger.Infow("Deleting", "GitSync", gitSync)
		if r.syncer != nil {
			r.syncer.StopWatching(gitSyncKey)
		}
		if controllerutil.ContainsFinalizer(gitSync, finalizerName) {
			controllerutil.RemoveFinalizer(gitSync, finalizerName)
		}
		return nil
	}

	// first validate it
	err := r.validate(gitSync)
	if err != nil {
		logger.Errorw("Validation failed", "err", err, "GitSync", gitSync)
		gitSync.Status.MarkFailed("InvalidSpec", err.Error())
		return err
	}

	// add Finalizer so we can ensure that we take appropriate action when CRD is deleted
	if !controllerutil.ContainsFinalizer(gitSync, finalizerName) {
		controllerutil.AddFinalizer(gitSync, finalizerName)
	}

	if r.syncer != nil {
		r.syncer.StartWatching(gitSyncKey)
	}

	gitSync.Status.MarkRunning() // should already be but just in case
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GitSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.GitSync{}).WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func (r *GitSyncReconciler) validate(gitSync *apiv1.GitSync) error {
	specs := gitSync.Spec
	destination := gitSync.Spec.Destination

	// Validate the repositoryPath
	if ok := gitshared.CheckGitURL(specs.RepoUrl); !ok {
		return fmt.Errorf("invalid remote repository url %s", specs.RepoUrl)
	}
	if len(specs.TargetRevision) == 0 {
		return fmt.Errorf("targetRevision cannot be empty for repository Path %s", specs.RepoUrl)
	}
	// Validate destination
	if len(destination.Cluster) == 0 {
		return fmt.Errorf("cluster name cannot be empty")
	}
	if !kubernetesshared.IsValidKubernetesNamespace(destination.Namespace) {
		return fmt.Errorf("namespace is not a valid string for cluster %s", destination.Cluster)
	}

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
