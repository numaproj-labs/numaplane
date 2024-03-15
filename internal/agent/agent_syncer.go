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

	// the source where we watch manifests
	gitSource *apiv1.CredentialedGitSource

	// this is the source of our key/value pairs
	kvSource kvsource.KVSource

	// current Config file
	config AgentConfig

	// keep track of revision number of config file so we know when it's new
	configRevision int
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
		logger:         logger,
		configRevision: -1, // setting this < 0 enables us to check it initially
	}
}

// Run function runs in a loop, syncing the manifest if it's changed, checking every x seconds
// It responds to all of the following events:
// 1. Config change;
// 2. Source Manifest change;
// 3. If file generator is used, then a change from that file
func (syncer *AgentSyncer) Run(ctx context.Context) {

	for {
		select {
		default:

			// Determine the latest value of our GitSource definition
			syncer.evaluateGitSource()

			// fetch the GitSource and apply the resources
			syncer.syncLatest()

			time.Sleep(time.Duration(syncer.config.TimeIntervalSec) * time.Second)
		case <-ctx.Done():
			syncer.logger.Info("context ended, terminating AgentSyncer watch")
			return
		}
	}

}

// determine if Config was updated and if so, get latest
// return if new
func (syncer *AgentSyncer) checkConfigUpdate() bool {
	configManager := GetConfigManagerInstance()
	var err error
	var newRevision int
	// Reload our copy of the Config if it changed (or load it the first time upon starting)
	if configManager.GetRevisionIndex() > syncer.configRevision {
		syncer.config, newRevision, err = configManager.GetConfig()
		if err != nil {
			syncer.logger.Error(err)
			return false
		}
		syncer.configRevision = newRevision
		return true
	}
	return false
}

// Determine the latest value of our GitSource definition
func (syncer *AgentSyncer) evaluateGitSource() {

	var keysValues map[string]string

	keysValuesModified := false // do we need to reevaluate the gitSource because the key/value pairs changed?

	// was Config updated?
	if syncer.checkConfigUpdate() {
		// create a KVSource which will return a new set of key/value pairs
		syncer.kvSource = createKVSource(syncer.config.Source.KeyValueGenerator)
		keysValuesModified = true
		keysValues, _ = syncer.kvSource.GetKeysValues()
		syncer.logger.Infof("Config update: syncer.kvSource=%+v, keysValues=%+v", syncer.kvSource, keysValues)

	} else {
		if syncer.kvSource == nil {
			// no KVSource defined, so just use the GitDefinition as is
			syncer.gitSource = &syncer.config.Source.GitDefinition
			return
		} else {
			keysValues, keysValuesModified = syncer.kvSource.GetKeysValues()
		}
	}

	// if the key/value pairs changed, then reevaluate the gitSource (which is presumably templated)
	if keysValuesModified {
		gitSource, err := evaluateGitDefinition(&syncer.config.Source.GitDefinition, keysValues)
		if err != nil {
			syncer.logger.Error(err)
			syncer.gitSource = &syncer.config.Source.GitDefinition
			return
		} else {
			syncer.gitSource = gitSource
			syncer.logger.Infof("keysValues modified: %+v; new gitSource value: %v", keysValues, syncer.gitSource)
			return
		}
	}

}

// clone/fetch repo
// apply resource if it changed
func (syncer *AgentSyncer) syncLatest() {
	// fetch using the syncer.gitSource
}
