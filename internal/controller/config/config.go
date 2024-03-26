package config

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
	apiv1 "github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
	"github.com/spf13/viper"
)

type ConfigManager struct {
	config *GlobalConfig
	lock   *sync.RWMutex
}

var instance *ConfigManager
var once sync.Once

// GetConfigManagerInstance  returns a singleton config manager throughout the application
func GetConfigManagerInstance() *ConfigManager {
	once.Do(func() {
		instance = &ConfigManager{
			config: &GlobalConfig{},
			lock:   new(sync.RWMutex),
		}
	})
	return instance
}

// GlobalConfig is the configuration for the controllers, it is
// supposed to be populated from the configmap attached to the
// controller manager.
type GlobalConfig struct {
	ClusterName            string `json:"clusterName" mapstructure:"clusterName"`
	SyncTimeIntervalMs     int    `json:"syncTimeIntervalMs" mapstructure:"syncTimeIntervalMs"`
	AutoHealTimeIntervalMs int    `json:"autoHealTimeIntervalMs" mapstructure:"autoHealTimeIntervalMs"`
	// RepoCredentials maps each Git Repository Path prefix to the corresponding credentials that are needed for it
	RepoCredentials []apiv1.RepoCredential `json:"repoCredentials" mapstructure:"repoCredentials"`
}

func (cm *ConfigManager) GetConfig() (GlobalConfig, error) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	config, err := CloneWithSerialization(cm.config)
	if err != nil {
		return GlobalConfig{}, err
	}
	return *config, nil
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
	{
		cm.lock.Lock()
		defer cm.lock.Unlock()
		err = v.Unmarshal(cm.config)
		if err != nil {
			return fmt.Errorf("failed unmarshal configuration file. %w", err)
		}
	}
	v.OnConfigChange(func(e fsnotify.Event) {
		cm.lock.Lock()
		defer cm.lock.Unlock()
		newConfig := GlobalConfig{}
		err = v.Unmarshal(&newConfig)
		if err != nil {
			onErrorReloading(err)
		}
		cm.config = &newConfig
	})
	v.WatchConfig()
	return nil
}

func CloneWithSerialization(orig *GlobalConfig) (*GlobalConfig, error) {
	origJSON, err := json.Marshal(orig)
	if err != nil {
		return nil, err
	}
	clone := GlobalConfig{}
	if err = json.Unmarshal(origJSON, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}
