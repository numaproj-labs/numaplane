package config

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
)

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
	Username string                   `json:"username"`
	Password corev1.SecretKeySelector `mapstructure:",squash"`
}

type SSHCredential struct {
	SSHKey *corev1.SecretKeySelector `json:"SSHKey" yaml:"SSHKey" `
}

type TLS struct {
	InsecureSkipVerify bool                      `json:"insecureSkipVerify"`
	CACertSecret       *corev1.SecretKeySelector `json:"CACertSecret"`
	CertSecret         *corev1.SecretKeySelector `json:"CertSecret"`
	KeySecret          *corev1.SecretKeySelector `json:"keySecret"`
}

func LoadConfig(onErrorReloading func(error), configPath string) (*GlobalConfig, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)
	err := v.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration file. %w", err)
	}
	r := &GlobalConfig{}
	err = v.Unmarshal(r)
	if err != nil {
		return nil, fmt.Errorf("failed unmarshal configuration file. %w", err)
	}
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		err = v.Unmarshal(r)
		if err != nil {
			onErrorReloading(err)
		}
	})
	return r, nil
}
