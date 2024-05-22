/*
Copyright 2023.

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

	"github.com/argoproj/gitops-engine/pkg/diff"
	gitopsSync "github.com/argoproj/gitops-engine/pkg/sync"
	gitopsSyncCommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	kubeUtil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimecontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/numaproj-labs/numaplane/internal/common"
	"github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/metrics"
	"github.com/numaproj-labs/numaplane/internal/sync"
	"github.com/numaproj-labs/numaplane/internal/util/logger"
	apiv1 "github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
)

const (
	ControllerNumaflowControllerRollout = "numaflow-controller-rollout-controller"
)

// NumaflowControllerRolloutReconciler reconciles a NumaflowControllerRollout object
type NumaflowControllerRolloutReconciler struct {
	client     client.Client
	scheme     *runtime.Scheme
	restConfig *rest.Config
	rawConfig  *rest.Config
	kubectl    kubeUtil.Kubectl

	definitions map[string]string
	stateCache  sync.LiveStateCache
}

func NewNumaflowControllerRolloutReconciler(
	client client.Client,
	s *runtime.Scheme,
	rawConfig *rest.Config,
	metricServer *metrics.MetricsServer,
	kubectl kubeUtil.Kubectl,
) (*NumaflowControllerRolloutReconciler, error) {
	stateCache := sync.NewLiveStateCache(rawConfig)
	logger.RefreshBaseLoggerLevel()
	numaLogger := logger.GetBaseLogger().WithName("state cache").WithValues("numaflowcontrollerrollout")
	err := stateCache.Init(numaLogger)
	if err != nil {
		return nil, err
	}

	restConfig := metrics.AddMetricsTransportWrapper(metricServer, rawConfig)
	definitions, err := loadDefinitions()
	if err != nil {
		return nil, err
	}
	return &NumaflowControllerRolloutReconciler{
		client,
		s,
		restConfig,
		rawConfig,
		kubectl,
		definitions,
		stateCache,
	}, nil
}

func loadDefinitions() (map[string]string, error) {
	definitionsConfig, err := config.GetConfigManagerInstance().GetControllerDefinitionsConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get the controller definitions config, %v", err)
	}

	definitions := make(map[string]string)
	for _, definition := range definitionsConfig.ControllerDefinitions {
		definitions[definition.Version] = definition.FullSpec
	}
	return definitions, nil
}

//+kubebuilder:rbac:groups=numaplane.numaproj.io,resources=numaflowcontrollerrollouts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=numaplane.numaproj.io,resources=numaflowcontrollerrollouts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=numaplane.numaproj.io,resources=numaflowcontrollerrollouts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NumaflowControllerRollout object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *NumaflowControllerRolloutReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// update the Base Logger's level according to the Numaplane Config
	logger.RefreshBaseLoggerLevel()
	numaLogger := logger.GetBaseLogger().WithName("reconciler").WithValues("numaflowcontrollerrollout", req.NamespacedName)

	// TODO: only allow one controllerRollout per namespace.
	numaflowControllerRollout := &apiv1.NumaflowControllerRollout{}
	if err := r.client.Get(ctx, req.NamespacedName, numaflowControllerRollout); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			numaLogger.Error(err, "Unable to get NumaflowControllerRollout", "request", req)
			return ctrl.Result{}, err
		}
	}

	// save off a copy of the original before we modify it
	numaflowControllerRolloutOrig := numaflowControllerRollout
	numaflowControllerRollout = numaflowControllerRolloutOrig.DeepCopy()

	err := r.reconcile(ctx, numaflowControllerRollout, req.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update the Spec if needed
	if r.needsUpdate(numaflowControllerRolloutOrig, numaflowControllerRollout) {
		numaflowControllerRolloutStatus := numaflowControllerRollout.Status
		if err := r.client.Update(ctx, numaflowControllerRollout); err != nil {
			numaLogger.Error(err, "Error Updating NumaflowControllerRollout", "NumaflowControllerRollout", numaflowControllerRollout)
			return ctrl.Result{}, err
		}
		// restore the original status, which would've been wiped in the previous call to Update()
		numaflowControllerRollout.Status = numaflowControllerRolloutStatus
	}

	// Update the Status subresource
	if numaflowControllerRollout.DeletionTimestamp.IsZero() { // would've already been deleted
		if err := r.client.Status().Update(ctx, numaflowControllerRollout); err != nil {
			numaLogger.Error(err, "Error Updating NumaflowControllerRollout Status", "NumaflowControllerRollout", numaflowControllerRollout)
			return ctrl.Result{}, err
		}
	}

	numaLogger.Debug("reconciliation successful")

	return ctrl.Result{}, nil
}

func (r *NumaflowControllerRolloutReconciler) needsUpdate(old, new *apiv1.NumaflowControllerRollout) bool {

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

// reconcile does the real logic
func (r *NumaflowControllerRolloutReconciler) reconcile(
	ctx context.Context,
	controllerRollout *apiv1.NumaflowControllerRollout,
	namespace string,
) error {
	numaLogger := logger.FromContext(ctx)

	// TODO: add owner reference
	if !controllerRollout.DeletionTimestamp.IsZero() {
		numaLogger.Info("Deleting NumaflowControllerRollout")
		if controllerutil.ContainsFinalizer(controllerRollout, finalizerName) {
			controllerutil.RemoveFinalizer(controllerRollout, finalizerName)
		}
		return nil
	}

	// add Finalizer so we can ensure that we take appropriate action when CRD is deleted
	if !controllerutil.ContainsFinalizer(controllerRollout, finalizerName) {
		controllerutil.AddFinalizer(controllerRollout, finalizerName)
	}

	// apply controller
	phase, err := r.sync(controllerRollout, namespace, numaLogger)
	if err != nil {
		return err
	}

	if phase != gitopsSyncCommon.OperationSucceeded {
		return fmt.Errorf("sync operation is not successful")
	}

	controllerRollout.Status.MarkRunning()
	return nil
}

// TODO: share sync logic with `syncer`
func (r *NumaflowControllerRolloutReconciler) sync(
	rollout *apiv1.NumaflowControllerRollout,
	namespace string,
	numaLogger *logger.NumaLogger,
) (gitopsSyncCommon.OperationPhase, error) {

	// Get the target manifests
	version := rollout.Spec.Controller.Version
	manifest := r.definitions[version]
	targetObjs, err := kubeUtil.SplitYAML([]byte(manifest))
	if err != nil {
		return gitopsSyncCommon.OperationError, err
	}

	var infoProvider kubeUtil.ResourceInfoProvider
	clusterCache, err := r.stateCache.GetClusterCache()
	infoProvider = clusterCache
	if err != nil {
		return gitopsSyncCommon.OperationError, err
	}
	liveObjByKey, err := r.stateCache.GetManagedLiveObjs(rollout.Name, targetObjs)
	if err != nil {
		return gitopsSyncCommon.OperationError, err
	}
	reconciliationResult := gitopsSync.Reconcile(targetObjs, liveObjByKey, namespace, infoProvider)

	// Ignore `status` field for all comparison.
	// TODO: make it configurable
	overrides := map[string]sync.ResourceOverride{
		"*/*": {
			IgnoreDifferences: sync.OverrideIgnoreDiff{JSONPointers: []string{"/status"}}},
	}

	resourceOps, cleanup, err := r.getResourceOperations()
	if err != nil {
		return gitopsSyncCommon.OperationError, err
	}
	defer cleanup()

	diffOpts := []diff.Option{
		diff.WithLogr(*numaLogger.LogrLogger),
		diff.WithServerSideDiff(true),
		diff.WithServerSideDryRunner(diff.NewK8sServerSideDryRunner(resourceOps)),
		diff.WithManager(common.SSAManager),
		diff.WithGVKParser(clusterCache.GetGVKParser()),
	}

	diffResults, err := sync.StateDiffs(reconciliationResult.Target, reconciliationResult.Live, overrides, diffOpts)
	if err != nil {
		numaLogger.Error(err, "Error on comparing git sync state")
		return gitopsSyncCommon.OperationError, err
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

	cluster, err := r.stateCache.GetClusterCache()
	if err != nil {
		numaLogger.Error(err, "Error on getting the cluster cache")
		//return gitopsSyncCommon.OperationError, err.Error()
	}
	openAPISchema := cluster.GetOpenAPISchema()

	syncCtx, cleanup, err := gitopsSync.NewSyncContext(
		"",
		reconciliationResult,
		r.restConfig,
		r.rawConfig,
		r.kubectl,
		namespace,
		openAPISchema,
		opts...,
	)
	defer cleanup()
	if err != nil {
		numaLogger.Error(err, "Error on creating syncing context")
		return gitopsSyncCommon.OperationError, err
	}

	syncCtx.Sync()

	phase, _, _ := syncCtx.GetState()
	return phase, nil
}

