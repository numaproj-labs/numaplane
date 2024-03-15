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
	"encoding/json"
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
	apiv1 "github.com/numaproj-labs/numaplane/api/v1alpha1"
	"github.com/spf13/viper"
)

type AgentConfig struct {
	ClusterName     string `mapstructure:"clusterName"`
	TimeIntervalSec uint   `mapstructure:"timeIntervalSec"`

	Source Source `mapstructure:"source"`
}

type Source struct {
	// optional, apply to GitDefinition
	KeyValueGenerator *KVGenerator `mapstructure:"keyValueGenerator,omitempty"`

	GitDefinition apiv1.CredentialedGitSource `mapstructure:"gitDefinition"`
}

type KVGenerator struct {
	Embedded *apiv1.SingleClusterGenerator `mapstructure:"embedded,omitempty"`

	Reference *apiv1.MultiClusterFileGenerator `mapstructure:"reference,omitempty"`
}

type ConfigManager struct {
	config *AgentConfig
	lock   *sync.RWMutex
	// revisionIndex increments each time the Config changes so the caller can keep track of when
	// the file changed
	revisionIndex int
}

var instance *ConfigManager
var once sync.Once

// GetConfigManagerInstance returns a singleton config manager throughout the Agent application
func GetConfigManagerInstance() *ConfigManager {
	once.Do(func() {
		instance = &ConfigManager{
			config:        &AgentConfig{},
			lock:          new(sync.RWMutex),
			revisionIndex: 0,
		}
	})
	return instance
}

func (cm *ConfigManager) LoadConfig(onErrorReloading func(error), configPath, configFileName, configFileType string) error {
	v := viper.New()
	v.SetConfigName(configFileName)
	v.SetConfigType(configFileType)
	v.AddConfigPath(configPath)
	err := v.ReadInConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration file. %w", err)
	}

	err = v.Unmarshal(cm.config)
	if err != nil {
		return fmt.Errorf("failed unmarshal configuration file. %w", err)
	}
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("OnConfigChange")
		cm.lock.Lock()
		defer cm.lock.Unlock()
		cm.revisionIndex++
		newConfig := AgentConfig{}
		err = v.Unmarshal(&newConfig)
		if err != nil {
			onErrorReloading(err)
		}
		cm.config = &newConfig
	})
	return nil
}

func (cm *ConfigManager) GetRevisionIndex() int {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	return cm.revisionIndex
}

func (cm *ConfigManager) GetConfig() (AgentConfig, int, error) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	config, err := CloneWithSerialization(cm.config)
	if err != nil {
		return AgentConfig{}, cm.revisionIndex, err
	}
	return *config, cm.revisionIndex, nil
}

// CloneWithSerialization uses encoding and decoding with json as a mechanism for performing a DeepCopy
// of the AgentConfig
func CloneWithSerialization(orig *AgentConfig) (*AgentConfig, error) {
	origJSON, err := json.Marshal(orig)
	if err != nil {
		return nil, err
	}
	clone := AgentConfig{}
	if err = json.Unmarshal(origJSON, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}
