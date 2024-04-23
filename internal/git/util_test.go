package git

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/golang/mock/gomock"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	cryptossh "golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gitshared "github.com/numaproj-labs/numaplane/internal/util/git"
	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
)

const (
	testNamespace = "test-ns"
)

var pool *dockertest.Pool

// TestMain configures the testing environment by initializing a Docker container that serves as a local git server.
// This function handles the connection to Docker, initiates the specified container if it's not currently active,
// and performs test case execution. Post-execution, it ensures thorough cleanup of all utilized resources.
// Inside the Docker container, it sets up a test repository accessible via:
// - SSH at ssh://root@localhost:2222/var/www/git/test.git
// - HTTP at http://localhost:8080/git/test.git, served by the Apache HTTP server
// - HTTPS at https://localhost:8443/git/test.git, secured with a self-signed certificate.
// The default credentials for HTTP access are root:root.
// The default credentials for SSH access are also root:root.
// A default public SSH key (ssh-ed25519) is pre-added to the container's authorized keys.
// The corresponding private key should be used to clone the repository.
func TestMain(m *testing.M) {
	// Connect to Docker
	p, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to Docker; is it running? %s", err)
	}
	pool = p

	relativePath := "../../tests/e2e-gitserver"
	absolutePath, err := filepath.Abs(relativePath)
	if err != nil {
		log.Fatalf("Failed to get absolute path: %s", err)
	}

	authorizedKeys := fmt.Sprintf("%s/authorized_keys:/root/.ssh/authorized_keys", absolutePath)
	httPasswd := fmt.Sprintf("%s/.htpasswd:/auth/.htpasswd", absolutePath)
	// Start the local Git server container if not already running
	opts := dockertest.RunOptions{
		Repository:   "quay.io/numaio/numaplane-e2e-gitserver",
		Tag:          "latest",
		ExposedPorts: []string{"22", "80", "443"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"22/tcp":  {{HostIP: "127.0.0.1", HostPort: "2222"}},
			"443/tcp": {{HostIP: "127.0.0.1", HostPort: "8443"}},
			"80/tcp":  {{HostIP: "127.0.0.1", HostPort: "8080"}},
		},
		Mounts: []string{
			authorizedKeys,
			httPasswd,
		},
	}
	resource, err := pool.RunWithOptions(&opts)
	if err != nil {
		log.Fatalf("Could not start the Docker resource: %s", err)
	}

	// Wait for the local Git server to be ready
	if err := pool.Retry(func() error {
		// Implement checks for both the Apache server and SSH service here

		// Check Apache server availability
		client := &http.Client{}
		req, err := http.NewRequest("GET", "http://localhost:8080", nil)
		if err != nil {
			return fmt.Errorf("error creating request: %s", err)
		}
		req.SetBasicAuth("root", "root")
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			return fmt.Errorf("apache server not yet ready %s", err.Error())
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				log.Println(err)
			}
		}(resp.Body)

		// Check SSH service availability
		_, err = net.Dial("tcp", "localhost:2222")
		if err != nil {
			return fmt.Errorf("SSH service not yet ready")
		}

		// If both checks pass, return nil to indicate success
		return nil
	}); err != nil {
		if resource != nil {
			_ = pool.Purge(resource)
		}
		log.Fatalf("Could not ensure local Git server services were ready: %s", err)
	}

	// Execute the test cases
	code := m.Run()

	// Ensure resources are cleaned up
	if resource != nil {
		if err := pool.Purge(resource); err != nil {
			log.Fatalf("Couldn't purge the Docker resource: %s", err)
		}
	}

	// Exit with the status code from the test execution
	os.Exit(code)
}

func newGitSync(name string, repoUrl string, path string, targetRevision string) *v1alpha1.GitSync {
	return &v1alpha1.GitSync{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
		},
		Spec: v1alpha1.GitSyncSpec{
			GitSource: v1alpha1.GitSource{
				GitLocation: v1alpha1.GitLocation{
					RepoUrl:        repoUrl,
					TargetRevision: targetRevision,
					Path:           path,
				},
			},
		},
		Status: v1alpha1.GitSyncStatus{},
	}
}

