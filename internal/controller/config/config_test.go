package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfigMatchValues(t *testing.T) {
	getwd, err := os.Getwd()
	assert.Nil(t, err, "Failed to get working directory")
	configPath := filepath.Join(getwd, "../../../", "config", "samples")
	configManager := GetConfigManagerInstance()
	config := configManager.GetConfig()
	err = configManager.LoadConfig(func(err error) {
	}, configPath)
	assert.Nil(t, err, "Failed to load configuration")

	log.Printf("Loaded Config: %+v", config)

	assert.Equal(t, "example-cluster", config.ClusterName, "ClusterName does not match")
	assert.Equal(t, uint(60), config.TimeIntervalSec, "TimeIntervalSec does not match")

	assert.NotNil(t, config.RepoCredentials, "RepoCredentials should not be nil")

	assert.NotNil(t, config.RepoCredentials["https://github.com/rustytest/testprivaterepo"].HTTPCredential, "HTTPCredential is missing")
	assert.Equal(t, "exampleUser", config.RepoCredentials["https://github.com/rustytest/testprivaterepo"].HTTPCredential.Username, "Username for HTTPCredential does not match")
	assert.Equal(t, "http-creds", config.RepoCredentials["https://github.com/rustytest/testprivaterepo"].HTTPCredential.Password.Name, "Password Name for HTTPCredential does not match")
	assert.Equal(t, "password", config.RepoCredentials["https://github.com/rustytest/testprivaterepo"].HTTPCredential.Password.Key, "Password Key for HTTPCredential does not match")

	assert.NotNil(t, config.RepoCredentials["git@github.com:rustytest/testprivaterepo.git"].SSHCredential, "SSHCredential is missing")
	assert.Equal(t, "ssh-creds", config.RepoCredentials["git@github.com:rustytest/testprivaterepo.git"].SSHCredential.SSHKey.LocalObjectReference.Name, "SSHKey Name for SSHCredential does not match")
	assert.Equal(t, "sshKey", config.RepoCredentials["git@github.com:rustytest/testprivaterepo.git"].SSHCredential.SSHKey.Key, "SSHKey Key for SSHCredential does not match")

	assert.NotNil(t, config.RepoCredentials["key3"].TLS, "TLS is missing")
	assert.True(t, config.RepoCredentials["key3"].TLS.InsecureSkipVerify, "insecureSkipVerify for TLS does not match")
	assert.Equal(t, "ca-cert-secret", config.RepoCredentials["key3"].TLS.CACertSecret.Name, "CACertSecret Name for TLS does not match")
	assert.Equal(t, "ca.crt", config.RepoCredentials["key3"].TLS.CACertSecret.Key, "CACertSecret Key for TLS does not match")
	assert.Equal(t, "cert-secret", config.RepoCredentials["key3"].TLS.CertSecret.Name, "CertSecret Name for TLS does not match")
	assert.Equal(t, "tls.crt", config.RepoCredentials["key3"].TLS.CertSecret.Key, "CertSecret Key for TLS does not match")
	assert.Equal(t, "key-secret", config.RepoCredentials["key3"].TLS.KeySecret.Name, "KeySecret Name for TLS does not match")
	assert.Equal(t, "tls.key", config.RepoCredentials["key3"].TLS.KeySecret.Key, "KeySecret Key for TLS does not match")
}

// to verify this test run with go test -race ./... it  won't give a race condition as we have used mutex.RwLock
// in onConfigChange in LoadConfig
func TestConfigManager_LoadConfigNoRace(t *testing.T) {
	configDir := os.TempDir()

	configPath := filepath.Join(configDir, "config.yaml")
	_, err := os.Create(configPath)
	assert.NoError(t, err)

	configContent := []byte("initial: value\n")
	err = os.WriteFile(configPath, configContent, 0644)
	assert.NoError(t, err)
	defer func(name string) {
		err := os.Remove(name)
		assert.Nil(t, err)
	}(configPath)

	// configManager
	cm := GetConfigManagerInstance()

	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	onError := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		errors = append(errors, err)
	}
	// concurrent Access of files
	goroutines := 10
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			err := cm.LoadConfig(onError, configDir)
			assert.NoError(t, err)
		}()
	}
	triggers := 10
	wg.Add(triggers)
	// Modification Trigger
	for i := 0; i < triggers; i++ {
		go func() {
			defer wg.Done()
			time.Sleep(1 * time.Second)
			newConfigContent := []byte("modified: value\n")
			err := os.WriteFile(configPath, newConfigContent, 0644)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
	assert.Len(t, errors, 0, fmt.Sprintf("There should be no errors, got: %v", errors))
}

func TestGetConfigManagerInstanceSingleton(t *testing.T) {
	var instance1 *ConfigManager
	var instance2 *ConfigManager
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		instance1 = GetConfigManagerInstance()
	}()
	go func() {
		defer wg.Done()
		instance2 = GetConfigManagerInstance()
	}()

	wg.Wait()
	assert.NotNil(t, instance1)
	assert.NotNil(t, instance2)
	assert.Equal(t, instance1, instance2) // they should give same memory address

}
