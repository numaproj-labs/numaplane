package config

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
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
	ClusterName     string `json:"clusterName"`
	TimeIntervalSec uint   `json:"timeIntervalSec"`
	// RepoCredentials maps each Git Repository Path prefix to the corresponding credentials that are needed for it
	RepoCredentials []RepoCredential `json:"repoCredentials"`
}

type RepoCredential struct {
	URL            string          `json:"url"`
	HTTPCredential *HTTPCredential `json:"httpCredential"`
	SSHCredential  *SSHCredential  `json:"sshCredential"`
	TLS            *TLS            `json:"tls"`
}

type HTTPCredential struct {
	Username string            `json:"username"`
	Password SecretKeySelector `json:"password"`
}

type SSHCredential struct {
	SSHKey SecretKeySelector `json:"SSHKey" yaml:"SSHKey" `
}

type TLS struct {
	InsecureSkipVerify bool `json:"insecureSkipVerify"`
}

type SecretKeySelector struct {
	corev1.LocalObjectReference `mapstructure:",squash"` // for viper to correctly parse the config
	Key                         string                   `json:"key" `
	NameSpace                   string                   `json:"namespace"` // namespace  where the secrets are stored
	Optional                    *bool                    `json:"optional,omitempty" `
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
	err = v.Unmarshal(cm.config)
	if err != nil {
		return fmt.Errorf("failed unmarshal configuration file. %w", err)
	}
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		cm.lock.Lock()
		defer cm.lock.Unlock()
		err = v.Unmarshal(cm.config)
		if err != nil {
			onErrorReloading(err)
		}
	})
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
