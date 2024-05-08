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
	"github.com/numaproj-labs/numaplane/internal/util/logger"
)

var (
	// logger is the global logger for the numaplane-agent
	numaLogger = logger.New().WithName("numaplane-agent")
	configPath = "/etc/agent" // Path in the volume mounted in the pod where yaml is present
)

func main() {
	var err error

	ctx := context.Background()

	// TODO: Add health check and readiness check

	// Create a ConfigManager to manage our config file, watching for changes
	configManager := agent.GetConfigManagerInstance()
	err = configManager.LoadConfig(func(err error) {
		numaLogger.Error(err, "Failed to reload configuration file")
	}, configPath, "config", "yaml")
	if err != nil {
		numaLogger.Fatal(err, "Failed to load configuration file")
	}

	// Get initial Config so we can log it
	config, _, err := configManager.GetConfig()
	if err != nil {
		numaLogger.Fatal(err, "Failed to get configuration file")
	}

	numaLogger.SetLevel(config.LogLevel)
	logger.SetBaseLogger(numaLogger)

	numaLogger.Infof("config: %+v", config)

	// create a new AgentSyncer
	syncer := agent.NewAgentSyncer(numaLogger)
	syncer.Run(ctx)
}