func Test_cloneRepo(t *testing.T) {
	testCases := []struct {
		name    string
		gitSync *v1alpha1.GitSync
		hasErr  bool
	}{
		{
			name: "valid repo",
			gitSync: newGitSync(
				"validRepo",
				"https://github.com/numaproj-labs/numaplane.git",
				"",
				"main",
			),
			hasErr: false,
		},
	}

	t.Parallel()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			localRepoPath := getLocalRepoPath(tc.gitSync)
			err := os.RemoveAll(localRepoPath)
			assert.Nil(t, err)
			cloneOptions := &git.CloneOptions{
				URL: tc.gitSync.Spec.RepoUrl,
			}
			r, cloneErr := cloneRepo(context.Background(), tc.gitSync, cloneOptions)
			assert.NoError(t, cloneErr)
			if tc.hasErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, r)
			}
		})
	}
}

func Test_GetLatestManifests(t *testing.T) {
	testCases := []struct {
		name    string
		gitSync *v1alpha1.GitSync
		hasErr  bool
	}{
		{
			name: "`main` as a TargetRevision",
			gitSync: newGitSync(
				"main",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"staging-usw2-k8s",
				"main",
			),
			hasErr: false,
		},
		{
			name: "tag name as a TargetRevision",
			gitSync: newGitSync(
				"tagName",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"staging-usw2-k8s",
				"v0.0.1",
			),
			hasErr: false,
		},
		{
			name: "commit hash as a TargetRevision",
			gitSync: newGitSync(
				"commitHash",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"staging-usw2-k8s",
				"7b68200947f2d2624797e56edf02c6d848bc48d1",
			),
			hasErr: false,
		},
		{
			name: "remote branch name as a TargetRevision",
			gitSync: newGitSync(
				"remoteBranch",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"staging-usw2-k8s",
				"refs/remotes/origin/pipeline",
			),
			hasErr: false,
		},
		{
			name: "local branch name as a TargetRevision",
			gitSync: newGitSync(
				"localBranch",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"staging-usw2-k8s",
				"main",
			),
			hasErr: false,
		},
		{
			name: "root path",
			gitSync: newGitSync(
				"rootPath",
				"https://github.com/numaproj-labs/numaplane-control-manifests.git",
				"",
				"main",
			),
			hasErr: false,
		},
		{
			name: "unresolvable TargetRevision",
			gitSync: newGitSync(
				"unresolvableTargetRevision",
				"https://github.com/numaproj-labs/numaplane.git",
				"config/samples",
				"unresolvable",
			),
			hasErr: true,
		},
		{
			name: "invalid path",
			gitSync: newGitSync(
				"invalidPath",
				"https://github.com/numaproj-labs/numaplane.git",
				"invalid_path",
				"main",
			),
			hasErr: true,
		},
		{
			name: "manifest from kustomize enabled repo",
			gitSync: &v1alpha1.GitSync{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      "kustomize-manifest",
				},
				Spec: v1alpha1.GitSyncSpec{
					GitSource: v1alpha1.GitSource{
						GitLocation: v1alpha1.GitLocation{
							RepoUrl:        "https://github.com/numaproj/numaflow.git",
							TargetRevision: "main",
							Path:           "config/namespace-install",
						},
						Kustomize: &v1alpha1.KustomizeSource{},
					},
				},
			},
			hasErr: false,
		},
		{
			name: "manifest from helm enabled repo",
			gitSync: &v1alpha1.GitSync{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      "helm-manifest",
				},
				Spec: v1alpha1.GitSyncSpec{
					GitSource: v1alpha1.GitSource{
						GitLocation: v1alpha1.GitLocation{
							RepoUrl:        "https://github.com/numaproj/helm-charts.git",
							TargetRevision: "main",
							Path:           "charts/numaflow",
						},
						Helm: &v1alpha1.HelmSource{},
					},
				},
			},
			hasErr: false,
		},
	}
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {
			localRepoPath := getLocalRepoPath(tc.gitSync)
			err := os.RemoveAll(localRepoPath)
			assert.Nil(t, err)
			cloneOptions := &git.CloneOptions{
				URL: tc.gitSync.Spec.RepoUrl,
			}
			r, cloneErr := cloneRepo(context.Background(), tc.gitSync, cloneOptions)
			assert.Nil(t, cloneErr)

			// To break the continuous check of repo update, added the context timeout.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			_, _, err = GetLatestManifests(ctx, r, nil, tc.gitSync)
			if tc.hasErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestGetLatestCommitHash(t *testing.T) {
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:          "https://github.com/shubhamdixit863/testingrepo",
		SingleBranch: true,
	})
	assert.Nil(t, err)
	commit, err := getLatestCommitHash(r, "main")
	assert.Nil(t, err)
	assert.Equal(t, 40, len(commit.String()))
}

