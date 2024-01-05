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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GitSyncSpec defines the desired state of GitSync
type GitSyncSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// RepositoryPaths lists one or more Git Repository paths to watch
	RepositoryPaths []RepositoryPath `json:"repositoryPaths"`

	// Destinations describe where to sync it
	Destinations []Destination `json:"destinations"`
}

// GitSyncStatus defines the observed state of GitSync
type GitSyncStatus struct {
	// Important: Run "make" to regenerate code after modifying this file

	// Recent commits that have been processed and their status, mapped by <RepoUrl>/<Path>
	CommitStatus map[string]CommitStatus `json:"commitStatus,omitempty"`
}

type RepositoryPath struct {
	// Name is a unique name
	Name string `json:"name"`

	// RepoUrl is the URL to the repository itself
	RepoUrl string `json:"repoUrl"`

	// Path is the full path from the root of the repository to where the resources are held
	// Can be a file or a directory
	// Note that all resources within this path (described by .yaml files) will be synced
	Path string `json:"path"`
}

type Destination struct {
	Cluster string `json:"cluster"`

	// Namespace is optional, as the Resources may be on the cluster level
	// (Note that some Resources describe their namespace within their spec: for those that don't it's useful to have it here)
	Namespace string `json:"namespace,omitempty"`
}

type CommitStatus struct {
	Hash string `json:"hash"`

	Synced bool `json:"synced,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GitSync is the Schema for the gitsyncs API
type GitSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitSyncSpec   `json:"spec"`
	Status GitSyncStatus `json:"status,omitempty"`
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
