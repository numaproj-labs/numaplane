package metrics

import (
	"strconv"

	"github.com/argoproj/pkg/kubeclientmetrics"
	"k8s.io/client-go/rest"
)

// AddMetricsTransportWrapper adds a transport wrapper which increments 'numaplane_app_k8s_request_total' counter on each kubernetes request
func AddMetricsTransportWrapper(server *MetricsServer, config *rest.Config) *rest.Config {
	inc := func(resourceInfo kubeclientmetrics.ResourceInfo) error {
		namespace := resourceInfo.Namespace
		kind := resourceInfo.Kind
		statusCode := strconv.Itoa(resourceInfo.StatusCode)
		server.IncKubernetesRequest(statusCode, string(resourceInfo.Verb), kind, namespace)
		return nil
	}

	newConfig := kubeclientmetrics.AddMetricsTransportWrapper(config, inc)
	return newConfig
}
