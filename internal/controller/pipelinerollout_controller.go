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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/numaproj-labs/numaplane/internal/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/util/logger"
	apiv1 "github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
)

// PipelineRolloutReconciler reconciles a PipelineRollout object
type PipelineRolloutReconciler struct {
	client     client.Client
	scheme     *runtime.Scheme
	restConfig *rest.Config
}

func NewPipelineRolloutReconciler(
	client client.Client,
	s *runtime.Scheme,
	restConfig *rest.Config,
) *PipelineRolloutReconciler {
	return &PipelineRolloutReconciler{
		client,
		s,
		restConfig,
	}
}

//+kubebuilder:rbac:groups=numaplane.numaproj.io.github.com.numaproj-labs,resources=pipelinerollouts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=numaplane.numaproj.io.github.com.numaproj-labs,resources=pipelinerollouts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=numaplane.numaproj.io.github.com.numaproj-labs,resources=pipelinerollouts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PipelineRollout object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *PipelineRolloutReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// update the Base Logger's level according to the Numaplane Config
	logger.RefreshBaseLoggerLevel()
	numaLogger := logger.GetBaseLogger().WithName("reconciler").WithValues("pipelinerollout", req.NamespacedName)

	numaLogger.Info("PipelineRollout Reconcile")

	pipelineRollout := &apiv1.PipelineRollout{}
	if err := r.client.Get(ctx, req.NamespacedName, pipelineRollout); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			numaLogger.Error(err, "Unable to get PipelineRollout", "request", req)
			return ctrl.Result{}, err
		}
	}

	numaLogger.Debugf("reconciling Pipeline definition Object: %+v", pipelineRollout.Spec.Pipeline)

	// todo:
	// 1. validate
	// 2. make sure all fields are in what we pass to UpdateCRSpec() (e.g. namespace)
	// 3. Add OwnerReference

	resourceInfo, err := kubernetes.ParseRuntimeExtension(ctx, pipelineRollout.Spec.Pipeline, "pipelines")
	if err != nil {
		return ctrl.Result{}, err
	}

	err = kubernetes.UpdateCRSpec(ctx, r.restConfig, resourceInfo)
	if err != nil {
		numaLogger.Debugf("error reconciling: %v", err)
	}
	/*
		var pipelineAsMap map[string]interface{}
		err := json.Unmarshal(pipelineRollout.Spec.Pipeline.Raw, &pipelineAsMap)
		if err != nil {
			numaLogger.Error(err, "error unmarshaling json")
		} else {
			numaLogger.Infof("pipeline as map: %+v", pipelineAsMap)
			unstruc := &unstructured.Unstructured{Object: pipelineAsMap}
			numaLogger.Infof("as unstructured: %+v", unstruc)

		}*/

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineRolloutReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.PipelineRollout{}).
		Complete(r)
}