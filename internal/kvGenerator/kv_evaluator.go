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

package kvevaluator

// Actually, now that I think about it, does it make sense to have these separate
// structs?
// We want to be able to switch from one to another at any time if the ConfigMap
// changes.
// So, maybe we really just want one struct which can derive in a different way at any time.
// Or maybe we have these 3 but we need logic outside of this which will delete one and create a different one
// if the ConfigMap has changed
// If you're going to do that, you might as well create a new one any time the ConfigMap changes

/*
type KVEvaluator interface {
	update() error
	getKeysValues() map[string]string
}

func NewSingleClusterEvaluator()

type SingleClusterEvaluator struct {
	values map[string]string
}

type MultiClusterEvaluator struct {
	values map[string]string
}

type MultiClusterFileEvaluator struct {
}
*/
