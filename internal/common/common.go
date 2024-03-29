package common

const (
	// AnnotationKeyGitSyncInstance Resource metadata annotations (keys and values) used for tracking
	AnnotationKeyGitSyncInstance = "numaplane.numaproj.io/tracking-id"
	// SSAManager is the default numaplane manager name used by server-side apply syncs
	SSAManager = "numaplane-controller"
	// EnvLogLevel log level that is defined by `--loglevel` option
	EnvLogLevel = "NUMAPLANE_LOG_LEVEL"
)
