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
	ClusterName     string                    `yaml:"clusterName" json:"clusterName"`
	TimeIntervalSec uint                      `yaml:"timeIntervalSec" json:"timeIntervalSec"`
	RepoCredentials map[string]*GitCredential `json:"repoCredentials" yaml:"repoCredentials"`
}

type GitCredential struct {
	HTTPCredential *HTTPCredential
	SSHCredential  *SSHCredential
	TLS            *TLS
}

type HTTPCredential struct {
	Username string
	Password *corev1.SecretKeySelector
}
type SSHCredential struct {
	SSHKey *corev1.SecretKeySelector
}

type TLS struct {
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty" protobuf:"bytes,1,opt,name=insecureSkipVerify"`
	// CACertSecret refers to the secret that contains the CA cert
	// +optional
	CACertSecret *corev1.SecretKeySelector `json:"caCertSecret,omitempty" protobuf:"bytes,2,opt,name=caCertSecret"`
	// CertSecret refers to the secret that contains the cert
	// +optional
	CertSecret *corev1.SecretKeySelector `json:"clientCertSecret,omitempty" protobuf:"bytes,3,opt,name=certSecret"`
	// KeySecret refers to the secret that contains the key
	// +optional
	KeySecret *corev1.SecretKeySelector `json:"clientKeySecret,omitempty" protobuf:"bytes,4,opt,name=keySecret"`
}

func LoadConfig(onErrorReloading func(error)) (*GlobalConfig, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/numaplane")
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
