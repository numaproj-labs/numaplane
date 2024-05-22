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

	gitopsSync "github.com/argoproj/gitops-engine/pkg/sync"
	gitopsSyncCommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	kubeUtil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/numaproj-labs/numaplane/internal/common"
	"github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/metrics"
	"github.com/numaproj-labs/numaplane/internal/sync"
	"github.com/numaproj-labs/numaplane/internal/util/logger"
	apiv1 "github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
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

	logger.RefreshBaseLoggerLevel()
	numaLogger := logger.GetBaseLogger().WithName("loadDefinitions").WithValues("numaflowcontrollerrollout_1")

	definitions := make(map[string]string)
	for _, definition := range definitionsConfig.ControllerDefinitions {
		definitions[definition.Version] = definition.FullSpec
		numaLogger.Info("def", "version", definition.Version, "spec", definition.FullSpec)
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

	numaflowControllerRollout := &apiv1.NumaflowControllerRollout{}
	if err := r.client.Get(ctx, req.NamespacedName, numaflowControllerRollout); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			numaLogger.Error(err, "Unable to get NumaflowControllerRollout", "request", req)
			return ctrl.Result{}, err
		}
	}

	// TODO: reconciliation logic here
	_, _ = r.sync(numaflowControllerRollout, req.Namespace, "", numaLogger)

	return ctrl.Result{}, nil
}

// TODO: share sync logic with `syncer`
func (r *NumaflowControllerRolloutReconciler) sync(
	rollout *apiv1.NumaflowControllerRollout,
	namespace string,
	revision string,
	numaLogger *logger.NumaLogger,
) (gitopsSyncCommon.OperationPhase, string) {

	// Get the target manifests
	version := rollout.Spec.Controller.Version
	manifest := r.definitions[version]
	numaLogger.Info("log manifest: " + manifest)
	targetObjs, err := kubeUtil.SplitYAML([]byte(manifest))
	if err != nil {
		return gitopsSyncCommon.OperationError, err.Error()
	}

	var infoProvider kubeUtil.ResourceInfoProvider
	clusterCache, err := r.stateCache.GetClusterCache()
	infoProvider = clusterCache
	if err != nil {
		return gitopsSyncCommon.OperationError, err.Error()
	}
	liveObjByKey, err := r.stateCache.GetManagedLiveObjs(rollout.Name, targetObjs)
	if err != nil {
		return gitopsSyncCommon.OperationError, err.Error()
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
		return gitopsSyncCommon.OperationError, err.Error()
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

	cluster, err := r.stateCache.GetClusterCache()
	if err != nil {
		numaLogger.Error(err, "Error on getting the cluster cache")
		//return gitopsSyncCommon.OperationError, err.Error()
	}
	openAPISchema := cluster.GetOpenAPISchema()

	syncCtx, cleanup, err := gitopsSync.NewSyncContext(
		revision,
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
		return gitopsSyncCommon.OperationError, err.Error()
	}

	syncCtx.Sync()

	phase, message, _ := syncCtx.GetState()
	return phase, message
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
	return ctrl.NewControllerManagedBy(mgr).
		// Reconcile NumaflowControllerRollouts when there's been a Generation changed (i.e. Spec change)
		For(&apiv1.NumaflowControllerRollout{}).WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
