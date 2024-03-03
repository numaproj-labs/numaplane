package shared

const (
	// AnnotationKeyGitSyncInstance Resource metadata annotations (keys and values) used for tracking
	AnnotationKeyGitSyncInstance = "numaplane.numaproj.io/tracking-id"
	// SSAManager is the default numaplane manager name used by server-side apply syncs
	SSAManager = "numaplane-controller"
	// EnvLogFormat log format that is defined by `--logformat` option
	EnvLogFormat = "ARGOCD_LOG_FORMAT"
	// EnvLogLevel log level that is defined by `--loglevel` option
	EnvLogLevel = "ARGOCD_LOG_LEVEL"
)
