package git

import (
	"context"
	"encoding/base64"
	"testing"

	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"

	controllerconfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	mocksClient "github.com/numaproj-labs/numaplane/internal/kubernetes/mocks"
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
		{
			name:     "valid github url",
			gitUrl:   "git@github.com:username/repository.git",
			expected: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := CheckGitURL(tc.gitUrl)
			assert.Equal(t, tc.expected, ok)
		})
	}

}

func TestGetAuthMethod(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	client := mocksClient.NewMockClient(mockCtrl)
	ctx := context.TODO()
	namespace := "test-namespace"

	// Test case for HTTP Credential
	t.Run("HTTPCredential", func(t *testing.T) {
		mockSecret := &corev1.Secret{Data: map[string][]byte{"password": []byte("testpassword")}}
		client.EXPECT().
			GetSecret(ctx, namespace, "http-password-key").
			Return(mockSecret, nil)

		repoCred := &controllerconfig.GitCredential{
			HTTPCredential: &controllerconfig.HTTPCredential{
				Username: "testuser",
				Password: controllerconfig.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "http-password-key"},
					Key:                  "password",
					Optional:             nil,
				},
			},
		}

		authMethod, err := GetAuthMethod(ctx, repoCred, client, namespace)
		assert.Nil(t, err)
		assert.IsType(t, &gitHttp.BasicAuth{}, authMethod)
		httpAuth, _ := authMethod.(*gitHttp.BasicAuth)
		assert.Equal(t, "testuser", httpAuth.Username)
		assert.Equal(t, "testpassword", httpAuth.Password)
	})

	// Test case for SSH Credential
	t.Run("SSHCredential", func(t *testing.T) {
		data, err := base64.StdEncoding.DecodeString("LS0tLS1CRUdJTiBPUEVOU1NIIFBSSVZBVEUgS0VZLS0tLS0KYjNCbGJuTnphQzFyWlhrdGRqRUFBQUFBQkc1dmJtVUFBQUFFYm05dVpRQUFBQUFBQUFBQkFBQUFNd0FBQUF0emMyZ3RaVwpReU5UVXhPUUFBQUNEVTl1elRwT1JwaDJublNtZ2tpSTJQQ25RdzBKbllaUFNGbk1Tc2NSSkltZ0FBQUtBbHBrU1BKYVpFCmp3QUFBQXR6YzJndFpXUXlOVFV4T1FBQUFDRFU5dXpUcE9ScGgybm5TbWdraUkyUENuUXcwSm5ZWlBTRm5NU3NjUkpJbWcKQUFBRUN5YzdoS0RIODJvRGNnNWtrQWkrL3lZNERySEsva2cyZGg2VHduNXhnWTB0VDI3Tk9rNUdtSGFlZEthQ1NJalk4SwpkRERRbWRoazlJV2N4S3h4RWtpYUFBQUFHSEoxYzNSNWNISnZaM0poYldWeVFHZHRZV2xzTG1OdmJRRUNBd1FGCi0tLS0tRU5EIE9QRU5TU0ggUFJJVkFURSBLRVktLS0tLQ==")
		assert.Nil(t, err)
		mockSecret := &corev1.Secret{Data: map[string][]byte{"sshKey": data}}
		client.EXPECT().
			GetSecret(ctx, namespace, "sshKey").
			Return(mockSecret, nil)
		repoCred := &controllerconfig.GitCredential{
			SSHCredential: &controllerconfig.SSHCredential{SSHKey: controllerconfig.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "sshKey"},
				Key:                  "sshKey",
				Optional:             nil,
			},
			},
		}
		authMethod, err := GetAuthMethod(ctx, repoCred, client, namespace)
		assert.NoError(t, err)
		assert.NotNil(t, authMethod, "Expected non-nil authMethod for SSHCredential")
		_, isSSHAuth := authMethod.(*ssh.PublicKeys)
		assert.True(t, isSSHAuth, "Expected SSH auth method")
	})

	t.Run("No Credential", func(t *testing.T) {
		repoCred := &controllerconfig.GitCredential{}
		authMethod, err := GetAuthMethod(ctx, repoCred, client, namespace)
		assert.NoError(t, err)
		assert.Nil(t, authMethod, "Expected nil authMethod for No Credentials")
	})
}

