package git

import (
	"context"
	"log"
	"testing"

	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/assert"

	"github.com/numaproj-labs/numaplane/internal/util/kubernetes"
	apiv1 "github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
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
			ok := kubernetes.IsValidKubernetesNamespace(tc.resourceName)
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

func TestGetURLScheme(t *testing.T) {

	testCases := []struct {
		name         string
		resourceName string
		scheme       string
	}{
		{
			name:         "should Return shh as scheme",
			resourceName: "git@github.com:shubhamdixit863/september2023web.git",
			scheme:       "ssh",
		},

		{
			name:         "should return ssh as scheme",
			resourceName: "ssh://root@localhost:2222/var/www/git/test.git",
			scheme:       "ssh",
		},
		{
			name:         "should return https as scheme",
			resourceName: "https://github.com/numaflow",
			scheme:       "https",
		},
		{
			name:         "should return http as scheme",
			resourceName: "http://github.com/numaflow",
			scheme:       "http",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url, err := Parse(tc.resourceName)
			log.Println(url.User)
			assert.NoError(t, err)
			assert.Equal(t, tc.scheme, url.Scheme)
		})
	}

}

func GetFakeKubernetesClient() (k8sClient.Client, error) {
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	if err != nil {
		return nil, err

	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()
	return fakeClient, nil
}

// Testing the case when GitSync CRD has a repository path but the RepoCredential Doesn't have the entry for it
func TestGetRepoCloneOptionsPrefixNotFound(t *testing.T) {
	client, err := GetFakeKubernetesClient()
	assert.Nil(t, err)
	// repoCred will be nil in this case
	cloneOptions, err := GetRepoCloneOptions(context.Background(), nil, client, "https://github.com/numaproj-labs/numaplane.git")
	assert.NoError(t, err)
	assert.NotNil(t, cloneOptions)
}

func TestGetRepoCloneOptionsPrefixFoundCredNilHttp(t *testing.T) {
	client, err := GetFakeKubernetesClient()
	assert.Nil(t, err)
	cred := &apiv1.RepoCredential{}
	options, err := GetRepoCloneOptions(context.Background(), cred, client, "https://github.com/numaproj-labs/numaplane.git")
	assert.NoError(t, err)
	assert.Equal(t, options.URL, "https://github.com/numaproj-labs/numaplane.git")
	// In this case only auth method would be nil as it is asssumed that its public repository and doesn't require auth
	assert.Nil(t, options.Auth)
}

func TestGetRepoCloneOptionsPrefixFoundCredNilSSh(t *testing.T) {
	client, err := GetFakeKubernetesClient()
	assert.Nil(t, err)
	cred := &apiv1.RepoCredential{}
	options, err := GetRepoCloneOptions(context.Background(), cred, client, "git@github.com:numaproj-labs/numaplane.git")
	assert.NoError(t, err)
	assert.Equal(t, options.URL, "ssh://git@github.com/numaproj-labs/numaplane.git") // go git transport.endPoint appends the protocol ssh
	// In this case only auth method would be nil as it is asssumed that its public repository and doesn't require auth
	assert.Nil(t, options.Auth)
}

// Testing case when the SSH credentials are provided but both are empty
func TestGetRepoCloneOptionsPrefixFoundCredEmptySSH(t *testing.T) {
	client, err := GetFakeKubernetesClient()
	assert.Nil(t, err)
	cred := &apiv1.RepoCredential{
		SSHCredential: &apiv1.SSHCredential{SSHKey: apiv1.SecretSource{FromKubernetesSecret: &apiv1.SecretKeySelector{
			ObjectReference: corev1.ObjectReference{Name: ""},
			Key:             "",
		}}},
	}
	options, err := GetRepoCloneOptions(context.Background(), cred, client, "git@github.com:numaproj-labs/numaplane.git")
	assert.Error(t, err)
	assert.Nil(t, options)
}

// Testing case when the SSH credentials are provided but the Name is empty
func TestGetRepoCloneOptionsPrefixFoundCredNameEmptySSH(t *testing.T) {
	client, err := GetFakeKubernetesClient()
	assert.Nil(t, err)
	cred := &apiv1.RepoCredential{
		SSHCredential: &apiv1.SSHCredential{SSHKey: apiv1.SecretSource{FromKubernetesSecret: &apiv1.SecretKeySelector{
			ObjectReference: corev1.ObjectReference{Name: "", Namespace: "testnamespace"},
			Key:             "somekey",
		}}},
	}
	options, err := GetRepoCloneOptions(context.Background(), cred, client, "git@github.com:numaproj-labs/numaplane.git")
	assert.Error(t, err)
	assert.Nil(t, options)
}

// Testing case when the SSH credentials are provided but Namespace is empty
func TestGetRepoCloneOptionsPrefixFoundCredNameSpaceEmptySSH(t *testing.T) {
	client, err := GetFakeKubernetesClient()
	assert.Nil(t, err)
	cred := &apiv1.RepoCredential{
		SSHCredential: &apiv1.SSHCredential{SSHKey: apiv1.SecretSource{FromKubernetesSecret: &apiv1.SecretKeySelector{
			ObjectReference: corev1.ObjectReference{Name: "somename"},
			Key:             "somekey",
		}}},
	}
	options, err := GetRepoCloneOptions(context.Background(), cred, client, "git@github.com:numaproj-labs/numaplane.git")
	assert.Error(t, err)
	assert.Nil(t, options)
}

// Testing case when the SSH credentials are provided but the Key is empty
func TestGetRepoCloneOptionsPrefixFoundCredKeyEmptySSH(t *testing.T) {
	client, err := GetFakeKubernetesClient()
	assert.Nil(t, err)
	cred := &apiv1.RepoCredential{
		SSHCredential: &apiv1.SSHCredential{SSHKey: apiv1.SecretSource{FromKubernetesSecret: &apiv1.SecretKeySelector{
			ObjectReference: corev1.ObjectReference{Name: "somename", Namespace: "testnamespace"},
			Key:             "",
		}}},
	}
	options, err := GetRepoCloneOptions(context.Background(), cred, client, "git@github.com:numaproj-labs/numaplane.git")
	assert.Error(t, err)
	assert.Nil(t, options)
}

// Testing case when the HTTP credentials are provided but Name is empty
func TestGetRepoCloneOptionsPrefixFoundCredNameEmptyHTTP(t *testing.T) {
	client, err := GetFakeKubernetesClient()
	assert.Nil(t, err)
	cred := &apiv1.RepoCredential{
		HTTPCredential: &apiv1.HTTPCredential{
			Username: "",
			Password: apiv1.SecretSource{
				FromKubernetesSecret: &apiv1.SecretKeySelector{
					ObjectReference: corev1.ObjectReference{Name: "", Namespace: "testnamespace"},
					Key:             "somekey",
				},
			},
		},
	}
	options, err := GetRepoCloneOptions(context.Background(), cred, client, "https://github.com/numaproj-labs/numaplane.git")
	assert.Error(t, err)
	assert.Nil(t, options)
}

// Testing case when the HTTP credentials are provided but the Key is empty
func TestGetRepoCloneOptionsPrefixFoundCredKeyEmptyHTTP(t *testing.T) {

	client, err := GetFakeKubernetesClient()
	assert.Nil(t, err)
	cred := &apiv1.RepoCredential{
		HTTPCredential: &apiv1.HTTPCredential{
			Username: "",
			Password: apiv1.SecretSource{
				FromKubernetesSecret: &apiv1.SecretKeySelector{
					ObjectReference: corev1.ObjectReference{Name: "somename", Namespace: "testnamespace"},
					Key:             "",
				},
			},
		},
	}
	options, err := GetRepoCloneOptions(context.Background(), cred, client, "https://github.com/numaproj-labs/numaplane.git")
	assert.Error(t, err)
	assert.Nil(t, options)
}

func TestHTTPAuthMethodFile(t *testing.T) {
	client, err := GetFakeKubernetesClient()
	assert.Nil(t, err)

	jsonFile := "testdata/credentials.json"
	repoCredential := &apiv1.RepoCredential{
		URL: "github.com/someorg",
		HTTPCredential: &apiv1.HTTPCredential{
			Username: "someuser",
			Password: apiv1.SecretSource{
				FromFile: &apiv1.FileKeySelector{
					Key:          "http-cred",
					JSONFilePath: &jsonFile,
				},
			},
		},
	}
	auth, _, err := GetAuthMethod(context.Background(), repoCredential, client, "https://github.com/someorg/somerepo.git")
	assert.NoError(t, err)
	basicAuth, ok := auth.(*gitHttp.BasicAuth)
	assert.True(t, ok)
	assert.Equal(t, "someuser", basicAuth.Username)
	assert.Equal(t, "my-password", basicAuth.Password)

	yamlFile := "testdata/credentials.yaml"
	repoCredential = &apiv1.RepoCredential{
		URL: "github.com/someorg",
		HTTPCredential: &apiv1.HTTPCredential{
			Username: "someuser",
			Password: apiv1.SecretSource{
				FromFile: &apiv1.FileKeySelector{
					Key:          "http-cred",
					YAMLFilePath: &yamlFile,
				},
			},
		},
	}
	auth, _, err = GetAuthMethod(context.Background(), repoCredential, client, "https://github.com/someorg/somerepo.git")
	assert.NoError(t, err)
	basicAuth, ok = auth.(*gitHttp.BasicAuth)
	assert.True(t, ok)
	assert.Equal(t, "someuser", basicAuth.Username)
	assert.Equal(t, "my-password", basicAuth.Password)
}
