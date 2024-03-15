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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// +kubebuilder:validation:Enum=Helm;Kustomize;Raw
// ApplicationSourceType specifies the type of the application source
type SourceType string

const (
	SourceTypeHelm      SourceType = "Helm"
	SourceTypeKustomize SourceType = "Kustomize"
	SourceTypeRaw       SourceType = "Raw"
)

type GitSource struct {
	GitLocation `json:",inline"`

	// Kustomize holds kustomize specific options
	Kustomize *KustomizeSource `json:"kustomize,omitempty"`

	// Helm holds helm specific options
	Helm *HelmSource `json:"helm,omitempty"`

	// Raw holds path or directory-specific options
	Raw *RawSource `json:"raw,omitempty"`
}

type CredentialedGitSource struct {
	GitSource `json:",inline"`

	RepoCredential *RepoCredential `json:"repoCredential,omitempty"`
}

type GitLocation struct {
	// RepoUrl is the URL to the repository itself
	RepoUrl string `json:"repoUrl"`

	// Path is the full path from the root of the repository to where the resources are held
	//  If the Path is empty, then the root directory will be used.
	// Can be a file or a directory
	// Note that all resources within this path (described by .yaml files) will be synced
	Path string `json:"path"`

	// TargetRevision specifies the target revision to sync to, it can be a branch, a tag,
	// or a commit hash.
	TargetRevision string `json:"targetRevision"`
}

// KustomizeSource holds kustomize specific options
type KustomizeSource struct{}

// HelmSource holds helm-specific options
type HelmSource struct {
	// ValuesFiles is a list of Helm value files to use when generating a template
	ValueFiles []string `json:"valueFiles,omitempty"`
	// Parameters is a list of Helm parameters which are passed to the helm template command upon manifest generation
	Parameters []HelmParameter `json:"parameters,omitempty"`
}

// HelmParameter is a parameter passed to helm template during manifest generation
type HelmParameter struct {
	// Name is the name of the Helm parameter
	Name string `json:"name,omitempty"`
	// Value is the value for the Helm parameter
	Value string `json:"value,omitempty"`
}

// RawSource holds raw specific options
type RawSource struct{}

// ExplicitType returns the type (e.g., Helm, Kustomize, etc.) of the application. If either none or multiple types are defined, returns an error.
func (gitSource *GitSource) ExplicitType() (SourceType, error) {
	var appTypes []SourceType
	if gitSource.Kustomize != nil {
		appTypes = append(appTypes, SourceTypeKustomize)
	}
	if gitSource.Helm != nil {
		appTypes = append(appTypes, SourceTypeHelm)
	}
	if gitSource.Raw != nil {
		appTypes = append(appTypes, SourceTypeRaw)
	}
	if len(appTypes) == 0 {
		// Fallback to a raw source type if a user has not specified anything.
		return SourceTypeRaw, nil
	}
	if len(appTypes) > 1 {
		typeNames := make([]string, len(appTypes))
		for i := range appTypes {
			typeNames[i] = string(appTypes[i])
		}
		return "", fmt.Errorf("multiple sources defined: %s", strings.Join(typeNames, ","))
	}
	appType := appTypes[0]
	return appType, nil
}

type RepoCredential struct {
	URL            string          `json:"url"`
	HTTPCredential *HTTPCredential `json:"httpCredential"`
	SSHCredential  *SSHCredential  `json:"sshCredential"`
	TLS            *TLS            `json:"tls"`
}

type HTTPCredential struct {
	Username string            `json:"username"`
	Password SecretKeySelector `json:"password"`
}

type SSHCredential struct {
	SSHKey SecretKeySelector `json:"SSHKey" yaml:"SSHKey" `
}

type TLS struct {
	InsecureSkipVerify bool `json:"insecureSkipVerify"`
}

type SecretKeySelector struct {
	corev1.LocalObjectReference `mapstructure:",squash"` // for viper to correctly parse the config
	Key                         string                   `json:"key" `
	Optional                    *bool                    `json:"optional,omitempty" `
}

type SingleClusterGenerator map[string]string

// multiple files containing key/value pairs: subsequent entries can override earlier entries
type MultiClusterFileGenerator struct {
	Files []*MultiClusterFile `json:"files"`

	ClusterKey   string `json:"clusterKey"`
	ClusterValue string `json:"clusterValue"`
}

type MultiClusterFile struct {
	GitLocation    `json:",inline"`
	RepoCredential *RepoCredential `json:"repoCredential,omitempty"`
}