package validations

import (
	"context"
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
			ok := CheckGitURL(tc.gitUrl, context.Background())
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
			name:         "Valid name",
			resourceName: "my-pipelines",
			expected:     true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := IsValidName(tc.resourceName)
			assert.Equal(t, tc.expected, ok)
		})
	}

}

func TestIsReservedName(t *testing.T) {

	testCases := []struct {
		name         string
		resourceName string
		expected     bool
	}{

		{
			name:         "Reserved Keyword",
			resourceName: "kube-233",
			expected:     true,
		},
		{
			name:         "Reserved KeyWord",
			resourceName: "kubernetes-123",
			expected:     true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := IsReservedName(tc.resourceName)
			assert.Equal(t, tc.expected, ok)
		})
	}

}
