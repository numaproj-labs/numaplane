package metrics

import (
	"os"
	"time"

	gitopsSyncCommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
)

type MetricsServer struct {
	syncCounter                    *prometheus.CounterVec
	workQueueSyncWaitTimeHistogram *prometheus.HistogramVec
	gitSyncUpdateErrorCounter      *prometheus.CounterVec
	reconcileHistogram             *prometheus.HistogramVec
	kubeRequestCounter             *prometheus.CounterVec
	kubectlExecCounter             *prometheus.CounterVec
	gitRequestCounter              *prometheus.CounterVec
	gitRequestFailedCounter        *prometheus.CounterVec
	gitRequestLatency              *prometheus.HistogramVec
	monitoredKubeAPI               *prometheus.GaugeVec
	kubeCachedResource             *prometheus.GaugeVec
	kubeCacheFailed                *prometheus.CounterVec
	hostname                       string
}

var (
	descAppDefaultLabels = []string{"name", "namespace"}
	syncCounter          = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "numaplane_gitsync_sync_total",
			Help: "Number of GitSync syncs.",
		},
		append(descAppDefaultLabels, "dest_namespace", "dest_cluster", "phase"),
	)

	workQueueSyncWaiTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "numaplane_workqueue_sync_wait_time_millisecond",
			Help:    "Work queue sync wait time for reconcile",
			Buckets: []float64{1, 2, 3, 4, 5},
		},
		[]string{},
	)

	gitSyncUpdateErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "numaplane_gitsync_update_error",
			Help: "Error on updating GitSync status.",
		},
		descAppDefaultLabels,
	)

	reconcileHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "numaplane_gitsync_reconcile_duration_second",
			Help: "GitSync reconciliation performance.",
		},
		[]string{"namespace", "dest_cluster"},
	)

	kubeRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "numaplane_gitsync_kube_request_total",
			Help: "Number of kubernetes requests executed during GitSync reconciliation.",
		},
		[]string{"response_code", "verb", "resource_kind", "resource_namespace"},
	)

	kubectlExecCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "numaplane_kubectl_exec_total",
		Help: "Number of kubectl executions",
	}, []string{"hostname", "command"})

	gitRequestCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "numaplane_git_request_total",
		Help: "Number of git request",
	}, []string{})

	gitRequestFailedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "numaplane_git_request_failed_total",
		Help: "Number of git request failed",
	}, []string{})

	gitRequestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "numaplane_git_request_latency",
			Help: "Git request latency",
		},
		[]string{},
	)

	monitoredKubeAPI = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "numaplane_monitored_kube_api_total",
		Help: "Number of monitored kubernetes API resources",
	}, []string{})

	kubeCachedResource = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "numaplane_kube_cache_resource_total",
		Help: "Number of kubernetes resource objects in the cache",
	}, []string{})

	kubeCacheFailed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "numaplane_kube_cache_error",
		Help: "Error on fetching cluster cache",
	}, []string{})
)

func NewMetricsServer() (*MetricsServer, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(syncCounter)
	metrics.Registry.MustRegister(workQueueSyncWaiTime)
	metrics.Registry.MustRegister(gitSyncUpdateErrorCounter)
	metrics.Registry.MustRegister(reconcileHistogram)
	metrics.Registry.MustRegister(kubeRequestCounter)
	metrics.Registry.MustRegister(kubectlExecCounter)
	metrics.Registry.MustRegister(gitRequestCounter)
	metrics.Registry.MustRegister(gitRequestFailedCounter)
	metrics.Registry.MustRegister(gitRequestLatency)
	metrics.Registry.MustRegister(monitoredKubeAPI)
	metrics.Registry.MustRegister(kubeCachedResource)
	metrics.Registry.MustRegister(kubeCacheFailed)

	return &MetricsServer{
		syncCounter:                    syncCounter,
		workQueueSyncWaitTimeHistogram: workQueueSyncWaiTime,
		gitSyncUpdateErrorCounter:      gitSyncUpdateErrorCounter,
		reconcileHistogram:             reconcileHistogram,
		kubeRequestCounter:             kubeRequestCounter,
		kubectlExecCounter:             kubectlExecCounter,
		gitRequestCounter:              gitRequestCounter,
		gitRequestFailedCounter:        gitRequestFailedCounter,
		gitRequestLatency:              gitRequestLatency,
		monitoredKubeAPI:               monitoredKubeAPI,
		kubeCachedResource:             kubeCachedResource,
		kubeCacheFailed:                kubeCacheFailed,
		hostname:                       hostname,
	}, nil
}

// IncSync increments the sync counter for an application
func (m *MetricsServer) IncSync(gitSync *v1alpha1.GitSync, state gitopsSyncCommon.OperationPhase) {
	if !state.Completed() {
		return
	}
	m.syncCounter.WithLabelValues(gitSync.Name, gitSync.Namespace, gitSync.Spec.Destination.Namespace, gitSync.Spec.Destination.Cluster, string(state)).Inc()
}

// ObserveWorkerQueueSyncWaitTime increments the sync counter for an application
func (m *MetricsServer) ObserveWorkerQueueSyncWaitTime(duration time.Duration) {
	m.workQueueSyncWaitTimeHistogram.WithLabelValues().Observe(float64(duration.Milliseconds()))
}

// InGitSyncUpdateError increments the sync counter for git sync status update failure
func (m *MetricsServer) InGitSyncUpdateError(gitSync *v1alpha1.GitSync) {
	m.gitSyncUpdateErrorCounter.WithLabelValues(gitSync.Name, gitSync.Namespace).Inc()
}

// ObserveReconcile increments the reconciled counter for an application
func (m *MetricsServer) ObserveReconcile(gitSync *v1alpha1.GitSync, duration time.Duration) {
	m.reconcileHistogram.WithLabelValues(gitSync.Namespace, gitSync.Spec.Destination.Cluster).Observe(duration.Seconds())
}

// IncKubernetesRequest increments the kubernetes requests counter for an application
func (m *MetricsServer) IncKubernetesRequest(statusCode, verb, resourceKind, resourceNamespace string) {
	m.kubeRequestCounter.WithLabelValues(statusCode, verb, resourceKind, resourceNamespace).Inc()
}

func (m *MetricsServer) IncKubectlExec(command string) {
	m.kubectlExecCounter.WithLabelValues(m.hostname, command).Inc()
}

func (m *MetricsServer) IncGitRequest() {
	m.gitRequestCounter.WithLabelValues().Inc()
}

func (m *MetricsServer) IncGitRequestFailed() {
	m.gitRequestFailedCounter.WithLabelValues().Inc()
}

func (m *MetricsServer) ObserveGitRequestLatency(duration time.Duration) {
	m.gitRequestLatency.WithLabelValues().Observe(duration.Seconds())
}

func (m *MetricsServer) SetMonitoredKubeAPI(apiRequest float64) {
	m.monitoredKubeAPI.WithLabelValues().Set(apiRequest)
}

func (m *MetricsServer) SetKubeCachedResource(cache float64) {
	m.kubeCachedResource.WithLabelValues().Set(cache)
}

func (m *MetricsServer) IncKubeCacheFailed() {
	m.kubeCacheFailed.WithLabelValues().Inc()
}
