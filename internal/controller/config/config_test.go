package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	apiv1 "github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfigMatchValues(t *testing.T) {
	getwd, err := os.Getwd()
	assert.Nil(t, err, "Failed to get working directory")
	configPath := filepath.Join(getwd, "../../../", "tests", "config")
	configManager := GetConfigManagerInstance()
	err = configManager.LoadAllConfigs(func(err error) {}, WithConfigsPath(configPath), WithConfigFileName("testconfig"))
	assert.NoError(t, err)
	config, err := configManager.GetConfig()
	assert.NoError(t, err)

	assert.Nil(t, err, "Failed to load configuration")

	assert.Equal(t, "staging-usw2-k8s", config.ClusterName, "ClusterName does not match")
	assert.Equal(t, "/tmp", config.PersistentRepoClonePath, "Persistent Repo Clone Path Doesn't match")
	assert.Equal(t, 30000, config.SyncTimeIntervalMs, "SyncTimeIntervalMs does not match")
	assert.Equal(t, false, config.AutoHealDisabled, "AutoHealDisabled does not match")
	assert.Equal(t, false, config.AutomatedSyncDisabled, "AutomatedSyncDisabled does not match")
	assert.Equal(t, false, config.CascadeDeletion, "CascadeDeletion Field does not match")
	assert.Equal(t, "group=apps,kind=Deployment;"+
		"group=,kind=ConfigMap;group=,kind=Secret;group=,kind=ServiceAccount;group=,kind=Namespace;"+
		"group=numaflow.numaproj.io,kind=;"+
		"group=numaplane.numaproj.io,kind=;"+
		"group=rbac.authorization.k8s.io,kind=RoleBinding;group=rbac.authorization.k8s.io,kind=Role",
		config.IncludedResources, "IncludedResources does not match")

	assert.NotNil(t, config.RepoCredentials, "RepoCredentials should not be nil")

	assert.NotNil(t, config.RepoCredentials[0].HTTPCredential, "HTTPCredential for numaproj-labs is missing")
	assert.Equal(t, "exampleUser", config.RepoCredentials[0].HTTPCredential.Username, "Username for HTTPCredential of numaproj-labs does not match")
	assert.Equal(t, "http-creds", config.RepoCredentials[0].HTTPCredential.Password.FromKubernetesSecret.Name, "Password Name for HTTPCredential of numaproj-labs does not match")
	assert.Equal(t, "password", config.RepoCredentials[0].HTTPCredential.Password.FromKubernetesSecret.Key, "Password Key for HTTPCredential of numaproj-labs does not match")
	assert.Equal(t, "numaplane-controller", config.RepoCredentials[0].HTTPCredential.Password.FromKubernetesSecret.Namespace, "Kubernetes namespace for password doesn't match")
	assert.NotNil(t, config.RepoCredentials[0].TLS, "TLS for numaproj-labs is missing")

	assert.NotNil(t, config.RepoCredentials[1].SSHCredential, "SSHCredential for numaproj is missing")
	assert.Equal(t, "ssh-creds", config.RepoCredentials[1].SSHCredential.SSHKey.FromKubernetesSecret.Name, "SSHKey Name for SSHCredential of numaproj does not match")
	assert.Equal(t, "sshKey", config.RepoCredentials[1].SSHCredential.SSHKey.FromKubernetesSecret.Key, "SSHKey Key for SSHCredential of numaproj does not match")
	assert.True(t, config.RepoCredentials[1].TLS.InsecureSkipVerify, "insecureSkipVerify for TLS of numaproj does not match")

	assert.NotNil(t, config.RepoCredentials[2].HTTPCredential, "HTTPCredential for numalabs is missing")
	assert.Equal(t, "exampleuser2", config.RepoCredentials[2].HTTPCredential.Username, "Username for HTTPCredential of numalabs does not match")
	assert.NotNil(t, config.RepoCredentials[2].HTTPCredential.Password.FromFile.JSONFilePath, "JSON File Path didn't get set")
	assert.Equal(t, "password", config.RepoCredentials[2].HTTPCredential.Password.FromFile.Key, "Password Key for HTTPCredential of numalabs does not match")
	assert.True(t, config.RepoCredentials[2].TLS.InsecureSkipVerify, "insecureSkipVerify for TLS of numalabs does not match")

	assert.NotNil(t, config.RepoCredentials[3].SSHCredential, "SSHCredential for numaproj is missing")
	assert.NotNil(t, config.RepoCredentials[3].SSHCredential.SSHKey.FromFile, "SSHCredential for numaproj not of type File")
	assert.NotNil(t, config.RepoCredentials[3].SSHCredential.SSHKey.FromFile.YAMLFilePath, "YAML File Path for SSHCredential of numaproj not set")
	assert.Equal(t, "sshKey", config.RepoCredentials[3].SSHCredential.SSHKey.FromFile.Key, "SSHKey Key for SSHCredential of numaproj does not match")
	assert.True(t, config.RepoCredentials[3].TLS.InsecureSkipVerify, "insecureSkipVerify for TLS of numaproj does not match")

	// now verify that if we modify the file, it will still be okay
	originalFile := "../../../tests/config/testconfig.yaml"
	fileToCopy := "../../../tests/config/testconfig2.yaml"
	originalFileBytes, err := os.ReadFile(originalFile)
	assert.Nil(t, err, "Failed to read config file")
	defer func() { // make sure this gets written back to what it was at the end of the test
		_ = os.WriteFile(originalFile, originalFileBytes, 0644)
	}()

	err = copyFile(fileToCopy, originalFile)
	assert.NoError(t, err)
	time.Sleep(10 * time.Second) // need to give it time to make sure that the file was reloaded
	config, err = configManager.GetConfig()
	assert.NoError(t, err)

	assert.Equal(t, "staging-usw2-k8s", config.ClusterName, "ClusterName does not match")
	assert.Equal(t, 30000, config.SyncTimeIntervalMs, "SyncTimeIntervalMs does not match")
	assert.Len(t, config.RepoCredentials, 0, "RepoCredentials should not be present")

}