func TestFindCredByUrl(t *testing.T) {
	mockCredential := &controllerconfig.GitCredential{
		HTTPCredential: &controllerconfig.HTTPCredential{
			Username: "testUser",
			Password: controllerconfig.SecretKeySelector{Key: "testKey"},
		},
	}
	testConfig := controllerconfig.GlobalConfig{
		RepoCredentials: []controllerconfig.RepoCredential{
			{
				URL:        "github.com/numaproj-labs",
				Credential: mockCredential,
			},
			{
				URL:        "gitlab.com/another-labs",
				Credential: mockCredential,
			},
			{
				URL:        "bitbucket.org/special-projects",
				Credential: mockCredential,
			},
		},
	}
	testCases := []struct {
		name     string
		gitUrl   string
		config   controllerconfig.GlobalConfig
		expected *controllerconfig.GitCredential
	}{
		{
			name:     "Match existing prefix HTTPS",
			gitUrl:   "https://github.com/numaproj-labs/numaplane",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "Match existing prefix SSH",
			gitUrl:   "git@github.com:numaproj-labs/numaflow-rs.git",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "Match existing prefix Git protocol",
			gitUrl:   "git://github.com/numaproj-labs/legacy-code.git",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "No match for URL",
			gitUrl:   "https://github.com/unknownorg/unknownrepo",
			config:   testConfig,
			expected: nil,
		},
		{
			name:     "Should match with subdirectories",
			gitUrl:   "https://github.com/numaproj-labs/subdir/subsubdir/repo.git",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "Match with different protocol and subdirectories",
			gitUrl:   "git@github.com:numaproj-labs/subdir/another-repo.git",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "No match due to different domain",
			gitUrl:   "https://notgithub.com/numaproj-labs/numaplane",
			config:   testConfig,
			expected: nil,
		},
		{
			name:     "Match GitLab prefix",
			gitUrl:   "https://gitlab.com/another-labs/project",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "No match for GitLab URL with different project",
			gitUrl:   "https://gitlab.com/another-labs/different-project",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "Match Bitbucket prefix",
			gitUrl:   "https://bitbucket.org/special-projects/my-project",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "Match with trailing slash",
			gitUrl:   "https://github.com/numaproj-labs/",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "Match with SSH and username",
			gitUrl:   "ssh://git@github.com/numaproj-labs/numaflow",
			config:   testConfig,
			expected: mockCredential,
		},

		{
			name:     "No match with different subdomain",
			gitUrl:   "https://subdomain.github.com/numaproj-labs/numaflow",
			config:   testConfig,
			expected: nil,
		},
		{
			name:     "Match ignoring user info in SSH URL",
			gitUrl:   "git@github.com:numaproj-labs/numaflow.git",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "Match ignoring user info in SSH URL",
			gitUrl:   "github.com:numaproj-labs",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "No match with incorrect base domain",
			gitUrl:   "https://github.net/numaproj-labs/numaflow",
			config:   testConfig,
			expected: nil,
		},
		{
			name:     "Match with additional URL parameters",
			gitUrl:   "https://github.com/numaproj-labs/numaflow.git?branch=main",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "No match with different repository case sensitivity",
			gitUrl:   "https://github.com/NumaProj-Labs/NumaPlane",
			config:   testConfig,
			expected: nil,
		},
		{
			name:     "Match with URL containing credentials",
			gitUrl:   "https://user:password@github.com/numaproj-labs/numaflow",
			config:   testConfig,
			expected: mockCredential,
		},
		{
			name:     "No match with different Git provider",
			gitUrl:   "https://bitbucket.com/numaproj-labs/numaflow",
			config:   testConfig,
			expected: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := FindCredByUrl(tc.gitUrl, tc.config)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
