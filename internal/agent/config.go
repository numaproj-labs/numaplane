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

package agent

import (
	apiv1 "github.com/numaproj-labs/numaplane/api/v1alpha1"
)

type AgentConfig struct {
	ClusterName     string `json:"clusterName"`
	TimeIntervalSec uint   `json:"timeIntervalSec"`

	Source Source `json:"source"`
}

type Source struct {
	// optional, apply to GitDefinition
	KVGenerator *KVGenerator `json:"kvGenerator,omitempty"`

	GitDefinition CredentialedGitSource `json:"gitDefinition"`
}

type CredentialedGitSource struct {
	apiv1.GitSource `json:",inline"`

	RepoCredential *apiv1.RepoCredential `json:"repoCredential,omitempty"`
}

type KVGenerator struct {
	Direct *apiv1.SingleClusterGenerator `json:"direct,omitempty"`

	Reference *apiv1.MultiClusterFileGenerator `json:"reference,omitempty"`
}