// Testing cloning repo with authentication

func GetFakeKubernetesClient(secret *corev1.Secret) (k8sClient.Client, error) {
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	if err != nil {
		return nil, err

	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	return fakeClient, nil
}

func FileExists(repo *git.Repository, fileName string) error {
	head, err := repo.Head()
	if err != nil {
		return err
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return err
	}
	tree, err := commit.Tree()
	if err != nil {
		return err
	}
	_, err = tree.File(fileName)
	if err != nil {
		return err
	}

	return nil
}

// Tests With Local Git server running inside docker container

func TestGitCloneRepoSshLocalGitServer(t *testing.T) {

	// below private key is the pair of public key that has been set in authorized key in docker container
	secretData := map[string][]byte{"sshKey": []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBBwLbaNG1RkwFAEyYYg04ymv5WYNS2LvoHSXMp+HCvAwAAAIhcN3LMXDdy
zAAAAAtzc2gtZWQyNTUxOQAAACBBwLbaNG1RkwFAEyYYg04ymv5WYNS2LvoHSXMp+HCvAw
AAAECl1AymWUHNdRiOu2r2dg97arF3S32bE5zcPTqynwyw50HAtto0bVGTAUATJhiDTjKa
/lZg1LYu+gdJcyn4cK8DAAAABWxvY2Fs
-----END OPENSSH PRIVATE KEY-----`)}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sshKey",
			Namespace: testNamespace,
		},
		Data: secretData,
	}

	client, err := GetFakeKubernetesClient(secret)
	assert.Nil(t, err)

	credential := &v1alpha1.RepoCredential{
		SSHCredential: &v1alpha1.SSHCredential{SSHKey: v1alpha1.SecretKeySelector{
			ObjectReference: corev1.ObjectReference{Name: "sshKey", Namespace: testNamespace},
			Key:             "sshKey",
			Optional:        nil,
		}},
	}
	repoUrL := "ssh://root@localhost:2222/var/www/git/repo1.git"
	cloneOptions, err := gitshared.GetRepoCloneOptions(context.Background(), credential, client, repoUrL, "master")
	assert.NoError(t, err)
	assert.NotNil(t, cloneOptions)

	cloneOptions.Auth.(*ssh.PublicKeys).HostKeyCallback = cryptossh.InsecureIgnoreHostKey()

	gitSync := newGitSync("test", repoUrL, "gitClone", "master")

	repo, err := cloneRepo(context.Background(), gitSync, cloneOptions)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	err = FileExists(repo, "data.yaml") // data.yaml default file exists in docker git
	assert.NoError(t, err)
	err = os.RemoveAll("gitClone")
	assert.NoError(t, err)

}

func TestGitCloneRepoWithTag(t *testing.T) {
	repoUrL := "https://github.com/numaproj-labs/numaplane"
	cloneOptions, err := gitshared.GetRepoCloneOptions(context.Background(), nil, nil, repoUrL, "tag/v0.1.0-beta.2")
	assert.NoError(t, err)
	assert.IsType(t, &git.CloneOptions{}, cloneOptions)
	gitSync := newGitSync("test", repoUrL, "gitCloned", "tag/v0.1.0-beta.2")

	repo, err := cloneRepo(context.Background(), gitSync, cloneOptions)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	err = FileExists(repo, "README.md") // data.yaml default file exists in docker git
	assert.NoError(t, err)
	err = os.RemoveAll("gitCloned")
	assert.NoError(t, err)
}

func TestGitCloneRepoWithBranch(t *testing.T) {
	repoUrL := "https://github.com/numaproj-labs/numaplane"
	cloneOptions, err := gitshared.GetRepoCloneOptions(context.Background(), nil, nil, repoUrL, "xxww")
	assert.NoError(t, err)
	assert.IsType(t, &git.CloneOptions{}, cloneOptions)
	gitSync := newGitSync("test", repoUrL, "gitCloned", "xxww")

	repo, err := cloneRepo(context.Background(), gitSync, cloneOptions)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	err = FileExists(repo, "README.md")
	assert.NoError(t, err)
	err = os.RemoveAll("gitCloned")
	assert.NoError(t, err)
}

func TestGitCloneRepoWithCommitHash(t *testing.T) {
	repoUrL := "https://github.com/numaproj-labs/numaplane"
	cloneOptions, err := gitshared.GetRepoCloneOptions(context.Background(), nil, nil, repoUrL, "4c74d04da72f5ede0e25bed4f1974286b3e698bb")
	assert.NoError(t, err)
	assert.IsType(t, &git.CloneOptions{}, cloneOptions)
	gitSync := newGitSync("test", repoUrL, "gitCloned", "4c74d04da72f5ede0e25bed4f1974286b3e698bb")

	repo, err := cloneRepo(context.Background(), gitSync, cloneOptions)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	err = FileExists(repo, "README.md")
	assert.NoError(t, err)
	err = os.RemoveAll("gitCloned")
	assert.NoError(t, err)
}

func TestGitCloneRepoHTTPLocalGitServer(t *testing.T) {

	secretData := map[string][]byte{"password": []byte(`root`)}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "http-cred",
			Namespace: testNamespace,
		},
		Data: secretData,
	}

	client, err := GetFakeKubernetesClient(secret)
	assert.Nil(t, err)

	credential := &v1alpha1.RepoCredential{
		HTTPCredential: &v1alpha1.HTTPCredential{
			Username: "root",
			Password: v1alpha1.SecretKeySelector{
				ObjectReference: corev1.ObjectReference{Name: "http-cred", Namespace: testNamespace},
				Key:             "password",
				Optional:        nil,
			},
		},
	}
	repoUrL := "http://localhost:8080/git/repo1.git"
	cloneOptions, err := gitshared.GetRepoCloneOptions(context.Background(), credential, client, repoUrL, "master")
	assert.NoError(t, err)
	assert.IsType(t, &git.CloneOptions{}, cloneOptions)
	gitSync := newGitSync("test", repoUrL, "gitCloned", "master")

	repo, err := cloneRepo(context.Background(), gitSync, cloneOptions)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	err = FileExists(repo, "data.yaml") // data.yaml default file exists in docker git
	assert.NoError(t, err)
	err = os.RemoveAll("gitCloned")
	assert.NoError(t, err)
}

func TestGitCloneRepoHTTPSLocalGitServer(t *testing.T) {

	secretData := map[string][]byte{"password": []byte(`root`)}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "http-cred",
			Namespace: testNamespace,
		},
		Data: secretData,
	}

	client, err := GetFakeKubernetesClient(secret)
	assert.Nil(t, err)

	credential := &v1alpha1.RepoCredential{
		HTTPCredential: &v1alpha1.HTTPCredential{
			Username: "root",
			Password: v1alpha1.SecretKeySelector{
				ObjectReference: corev1.ObjectReference{Name: "http-cred", Namespace: testNamespace},
				Key:             "password",
				Optional:        nil,
			},
		},
		TLS: &v1alpha1.TLS{
			InsecureSkipVerify: true, // As we are using local ca certificates in docker container so skipping strict checking

		},
	}
	repoUrL := "https://localhost:8443/git/repo1.git"
	cloneOptions, err := gitshared.GetRepoCloneOptions(context.Background(), credential, client, repoUrL, "master")
	assert.NoError(t, err)
	assert.IsType(t, &git.CloneOptions{}, cloneOptions)
	gitSync := newGitSync("test", repoUrL, "gitCloned", "master")
	repo, err := cloneRepo(context.Background(), gitSync, cloneOptions)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	err = FileExists(repo, "data.yaml") // data.yaml default file exists in docker git
	assert.NoError(t, err)
	err = os.RemoveAll("gitCloned")
	assert.NoError(t, err)
}
