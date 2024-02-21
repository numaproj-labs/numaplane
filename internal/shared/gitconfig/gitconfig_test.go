package gitconfig

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
				Password: controllerconfig.SecretKeySelector{Key: "http-password-key"},
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
				Key: "sshKey",
			}},
		}
		authMethod, err := GetAuthMethod(ctx, repoCred, client, namespace)
		assert.NoError(t, err)
		assert.NotNil(t, authMethod, "Expected non-nil authMethod for SSHCredential")
		_, isSSHAuth := authMethod.(*ssh.PublicKeys)
		assert.True(t, isSSHAuth, "Expected SSH auth method")
	})

}
