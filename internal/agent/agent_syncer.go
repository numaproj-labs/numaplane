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
	"context"
	"time"

	"go.uber.org/zap"

	apiv1 "github.com/numaproj-labs/numaplane/api/v1alpha1"
	kvsource "github.com/numaproj-labs/numaplane/internal/keyvaluegenerator"
	//kubeutil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	//"k8s.io/client-go/rest"
	//"sigs.k8s.io/controller-runtime/pkg/client"
)

type AgentSyncer struct {
	logger *zap.SugaredLogger

	configMap         AgentConfig
	configMapRevision int // setting this < 0 enables us to check it initially
	// todo: add all of these in
	/*
		client    client.Client
		config    *rest.Config
		rawConfig *rest.Config
		kubectl   kubeutil.Kubectl

		stateCache  LiveStateCache
	*/
}

func NewAgentSyncer(logger *zap.SugaredLogger) *AgentSyncer {
	return &AgentSyncer{
		logger:            logger,
		configMapRevision: -1,
	}
}

// Run function runs in a loop, syncing the manifest if it's changed, checking every x seconds
// It responds to all of the following events:
// 1. ConfigMap change;
// 2. Source Manifest change;
// 3. If file generator is used, then a change from that file
func (syncer *AgentSyncer) Run(ctx context.Context) error {

	for {
		select {
		default:

			// Determine the latest value of our GitSource definition
			gitSource := syncer.getGitSource()

			syncer.syncLatest(gitSource)

			time.Sleep(time.Duration(syncer.configMap.TimeIntervalSec) * time.Second)
		case <-ctx.Done():
			syncer.logger.Info("context ended, terminating AgentSyncer watch")
			return nil
		}
	}

}

// determine if ConfigMap was updated and if so, get latest
func (syncer *AgentSyncer) checkConfigMapUpdated() bool {
	configManager := GetConfigManagerInstance()
	var err error
	var newRevision int
	// Reload our copy of the ConfigMap if it changed (or load it the first time upon starting)
	if configManager.GetRevisionIndex() > syncer.configMapRevision {
		syncer.configMap, newRevision, err = configManager.GetConfig()
		if err != nil {
			syncer.logger.Error(err)
			return false
		}
		syncer.configMapRevision = newRevision
		return true
	}
	return false
}

func (syncer *AgentSyncer) getGitSource() apiv1.CredentialedGitSource {
	var err error
	// this is the source of our key/value pairs
	var kvSource kvsource.KVSource

	keysValuesModified := false // do we need to reevaluate the gitSource because the key/value pairs changed?

	if syncer.checkConfigMapUpdated() {
		// create a KVSource which will return a new set of key/value pairs
		kvSource = createKVSource(syncer.configMap.Source.KVGenerator)
		keysValuesModified = true

	}

	if kvSource == nil {
		return syncer.configMap.Source.GitDefinition
	} else {
		var keysValues map[string]string
		keysValues, keysValuesModified = kvSource.GetKeysValues()
		// if the key/value pairs changed, then reevaluate the gitSource
		if keysValuesModified {
			return evaluateGitDefinition(syncer.configMap.Source.GitDefinition, keysValues)
		} else {
			return syncer.configMap.Source.GitDefinition
		}
	}

}

// clone/fetch repo
// apply resource if it changed
func (syncer *AgentSyncer) syncLatest(gitSource apiv1.CredentialedGitSource) {

}
