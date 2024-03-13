package kubernetes

import (
	"os"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/tracing"

	log "github.com/numaproj-labs/numaplane/internal/util/logging"
)

var tracer tracing.Tracer = &tracing.NopTracer{}

func init() {
	if os.Getenv("NUMAPLANE_TRACING_ENABLED") == "1" {
		tracer = tracing.NewLoggingTracer(log.NewLogrusLogger(log.NewWithCurrentConfig()))
	}
}

func NewKubectl() kube.Kubectl {
	return &kube.KubectlCmd{Tracer: tracer, Log: log.NewLogrusLogger(log.NewWithCurrentConfig())}
}
