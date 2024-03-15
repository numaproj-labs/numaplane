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

package main

import (
	"context"
	"flag"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimecontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apiv1 "github.com/numaproj-labs/numaplane/api/v1alpha1"
	"github.com/numaproj-labs/numaplane/internal/controller"
	"github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/sync"
	"github.com/numaproj-labs/numaplane/internal/util/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/util/logging"
)

const (
	ControllerGitSync = "gitsync-controller"
)

var (
	// scheme is the runtime.Scheme to which all Numaplane API types are registered.
	scheme = runtime.NewScheme()
	// logger is the global logger for the controller-manager.
	logger     = logging.NewLogger().Named("controller-manager")
	configPath = "/etc/numaplane" // Path in the volume mounted in the pod where yaml is present
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(apiv1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "numaplane-controller-lock",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		logger.Fatalw("Unable to get a controller-runtime manager", zap.Error(err))
	}

	// Load Config For the pod
	configManager := config.GetConfigManagerInstance()
	err = configManager.LoadConfig(func(err error) {
		logger.Errorw("Failed to reload global configuration file", zap.Error(err))
	}, configPath, "config", "yaml")
	if err != nil {
		logger.Fatalw("Failed to load config file", zap.Error(err))
	}

	kubectl := kubernetes.NewKubectl()
	syncer := sync.NewSyncer(
		mgr.GetClient(),
		mgr.GetConfig(),
		mgr.GetConfig(),
		kubectl)
	// Add syncer runner
	if err = mgr.Add(LeaderElectionRunner(syncer.Start)); err != nil {
		logger.Fatalw("Unable to add autoscaling runner", zap.Error(err))
	}
	gitSyncReconciler, err := controller.NewGitSyncReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		configManager,
		syncer,
	)
	if err != nil {
		logger.Fatalw("Unable to create GitSync controller", zap.Error(err))
	}

	gitSyncController, err := runtimecontroller.New(ControllerGitSync, mgr, runtimecontroller.Options{
		Reconciler: gitSyncReconciler,
	})
	if err != nil {
		logger.Fatalw("Unable to set GitSync controller", zap.Error(err))
	}

	// Watch GitSync objects
	if err := gitSyncController.Watch(&source.Kind{Type: &apiv1.GitSync{}}, &handler.EnqueueRequestForObject{},
		predicate.Or(
			predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{},
		)); err != nil {
		logger.Fatalw("Unable to watch GitSyncs", zap.Error(err))
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Fatalw("unable to set up health check", zap.Error(err))
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Fatalw("unable to set up ready check", zap.Error(err))
	}

	logger.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Fatalw("Unable to start manager", zap.Error(err))
	}
}

// LeaderElectionRunner is used to convert a function to be able to run as a LeaderElectionRunnable.
type LeaderElectionRunner func(ctx context.Context) error

func (ler LeaderElectionRunner) Start(ctx context.Context) error {
	return ler(ctx)
}

func (ler LeaderElectionRunner) NeedLeaderElection() bool {
	return true
}

var _ manager.Runnable = (*LeaderElectionRunner)(nil)
var _ manager.LeaderElectionRunnable = (*LeaderElectionRunner)(nil)
