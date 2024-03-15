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

package keyvaluegenerator

import (
	"maps"

	apiv1 "github.com/numaproj-labs/numaplane/api/v1alpha1"
)

// the purpose of this interface is to derive keys/values which can be applied to a spec
type KVSource interface {
	// return the keys/values as well as whether there was a modification to them as part
	// of this call
	GetKeysValues() (map[string]string, bool)
}

func NewBasicKVSource(kv map[string]string) *BasicKVSource {
	return &BasicKVSource{values: maps.Clone(kv)}
}

type BasicKVSource struct {
	values map[string]string
}

func (source *BasicKVSource) GetKeysValues() (map[string]string, bool) {
	return source.values, false
}

func NewMultiClusterFileKVSource(sourceDefinition *apiv1.MultiClusterFileGenerator) *MultiClusterFileKVSource {
	return &MultiClusterFileKVSource{sourceDefinition: *sourceDefinition.DeepCopy()}
}

type MultiClusterFileKVSource struct {
	sourceDefinition apiv1.MultiClusterFileGenerator
	values           map[string]string
}

func (source *MultiClusterFileKVSource) GetKeysValues() (map[string]string, bool) {
	// TODO:
	// starting from last file in the list:
	//   clone/fetch repo with credentials to get latest file
	//   return keys/values for our cluster if present; also return whether the file is new
	//   and key/value pairs changed

	modified := false
	return source.values, modified
}
