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

	apiv1 "github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	clog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/numaproj-labs/numaplane/internal/controller"
	"github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/metrics"
	"github.com/numaproj-labs/numaplane/internal/sync"
	"github.com/numaproj-labs/numaplane/internal/util/kubernetes"
	"github.com/numaproj-labs/numaplane/internal/util/logger"
	numaflowv1 "github.com/numaproj/numaflow/pkg/apis/numaflow/v1alpha1"
)

var (
	// scheme is the runtime.Scheme to which all Numaplane API types are registered.
	scheme = runtime.NewScheme()
	// logger is the global logger for the controller-manager.
	numaLogger    = logger.New().WithName("controller-manager")
	configPath    = "/etc/numaplane" // Path in the volume mounted in the pod where yaml is present
	defConfigPath = "/etc/numarollout"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(apiv1.AddToScheme(scheme))

	utilruntime.Must(numaflowv1.AddToScheme(scheme))
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

	// Load Config For the pod
	configManager := config.GetConfigManagerInstance()
	err := configManager.LoadAllConfigs(func(err error) {
		numaLogger.Error(err, "Failed to reload global configuration file")
	},
		config.WithConfigsPath(configPath),
		config.WithConfigFileName("config"),
		config.WithDefConfigPath(defConfigPath),
		config.WithDefConfigFileName("controller_definitions"))
	if err != nil {
		numaLogger.Fatal(err, "Failed to load config file")
	}

	config, err := configManager.GetConfig()
	if err != nil {
		numaLogger.Fatal(err, "Failed to get config")
	}

	numaLogger.SetLevel(config.LogLevel)
	logger.SetBaseLogger(numaLogger)
	clog.SetLogger(*numaLogger.LogrLogger)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
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
		numaLogger.Fatal(err, "Unable to get a controller-runtime manager")
	}

	interval := config.SyncTimeIntervalMs
	metricServer, err := metrics.NewMetricsServer()
	if err != nil {
		numaLogger.Fatal(err, "Failed to initialize metric")
	}
	kubectl := kubernetes.NewKubectl()
	syncer := sync.NewSyncer(
		mgr.GetClient(),
		mgr.GetConfig(),
		metricServer,
		kubectl,
		sync.WithTaskInterval(interval))
	// Add syncer runner
	if err = mgr.Add(LeaderElectionRunner(syncer.Start)); err != nil {
		numaLogger.Fatal(err, "Unable to add syncer runner")
	}

	gitSyncReconciler, err := controller.NewGitSyncReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		configManager,
		syncer,
	)
	if err != nil {
		numaLogger.Fatal(err, "Unable to create GitSync controller")
	}

	if err = gitSyncReconciler.SetupWithManager(mgr); err != nil {
		numaLogger.Fatal(err, "Unable to set up GitSync controller")
	}

	pipelineRolloutReconciler := controller.NewPipelineRolloutReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetConfig(),
	)

	if err = pipelineRolloutReconciler.SetupWithManager(mgr); err != nil {
		numaLogger.Fatal(err, "Unable to set up PipelineRollout controller")
	}

	numaflowControllerRolloutReconciler, err := controller.NewNumaflowControllerRolloutReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetConfig(),
		metricServer,
		kubectl,
	)
	if err != nil {
		numaLogger.Fatal(err, "Unable to create NumaflowControllerRollout controller")
	}

	if err = numaflowControllerRolloutReconciler.SetupWithManager(mgr); err != nil {
		numaLogger.Fatal(err, "Unable to set up NumaflowControllerRollout controller")
	}

	isbServiceRolloutReconciler := controller.NewISBServiceRolloutReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetConfig(),
	)

	if err = isbServiceRolloutReconciler.SetupWithManager(mgr); err != nil {
		numaLogger.Fatal(err, "Unable to set up ISBServiceRollout controller")
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		numaLogger.Fatal(err, "Unable to set up health check")
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		numaLogger.Fatal(err, "Unable to set up ready check")
	}

	numaLogger.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		numaLogger.Fatal(err, "Unable to start manager")
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
