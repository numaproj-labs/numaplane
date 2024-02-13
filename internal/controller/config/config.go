package config

import (
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

func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		config: &GlobalConfig{},
		lock:   new(sync.RWMutex),
	}
}

// GlobalConfig is the configuration for the controllers, it is
// supposed to be populated from the configmap attached to the
// controller manager.
type GlobalConfig struct {
	ClusterName     string                    `json:"clusterName"`
	TimeIntervalSec uint                      `json:"timeIntervalSec"`
	RepoCredentials map[string]*GitCredential `json:"repoCredentials"`
}

type GitCredential struct {
	HTTPCredential *HTTPCredential `json:"HTTPCredential"`
	SSHCredential  *SSHCredential  `json:"SSHCredential"`
	TLS            *TLS            `json:"TLS"`
}

type HTTPCredential struct {
	Username string            `json:"username"`
	Password SecretKeySelector `json:"password"`
}

type SSHCredential struct {
	SSHKey SecretKeySelector `json:"SSHKey" yaml:"SSHKey" `
}

type TLS struct {
	InsecureSkipVerify bool              `json:"insecureSkipVerify"`
	CACertSecret       SecretKeySelector `json:"CACertSecret"`
	CertSecret         SecretKeySelector `json:"CertSecret"`
	KeySecret          SecretKeySelector `json:"keySecret"`
}

type SecretKeySelector struct {
	corev1.LocalObjectReference `mapstructure:",squash"` // for viper to correctly parse the config
	Key                         string                   `json:"key" `
	Optional                    *bool                    `json:"optional,omitempty" `
}

func (cm *ConfigManager) GetConfig() *GlobalConfig {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	return cm.config
}

func (cm *ConfigManager) LoadConfig(onErrorReloading func(error), configPath string) error {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
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
		cm.lock.RLock()
		defer cm.lock.RUnlock()
		err = v.Unmarshal(cm.config)
		if err != nil {
			onErrorReloading(err)
		}
	})
	return nil
}
