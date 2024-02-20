package validations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckGitURL(t *testing.T) {
	testCases := []struct {
		name     string
		gitUrl   string
		expected bool
	}{
		{
			name:     "Valid Git Url",
			gitUrl:   "https://github.com/numaproj-labs/numaplane",
			expected: true,
		},

		{
			name:     "Valid git url with scp",
			gitUrl:   "https://user:password@host.xz/organization/repo.git?ref=test",
			expected: true,
		},

		{
			name:     "Valid git url with ftp",
			gitUrl:   "file:///path/to/repo.git/",
			expected: true,
		},

		{
			name:     "Valid git url with ssh",
			gitUrl:   "ssh://user-1234@host.xz/path/to/repo.git/tt",
			expected: true,
		},

		{
			name:     "InValid Git Url",
			gitUrl:   "fil://example.com/my-project.git",
			expected: false,
		},
		{
			name:     "InValid Git Url",
			gitUrl:   "someinvalid",
			expected: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := CheckGitURL(tc.gitUrl)
			assert.Equal(t, tc.expected, ok)
		})
	}

}

func TestIsValidName(t *testing.T) {

	testCases := []struct {
		name         string
		resourceName string
		expected     bool
	}{
		{
			name:         "Invalid Name Non Alpha numeric",
			resourceName: "-8991",
			expected:     false,
		},

		{
			name:         "Invalid Name Contains period",
			resourceName: "my.pipeline",
			expected:     false,
		},

		{
			name:         "Invalid Name more than 63 chars",
			resourceName: "mypipeline89898yhgfrt12346tyuh78716tqgfh789765trty12tgy78981278uhyg1qty78",
			expected:     false,
		},

		{
			name:         "Valid name",
			resourceName: "my-pipelines",
			expected:     true,
		},
		{
			name:         "Reserved Keyword",
			resourceName: "kube-233",
			expected:     false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := IsValidKubernetesNamespace(tc.resourceName)
			assert.Equal(t, tc.expected, ok)
		})
	}

}

func TestIsValidManiFestFile(t *testing.T) {

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
			name:         "Invalid File",
			resourceName: "main.go",
			expected:     false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := IsValidManiFestFile(tc.resourceName)
			assert.Equal(t, tc.expected, ok)
		})
	}

}
