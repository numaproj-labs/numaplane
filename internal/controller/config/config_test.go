package config

import (
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	getwd, err := os.Getwd()
	assert.Nil(t, err, "Failed to get working directory")
	configPath := filepath.Join(getwd, "../../../", "config", "samples")
	config, err := LoadConfig(func(err error) {
	}, configPath)
	assert.Nil(t, err, "Failed to load configuration")

	log.Printf("Loaded Config: %+v", config)

	assert.Equal(t, "example-cluster", config.ClusterName, "ClusterName does not match")
	assert.Equal(t, uint(60), config.TimeIntervalSec, "TimeIntervalSec does not match")

	assert.NotNil(t, config.RepoCredentials, "RepoCredentials should not be nil")

	assert.NotNil(t, config.RepoCredentials["key1"].HTTPCredential, "HTTPCredential is missing")
	assert.Equal(t, "exampleUser", config.RepoCredentials["key1"].HTTPCredential.Username, "Username for HTTPCredential does not match")
	assert.Equal(t, "http-creds", config.RepoCredentials["key1"].HTTPCredential.Password.Name, "Password Name for HTTPCredential does not match")
	assert.Equal(t, "password", config.RepoCredentials["key1"].HTTPCredential.Password.Key, "Password Key for HTTPCredential does not match")

	assert.NotNil(t, config.RepoCredentials["key2"].SSHCredential, "SSHCredential is missing")
	//assert.Equal(t, "ssh-creds", config.RepoCredentials["key2"].SSHCredential.SSHKey.LocalObjectReference.Name, "SSHKey Name for SSHCredential does not match")
	assert.Equal(t, "sshKey", config.RepoCredentials["key2"].SSHCredential.SSHKey.Key, "SSHKey Key for SSHCredential does not match")

	assert.NotNil(t, config.RepoCredentials["key3"].TLS, "TLS is missing")
	assert.True(t, config.RepoCredentials["key3"].TLS.InsecureSkipVerify, "insecureSkipVerify for TLS does not match")
	//assert.Equal(t, "ca-cert-secret", config.RepoCredentials["key3"].TLS.CACertSecret.Name, "CACertSecret Name for TLS does not match")
	assert.Equal(t, "ca.crt", config.RepoCredentials["key3"].TLS.CACertSecret.Key, "CACertSecret Key for TLS does not match")
	//assert.Equal(t, "cert-secret", config.RepoCredentials["key3"].TLS.CertSecret.Name, "CertSecret Name for TLS does not match")
	assert.Equal(t, "tls.crt", config.RepoCredentials["key3"].TLS.CertSecret.Key, "CertSecret Key for TLS does not match")
	//assert.Equal(t, "key-secret", config.RepoCredentials["key3"].TLS.KeySecret.Name, "KeySecret Name for TLS does not match")
	assert.Equal(t, "tls.key", config.RepoCredentials["key3"].TLS.KeySecret.Key, "KeySecret Key for TLS does not match")
}
