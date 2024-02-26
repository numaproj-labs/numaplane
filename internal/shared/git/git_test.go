package git

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/numaproj-labs/numaplane/internal/shared/k8"
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
			ok := k8.IsValidKubernetesNamespace(tc.resourceName)
			assert.Equal(t, tc.expected, ok)
		})
	}

}

func TestParse(t *testing.T) {

	testCases := []struct {
		name         string
		resourceName string
		expected     string
	}{
		{
			name:         "should Return git as username",
			resourceName: "git@github.com:shubhamdixit863/september2023web.git",
			expected:     "git",
		},

		{
			name:         "should return root as username",
			resourceName: "ssh://root@localhost:2222/var/www/git/test.git",
			expected:     "root",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url, err := Parse(tc.resourceName)
			log.Println(url.User)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, url.User.Username())
		})
	}

}
