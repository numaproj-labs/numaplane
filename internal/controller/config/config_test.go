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

	// Function to find the credentials for a given URL
	findCredsByUrl := func(url string) *GitCredential {
		for _, cred := range config.RepoCredentials {
			if cred.URL == url {
				log.Println("printed", cred.URL)
				return cred.Credential
			}
		}
		return nil
	}

	// Test HTTP credentials
	httpCreds := findCredsByUrl("github.com/rustyTest/testprivateRepo")
	log.Println(httpCreds)
	assert.NotNil(t, httpCreds.HTTPCredential, "HTTPCredential is missing for https://github.com/rustytest/testprivaterepo")
	if httpCreds.HTTPCredential != nil {
		assert.Equal(t, "exampleUser", httpCreds.HTTPCredential.Username, "Username for HTTPCredential does not match")
		assert.Equal(t, "http-creds", httpCreds.HTTPCredential.Password.Name, "Password Name for HTTPCredential does not match")
		assert.Equal(t, "password", httpCreds.HTTPCredential.Password.Key, "Password Key for HTTPCredential does not match")
	}

	// Test SSH credentials
	sshCreds := findCredsByUrl("github.com/rustyTest/privateRepo")
	assert.NotNil(t, sshCreds.SSHCredential, "SSHCredential is missing for git@github.com:rustytest/testprivaterepo.git")
	if sshCreds.SSHCredential != nil {
		assert.Equal(t, "ssh-creds", sshCreds.SSHCredential.SSHKey.Name, "SSHKey Name for SSHCredential does not match")
		assert.Equal(t, "sshKey", sshCreds.SSHCredential.SSHKey.Key, "SSHKey Key for SSHCredential does not match")
	}

	// Test TLS credentials
	tlsCreds := findCredsByUrl("key3")
	assert.NotNil(t, tlsCreds.TLS, "TLS is missing for key3")
	if tlsCreds.TLS != nil {
		assert.True(t, tlsCreds.TLS.InsecureSkipVerify, "insecureSkipVerify for TLS does not match")
		assert.Equal(t, "ca-cert-secret-3", tlsCreds.TLS.CACertSecret.Name, "CACertSecret Name for TLS does not match")
		assert.Equal(t, "ca.crt3", tlsCreds.TLS.CACertSecret.Key, "CACertSecret Key for TLS does not match")
		assert.Equal(t, "cert-secret-3", tlsCreds.TLS.CertSecret.Name, "CertSecret Name for TLS does not match")
		assert.Equal(t, "tls.crt3", tlsCreds.TLS.CertSecret.Key, "CertSecret Key for TLS does not match")
		assert.Equal(t, "key-secret-3", tlsCreds.TLS.KeySecret.Name, "KeySecret Name for TLS does not match")
		assert.Equal(t, "tls.key3", tlsCreds.TLS.KeySecret.Key, "KeySecret Key for TLS does not match")
	}
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
