package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfigMatchValues(t *testing.T) {
	getwd, err := os.Getwd()
	assert.Nil(t, err, "Failed to get working directory")
	configPath := filepath.Join(getwd, "../../../", "config", "samples")
	configManager := GetConfigManagerInstance()
	err = configManager.LoadConfig(func(err error) {}, configPath, "config", "yaml")
	assert.NoError(t, err)
	config, err := configManager.GetConfig()
	assert.NoError(t, err)

	assert.Nil(t, err, "Failed to load configuration")

	assert.Equal(t, "staging-usw2-k8s", config.ClusterName, "ClusterName does not match")
	assert.Equal(t, uint(60), config.TimeIntervalSec, "TimeIntervalSec does not match")

	assert.NotNil(t, config.RepoCredentials, "RepoCredentials should not be nil")

	assert.NotNil(t, config.RepoCredentials[0].HTTPCredential, "HTTPCredential for numaproj-labs is missing")
	assert.Equal(t, "exampleUser", config.RepoCredentials[0].HTTPCredential.Username, "Username for HTTPCredential of numaproj-labs does not match")
	assert.Equal(t, "http-creds", config.RepoCredentials[0].HTTPCredential.Password.Name, "Password Name for HTTPCredential of numaproj-labs does not match")
	assert.Equal(t, "password", config.RepoCredentials[0].HTTPCredential.Password.Key, "Password Key for HTTPCredential of numaproj-labs does not match")

	assert.NotNil(t, config.RepoCredentials[0].TLS, "TLS for numaproj-labs is missing")
	assert.NotNil(t, config.RepoCredentials[1].SSHCredential, "SSHCredential for numaproj is missing")
	assert.Equal(t, "ssh-creds", config.RepoCredentials[1].SSHCredential.SSHKey.Name, "SSHKey Name for SSHCredential of numaproj does not match")
	assert.Equal(t, "sshKey", config.RepoCredentials[1].SSHCredential.SSHKey.Key, "SSHKey Key for SSHCredential of numaproj does not match")

	assert.True(t, config.RepoCredentials[1].TLS.InsecureSkipVerify, "insecureSkipVerify for TLS of numaproj does not match")

	assert.NotNil(t, config.RepoCredentials[2].HTTPCredential, "HTTPCredential for numalabs is missing")
	assert.Equal(t, "exampleuser3", config.RepoCredentials[2].HTTPCredential.Username, "Username for HTTPCredential of numalabs does not match")
	assert.Equal(t, "http-creds", config.RepoCredentials[2].HTTPCredential.Password.Name, "Password Name for HTTPCredential of numalabs does not match")
	assert.Equal(t, "password", config.RepoCredentials[2].HTTPCredential.Password.Key, "Password Key for HTTPCredential of numalabs does not match")

	assert.True(t, config.RepoCredentials[2].TLS.InsecureSkipVerify, "insecureSkipVerify for TLS of numalabs does not match")
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
	err = cm.LoadConfig(onError, configDir, "config", "yaml")
	assert.NoError(t, err)
	goroutines := 10
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := cm.GetConfig() // loading config multiple times in go routines
			assert.NoError(t, err)
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
		RepoCredentials: []RepoCredential{
			{
				URL: "repo1",
				HTTPCredential: &HTTPCredential{
					Username: "user1",
					Password: SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secretName1",
						},
						Key:      "password",
						Optional: nil,
					},
				},
				SSHCredential: &SSHCredential{
					SSHKey: SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secretNameSSH",
						},
						Key:      "sshKey",
						Optional: nil,
					},
				},
				TLS: &TLS{
					InsecureSkipVerify: true,
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
		RepoCredentials: []RepoCredential{
			{
				URL: "repo1",
				HTTPCredential: &HTTPCredential{
					Username: "user1",
					Password: SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secretName1",
						},
						Key:      "password",
						Optional: nil,
					},
				},
				SSHCredential: &SSHCredential{
					SSHKey: SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secretNameSSH",
						},
						Key:      "sshKey",
						Optional: nil,
					},
				},
				TLS: &TLS{
					InsecureSkipVerify: true,
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
