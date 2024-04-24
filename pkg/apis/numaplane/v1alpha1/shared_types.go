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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
)

// +kubebuilder:validation:Enum=Helm;Kustomize;Raw
// SourceType specifies the type of the application source
type SourceType string

const (
	SourceTypeHelm      SourceType = "Helm"
	SourceTypeKustomize SourceType = "Kustomize"
	SourceTypeRaw       SourceType = "Raw"
)

type GitSource struct {
	GitLocation `json:",inline" mapstructure:",squash"`

	// Kustomize holds kustomize specific options
	Kustomize *KustomizeSource `json:"kustomize,omitempty" mapstructure:"kustomize,omitempty"`

	// Helm holds helm specific options
	Helm *HelmSource `json:"helm,omitempty" mapstructure:"helm,omitempty"`

	// Raw holds path or directory-specific options
	Raw *RawSource `json:"raw,omitempty" mapstructure:"raw,omitempty"`
}

type CredentialedGitSource struct {
	GitSource `json:",inline" mapstructure:",squash"`

	RepoCredential *RepoCredential `json:"repoCredential,omitempty" mapstructure:"repoCredential,omitempty"`
}

// CredentialedGitLocation points to a location in git that may or may not require credentials
type CredentialedGitLocation struct {
	GitLocation    `json:",inline" mapstructure:",squash"`
	RepoCredential *RepoCredential `json:"repoCredential,omitempty" mapstructure:"repoCredential,omitempty"`
}

type GitLocation struct {
	// RepoUrl is the URL to the repository itself
	RepoUrl string `json:"repoUrl" mapstructure:"repoUrl"`

	// Path is the full path from the root of the repository to where the resources are held
	//  If the Path is empty, then the root directory will be used.
	// Can be a file or a directory
	// Note that all resources within this path (described by .yaml files) will be synced
	Path string `json:"path" mapstructure:"path"`

	// TargetRevision specifies the target revision to sync to, it can be a branch, a tag,
	// or a commit hash.
	TargetRevision string `json:"targetRevision" mapstructure:"targetRevision"`
}

// KustomizeSource holds kustomize specific options
type KustomizeSource struct{}

// HelmSource holds helm-specific options
type HelmSource struct {
	// ValuesFiles is a list of Helm value files to use when generating a template
	ValueFiles []string `json:"valueFiles,omitempty" mapstructure:"valueFiles,omitempty"`
	// Parameters is a list of Helm parameters which are passed to the helm template command upon manifest generation
	Parameters []HelmParameter `json:"parameters,omitempty" mapstructure:"parameters,omitempty"`
}

// HelmParameter is a parameter passed to helm template during manifest generation
type HelmParameter struct {
	// Name is the name of the Helm parameter
	Name string `json:"name,omitempty" mapstructure:"name,omitempty"`
	// Value is the value for the Helm parameter
	Value string `json:"value,omitempty" mapstructure:"value,omitempty"`
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
	URL            string          `json:"url" mapstructure:"url"`
	HTTPCredential *HTTPCredential `json:"httpCredential,omitempty" mapstructure:"httpCredential,omitempty"`
	SSHCredential  *SSHCredential  `json:"sshCredential,omitempty" mapstructure:"sshCredential,omitempty"`
	TLS            *TLS            `json:"tls,omitempty" mapstructure:"tls,omitempty"`
}

type HTTPCredential struct {
	Username string        `json:"username" mapstructure:"username"`
	Password *SecretSource `json:"password" mapstructure:"password"`
}

type SSHCredential struct {
	SSHKey SecretSource `json:"SSHKey" mapstructure:"SSHKey"`
}

type TLS struct {
	InsecureSkipVerify bool `json:"insecureSkipVerify" mapstructure:"insecureSkipVerify"`
}

type SecretSource struct {
	FromKubernetesSecret *SecretKeySelector `json:"fromKubernetesSecret" mapstructure:"fromKubernetesSecret"`
	FromFile             *FileKeySelector   `json:"fromFile" mapstructure:"fromFile"`
}

type SecretKeySelector struct {
	corev1.ObjectReference `json:",inline" mapstructure:",squash"` // for viper to correctly parse the config
	Key                    string                                  `json:"key" mapstructure:"key"`
}

// FileKeySelector reads a key from a JSON or YAML file formatted as a map
type FileKeySelector struct {
	JSONFilePath *string `json:"jsonFilePath,omitempty" mapstructure:"jsonFilePath"` // should either by json or yaml
	YAMLFilePath *string `json:"yamlFilePath,omitempty" mapstructure:"yamlFilePath"`
	Key          string  `json:"key" mapstructure:"key"`
}

func (fks *FileKeySelector) GetSecretValue() (string, error) {
	if fks.JSONFilePath != nil && fks.YAMLFilePath != nil {
		return "", fmt.Errorf("JSONFilePath and YAMLFilePath can't both be set for FileKeySelector: %+v", *fks)
	}
	// read the file into a map
	var m map[string]interface{}
	if fks.JSONFilePath != nil {
		dat, err := os.ReadFile(*fks.JSONFilePath)
		if err != nil {
			return "", fmt.Errorf("Error reading JSONFilePath for FileKeySelector: %+v, err=%v", *fks, err)
		}
		err = json.Unmarshal(dat, &m)
		if err != nil {
			return "", fmt.Errorf("Failed to unmarshal JSONFilePath for FileKeySelector into map[string]interface{}: %+v, err=%v, file contents=%q", *fks, err, string(dat))
		}
	} else if fks.YAMLFilePath != nil {
		dat, err := os.ReadFile(*fks.YAMLFilePath)
		if err != nil {
			return "", fmt.Errorf("Error reading YAMLFilePath for FileKeySelector: %+v, err=%v", *fks, err)
		}
		err = yaml.Unmarshal(dat, &m)
		if err != nil {
			return "", fmt.Errorf("Failed to unmarshal YAMLFilePath for FileKeySelector into map[string]interface{}: %+v, err=%v, file contents=%q", *fks, err, string(dat))
		}
	} else {
		return "", fmt.Errorf("either JSONFilePath or YAMLFilePath should be set for FileKeySelector: %+v", *fks)
	}

	// now get the key from the map
	valueAsInterface, found := m[fks.Key]
	if !found {
		return "", fmt.Errorf("Failed to locate key: %+v, file contents as map=%q", *fks, m)
	}
	valueAsString, ok := valueAsInterface.(string)
	if !ok {
		return "", fmt.Errorf("key in FileSeySelector %+v can't be cast as string, file contents as map=%q", *fks, m)
	}
	return valueAsString, nil
}

// SingleClusterGenerator consists of a single set of key/value pairs that can be used for templating
type SingleClusterGenerator struct {
	Values map[string]string `json:"values" mapstructure:"values"`
}

// MultiClusterFileGenerator consists of multiple files containing key/value pairs: subsequent entries can override earlier entries
// each file will contain a map of cluster name to the key/value pairs that it uses for templating
type MultiClusterFileGenerator struct {
	Files []*CredentialedGitLocation `json:"files" mapstructure:"files"`
}
