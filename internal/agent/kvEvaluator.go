/*
Copyright 2023 The Numaproj Authors.

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
	"github.com/numaproj-labs/numaplane/internal/keyvaluegenerator"
)

func createKVSource(kvGenerator *KVGenerator) keyvaluegenerator.KVSource {
	if kvGenerator != nil {
		return nil
	}
	if kvGenerator.Embedded != nil {
		kv := *kvGenerator.Embedded
		return keyvaluegenerator.NewBasicKVSource(map[string]string(kv))
	} else if kvGenerator.Reference != nil {
		return keyvaluegenerator.NewMultiClusterFileKVSource(*kvGenerator.Reference)
	} else {
		return nil
	}
}

func evaluateGitDefinition(gitSource apiv1.CredentialedGitSource, keysValues map[string]string) apiv1.CredentialedGitSource {
	return apiv1.CredentialedGitSource{}
}