func TestLoadNumaRolloutConfigMatchValues(t *testing.T) {
	getwd, err := os.Getwd()
	assert.Nil(t, err, "Failed to get working directory")
	configPath := filepath.Join(getwd, "../../../", "tests", "config")
	configManager := GetConfigManagerInstance()
	err = configManager.LoadAllConfigs(func(err error) {}, WithDefConfigPath(configPath), WithDefConfigFileName("controller-definitions-config"))
	assert.NoError(t, err)
	config, err := configManager.GetControllerDefinitionsConfig()
	assert.NoError(t, err)

	assert.Nil(t, err, "Failed to load configuration")

	assert.NotNil(t, config.ControllerDefinitions, "ControllerDefinitions should not be nil")

	assert.Equal(t, "1.2.1", config.ControllerDefinitions[0].Version, "Version for ControllerDefinitions[0] does not match")
	assert.Equal(t, "apiVersion: apps/v1\nkind: Deployment\n", config.ControllerDefinitions[0].FullSpec, "FullSpec for ControllerDefinitions[0] does not match")
	assert.Equal(t, "1.1.7", config.ControllerDefinitions[1].Version, "Version for ControllerDefinitions[1] does not match")
	assert.Equal(t, "", config.ControllerDefinitions[1].FullSpec, "FullSpec for ControllerDefinitions[1] does not match")
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = source.Close()
	}()

	destination, err := os.Create(dst) // create only if it doesn't exist
	if err != nil {
		return err
	}
	defer func() {
		_ = destination.Close()
	}()
	_, err = io.Copy(destination, source)
	return err
}

// to verify this test run with go test -race ./... it  won't give a race condition as we have used mutex.RwLock
// in onConfigChange in LoadAllConfigs
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
	err = cm.LoadAllConfigs(onError, WithConfigsPath(configDir), WithConfigFileName("config"))
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
		ClusterName:        "testCluster",
		SyncTimeIntervalMs: 60000,
		RepoCredentials: []apiv1.RepoCredential{
			{
				URL: "repo1",
				HTTPCredential: &apiv1.HTTPCredential{
					Username: "user1",
					Password: apiv1.SecretSource{
						FromKubernetesSecret: &apiv1.SecretKeySelector{
							ObjectReference: corev1.ObjectReference{
								Name: "secretName1",
							},
							Key: "password",
						},
					},
				},
				SSHCredential: &apiv1.SSHCredential{
					SSHKey: apiv1.SecretSource{
						FromKubernetesSecret: &apiv1.SecretKeySelector{
							ObjectReference: corev1.ObjectReference{
								Name: "secretNameSSH",
							},
							Key: "sshKey",
						},
					},
				},
				TLS: &apiv1.TLS{
					InsecureSkipVerify: true,
				},
			},
		}}

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
		ClusterName:        "testCluster",
		SyncTimeIntervalMs: 60000,
		RepoCredentials: []apiv1.RepoCredential{
			{
				URL: "repo1",
				HTTPCredential: &apiv1.HTTPCredential{
					Username: "user1",
					Password: apiv1.SecretSource{
						FromKubernetesSecret: &apiv1.SecretKeySelector{
							ObjectReference: corev1.ObjectReference{
								Name: "secretName1",
							},
							Key: "password",
						},
					},
				},
				SSHCredential: &apiv1.SSHCredential{
					SSHKey: apiv1.SecretSource{
						FromKubernetesSecret: &apiv1.SecretKeySelector{
							ObjectReference: corev1.ObjectReference{
								Name: "secretNameSSH",
							},
							Key: "sshKey",
						},
					},
				},

				TLS: &apiv1.TLS{
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