// getResourceOperations will return the kubectl implementation of the ResourceOperations
// interface that provides functionality to manage kubernetes resources. Returns a
// cleanup function that must be called to remove the generated kube config for this
// server.
func (r *NumaflowControllerRolloutReconciler) getResourceOperations() (kubeUtil.ResourceOperations, func(), error) {
	clusterCache, err := r.stateCache.GetClusterCache()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting cluster cache: %w", err)
	}

	ops, cleanup, err := r.kubectl.ManageResources(r.restConfig, clusterCache.GetOpenAPISchema())
	if err != nil {
		return nil, nil, fmt.Errorf("error creating kubectl ResourceOperations: %w", err)
	}
	return ops, cleanup, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NumaflowControllerRolloutReconciler) SetupWithManager(mgr ctrl.Manager) error {

	controller, err := runtimecontroller.New(ControllerNumaflowControllerRollout, mgr, runtimecontroller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch NumaflowControllerRollouts
	if err := controller.Watch(source.Kind(mgr.GetCache(), &apiv1.NumaflowControllerRollout{}), &handler.EnqueueRequestForObject{}, predicate.GenerationChangedPredicate{}); err != nil {
		return err
	}

	// Watch Deployments of numaflow-controller
	// TODO: does this work?
	numaflowControllerDeployments := appv1.Deployment{}
	numaflowControllerDeployments.Name = "numaflow-controller"
	if err := controller.Watch(source.Kind(mgr.GetCache(), &numaflowControllerDeployments),
		handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &apiv1.NumaflowControllerRollout{}, handler.OnlyControllerOwner()),
		predicate.GenerationChangedPredicate{}); err != nil {
		return err
	}

	return nil
}
