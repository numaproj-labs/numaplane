package config

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"log"
	"os"
	"path/filepath"
	"reflect"
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
	err = configManager.LoadConfig(func(err error) {
	}, configPath)
	config, err := configManager.GetConfig()
	assert.NoError(t, err)

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
	assert.Equal(t, "ssh-creds", config.RepoCredentials["key2"].SSHCredential.SSHKey.LocalObjectReference.Name, "SSHKey Name for SSHCredential does not match")
	assert.Equal(t, "sshKey", config.RepoCredentials["key2"].SSHCredential.SSHKey.Key, "SSHKey Key for SSHCredential does not match")

	assert.NotNil(t, config.RepoCredentials["key3"].TLS, "TLS is missing")
	assert.False(t, config.RepoCredentials["key3"].TLS.InsecureSkipVerify, "insecureSkipVerify for TLS does not match")
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

func TestCloneWithSerialization(t *testing.T) {
	original := &GlobalConfig{
		ClusterName:     "testCluster",
		TimeIntervalSec: 60,
		RepoCredentials: map[string]*GitCredential{
			"repo1": {
				HTTPCredential: &HTTPCredential{
					Username: "user1",
					Password: SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secretName1",
						},
						Key: "password",
					},
				},
				SSHCredential: &SSHCredential{
					SSHKey: SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secretNameSSH",
						},
						Key: "sshKey",
					},
				},
				TLS: &TLS{
					InsecureSkipVerify: true,
					CACertSecret: SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "caCertSecretName",
						},
						Key: "caCert",
					},
				},
			},
		},
	}

	cloned, err := CloneWithSerialization(original)
	if err != nil {
		t.Fatalf("CloneWithSerialization failed: %v", err)
	}
	if cloned == original {
		t.Errorf("Cloned object points to the same instance as original")
	}

	if !reflect.DeepEqual(original, cloned) {
		t.Errorf("Cloned object is not deeply equal to the original")
	}

	cloned.ClusterName = "modifiedCluster"
	if original.ClusterName == "modifiedCluster" {
		t.Errorf("Modifying clone affected the original object")
	}
}

func createGlobalConfigForBenchmarking() *GlobalConfig {
	return &GlobalConfig{
		ClusterName:     "testCluster",
		TimeIntervalSec: 60,
		RepoCredentials: map[string]*GitCredential{
			"repo1": {
				HTTPCredential: &HTTPCredential{
					Username: "user1",
					Password: SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secretName1",
						},
						Key: "password",
					},
				},
				SSHCredential: &SSHCredential{
					SSHKey: SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secretNameSSH",
						},
						Key: "sshKey",
					},
				},
				TLS: &TLS{
					InsecureSkipVerify: true,
					CACertSecret: SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "caCertSecretName",
						},
						Key: "caCert",
					},
				},
			},
		},
	}
}

/**
 go test -bench=. results for CloneWithSerialization
goos: darwin
goarch: arm64
pkg: github.com/numaproj-labs/numaplane/internal/controller/config
BenchmarkCloneWithSerialization-8         241724              5068 ns/op
Used
PASS


goos: linux
goarch: amd64
pkg: main/convert
cpu: Intel(R) Xeon(R) CPU @ 2.20GHz
BenchmarkCloneWithSerialization-6          29619         47935 ns/op
PASS


*/

func BenchmarkCloneWithSerialization(b *testing.B) {
	testConfig := createGlobalConfigForBenchmarking()

	// Reset the timer to exclude the setup time from the benchmark results
	b.ResetTimer()

	// Run the CloneWithSerialization function b.N times
	for i := 0; i < b.N; i++ {
		_, err := CloneWithSerialization(testConfig)
		if err != nil {
			b.Fatalf("CloneWithSerialization failed: %v", err)
		}
	}
}
