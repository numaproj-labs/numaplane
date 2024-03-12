package kubernetes

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/numaproj-labs/numaplane/internal/shared"
)

func TestIsValidKubernetesNamespace(t *testing.T) {
	testCases := []struct {
		name     string
		expected bool
	}{
		{"valid-namespace", true},
		{"INVALID", false},
		{"kubernetes-namespace", false},
		{"kube-system", false},
		{"1234", true},
		{"valid123", true},
		{"valid.namespace", false},
		{"-invalid", false},
		{"invalid-", false},
		{"valid-namespace-with-long-name-123456789012345678901234567890123456789012345678901234567890123", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := IsValidKubernetesNamespace(tc.name)
			if actual != tc.expected {
				t.Errorf("For namespace '%s', expected %v but got %v", tc.name, tc.expected, actual)
			}
		})
	}
}

func TestGetGitSyncInstanceAnnotation(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	err = SetGitSyncInstanceAnnotation(&obj, shared.AnnotationKeyGitSyncInstance, "my-gitsync")
	assert.Nil(t, err)

	annotation, err := GetGitSyncInstanceAnnotation(&obj, shared.AnnotationKeyGitSyncInstance)
	assert.Nil(t, err)
	assert.Equal(t, "my-gitsync", annotation)
}

func TestGetGitSyncInstanceAnnotationWithInvalidData(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc-with-invalid-data.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	_, err = GetGitSyncInstanceAnnotation(&obj, "valid-annotation")
	assert.Error(t, err)
	assert.Equal(t, "failed to get annotations from target object /v1, Kind=Service /my-service: .metadata.annotations accessor error: contains non-string key in the map: <nil> is of the type <nil>, expected string", err.Error())
}

func TestIsValidKubernetesManifestFile(t *testing.T) {

	testCases := []struct {
		name         string
		resourceName string
		expected     bool
	}{
		{
			name:         "Invalid Name",
			resourceName: "data.md",
			expected:     false,
		},

		{
			name:         "valid name",
			resourceName: "my.yml",
			expected:     true,
		},

		{
			name:         "Valid Json file",
			resourceName: "pipeline.json",
			expected:     true,
		},

		{
			name:         "Valid name yaml",
			resourceName: "pipeline.yaml",
			expected:     true,
		},
		{
			name:         "Valid name yaml",
			resourceName: "pipeline.xyz.yaml",
			expected:     true,
		},
		{
			name:         "Valid name yaml",
			resourceName: "pipeline.xyz.hjk.json",
			expected:     true,
		},
		{
			name:         "Invalid File",
			resourceName: "main.go",
			expected:     false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := IsValidKubernetesManifestFile(tc.resourceName)
			assert.Equal(t, tc.expected, ok)
		})
	}

}
