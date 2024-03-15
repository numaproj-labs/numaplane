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

package main

import (
	"context"

	"github.com/numaproj-labs/numaplane/internal/agent"
	"github.com/numaproj-labs/numaplane/internal/util/logging"
)

var (
	// logger is the global logger for the numaplane-agent
	logger     = logging.NewLogger().Named("numaplane-agent")
	configPath = "/etc/agent" // Path in the volume mounted in the pod where yaml is present
)

func main() {
	var err error

	ctx := context.Background()

	// TODO: Add health check and readiness check

	// Create a ConfigManager to manage our config file, watching for changes
	configManager := agent.GetConfigManagerInstance()
	err = configManager.LoadConfig(func(err error) {
		logger.Errorw("Failed to reload configuration file", err)
	}, configPath, "config", "yaml")
	if err != nil {
		logger.Fatalw("Failed to load configuration file", err)
	}

	config, _, err := configManager.GetConfig()
	if err != nil {
		logger.Fatalw("Failed to get configuration file", err)
	}
	logger.Infof("config: %+v", config)
	//todo: delete:
	logger.Infof("embedded: %+v", config.Source.KeyValueGenerator.Embedded)
	//logger.Infof("helm: %+v", *config.Source.GitDefinition.Helm)

	// create a new AgentSyncer
	syncer := agent.NewAgentSyncer(logger)
	syncer.Run(ctx)

}
