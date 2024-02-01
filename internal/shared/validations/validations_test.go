package validations

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
			name:     "InValid Git Url",
			gitUrl:   "https://test.com/test",
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
			name:         "InValid Reserved Keyword",
			resourceName: "kube-233",
			expected:     false,
		},
		{
			name:         "Valid name",
			resourceName: "my-pipelines",
			expected:     false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := CheckGitURL(tc.resourceName)
			assert.Equal(t, tc.expected, ok)
		})
	}

}
