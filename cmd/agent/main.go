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
	"flag"

	"github.com/numaproj-labs/numaplane/internal/shared/logging"
)

var (
	// logger is the global logger for the numaplane-agent
	logger = logging.NewLogger().Named("numaplane-agent")
)

func main() {
	var enableLeaderElection bool
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for numaplane agent. "+
			"Enabling this will ensure there is only one active controller manager.")

	// TODO: Add health check and readiness check

	// Create a ConfigManager

	// Create a Generator if one is defined

	// create a new AgentSyncer

	// start the AgentSyncer, which should cause it to loop repeatedly,
	// each time:
	// wait n seconds
	// if Generator is defined, call generator.GetClusterKVPairs() and evaluate Source
	// else get Source
	// clone/fetch repo
	// apply resource

}
