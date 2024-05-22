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

// Important: Run "make" to regenerate code after modifying this file

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitSyncSpec defines the desired state of GitSync
type GitSyncSpec struct {
	GitSource `json:",inline"`

	// Destination describes which cluster/namespace to sync it
	Destination Destination `json:"destination"`
}

// GitSyncStatus defines the observed state of GitSync
type GitSyncStatus struct {
	Status `json:",inline"`

	// Last commit processed and the status
	CommitStatus *CommitStatus `json:"commitStatus,omitempty"`
}

// Destination indicates a Cluster to sync to
type Destination struct {
	Cluster string `json:"cluster"`

	// Namespace is optional, as the Resources may be on the cluster level
	// (Note that some Resources describe their namespace within their spec: for those that don't it's useful to have it here)
	Namespace string `json:"namespace,omitempty"`
}

// CommitStatus maintains the status of syncing an individual Git commit
type CommitStatus struct {
	// Hash of the git commit
	Hash string `json:"hash"`

	// Synced indicates if the sync went through
	Synced bool `json:"synced"`

	// SyncTime represents the last time that we attempted to sync this commit (regardless of whether it succeeded)
	SyncTime metav1.Time `json:"syncTime"`

	// Error indicates an error that occurred upon attempting sync, if any
	Error string `json:"error,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="The current phase"

// GitSync is the Schema for the gitsyncs API
type GitSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitSyncSpec   `json:"spec"`
	Status GitSyncStatus `json:"status,omitempty"`
}

// String returns the general purpose string representation
func (gitSync GitSync) String() string {
	return gitSync.Namespace + "/" + gitSync.Name
}

//+kubebuilder:object:root=true

// GitSyncList contains a list of GitSync
type GitSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitSync{}, &GitSyncList{})
}

// ContainsClusterDestination determines if the cluster matches the Destination
func (gitSyncSpec *GitSyncSpec) ContainsClusterDestination(cluster string) bool {
	return gitSyncSpec.Destination.Cluster == cluster
}

// GetDestinationNamespace gets the namespace with the given cluster,
// if not found, then return empty.
func (gitSyncSpec *GitSyncSpec) GetDestinationNamespace(cluster string) string {
	if gitSyncSpec.Destination.Cluster == cluster {
		return gitSyncSpec.Destination.Namespace
	}
	return ""
}

// ExplicitType returns the type (e.g., Helm, Kustomize, etc.) of the application. If either none or multiple types are defined, returns an error.
func (gitSyncSpec *GitSyncSpec) ExplicitType() (SourceType, error) {
	return gitSyncSpec.GitSource.ExplicitType()
}
