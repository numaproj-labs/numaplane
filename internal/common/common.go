package common

import "k8s.io/apimachinery/pkg/runtime/schema"

const (
	// AnnotationKeyGitSyncInstance Resource metadata annotations (keys and values) used for tracking
	AnnotationKeyGitSyncInstance = "numaplane.numaproj.io/tracking-id"
	// SSAManager is the default numaplane manager name used by server-side apply syncs
	SSAManager = "numaplane-controller"
	// EnvLogFormat log format that is defined by `--logformat` option
	EnvLogFormat = "NUMAPLANE_LOG_FORMAT"
	// EnvLogLevel log level that is defined by `--loglevel` option
	EnvLogLevel = "NUMAPLANE_LOG_LEVEL"
)

// PredefinedGroupVersionKinds contains the list of GVK which we need to delete in the cluster post gitsync gets deleted
var PredefinedGroupVersionKinds = []schema.GroupVersionKind{
	{Group: "", Version: "v1", Kind: "Pod"},
	{Group: "apps", Version: "v1", Kind: "Deployment"},
	{Group: "apps", Version: "v1", Kind: "ReplicaSet"},
	{Group: "apps", Version: "v1", Kind: "StatefulSet"},
	{Group: "apps", Version: "numaflow.numaproj.io/v1alpha1", Kind: "Pipeline"},
	{Group: "apps", Version: "numaflow.numaproj.io/v1alpha1", Kind: "InterStepBufferService"},
}
