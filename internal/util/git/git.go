package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	controllerConfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/util/kubernetes"
	"github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
	apiv1 "github.com/numaproj-labs/numaplane/pkg/apis/numaplane/v1alpha1"
)

// GetAuthMethod returns an authMethod for both cloning and fetching from a repo with HTTP, SSH, or TLS credentials from Kubernetes secrets.
func GetAuthMethod(ctx context.Context, repoCred *apiv1.RepoCredential, kubeClient k8sClient.Client, repoUrl string) (transport.AuthMethod, bool, error) {
	scheme, err := GetURLScheme(repoUrl)
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse URL scheme: %w", err)
	}

	var auth transport.AuthMethod
	var insecureSkipTLS bool
	if repoCred != nil {
		// Configure TLS if applicable
		if repoCred.TLS != nil {
			insecureSkipTLS = repoCred.TLS.InsecureSkipVerify
		}

		switch scheme {
		case "http", "https":
			if cred := repoCred.HTTPCredential; cred != nil {
				password, err := getSecretValue(ctx, kubeClient, cred.Password)
				if err != nil {
					return nil, false, fmt.Errorf("failed to get HTTP credential: %w", err)
				}
				auth = &gitHttp.BasicAuth{
					Username: cred.Username,
					Password: password,
				}

			}

		case "ssh":
			if cred := repoCred.SSHCredential; cred != nil {
				sshKey, err := getSecretValue(ctx, kubeClient, cred.SSHKey)
				if err != nil {
					return nil, false, fmt.Errorf("Failed to get SSH credential: %w", err)
				}
				parsedUrl, err := Parse(repoUrl)
				if err != nil {
					return nil, false, err
				}
				auth, err = ssh.NewPublicKeys(parsedUrl.User.Username(), []byte(sshKey), "")
				if err != nil {
					return nil, false, fmt.Errorf("failed to create SSH public keys: %w", err)
				}
			}
		default:
			return nil, false, fmt.Errorf("unsupported URL scheme: %s", scheme)
		}
	}

	return auth, insecureSkipTLS, nil
}

// get a secret value, either from a File or from a Kubernetes Secret
func getSecretValue(ctx context.Context, kubeClient k8sClient.Client, secretSource v1alpha1.SecretSource) (string, error) {
	var secretValue string
	var err error
	if secretSource.FromKubernetesSecret != nil {
		secretValue, err = kubernetes.GetSecretValue(ctx, kubeClient, *secretSource.FromKubernetesSecret)
		if err != nil {
			return "", fmt.Errorf("failed to get secret %+v from K8S Secret: %w", *secretSource.FromKubernetesSecret, err)
		}
	} else if secretSource.FromFile != nil {
		secretValue, err = secretSource.FromFile.GetSecretValue()
		if err != nil {
			return "", fmt.Errorf("failed to get secret %+v from file: %w", *secretSource.FromFile, err)
		}
	} else {
		return "", fmt.Errorf("invalid SecretSource: either FromKubernetesSecret or FromFile should be specified: %+v", secretSource)
	}
	return secretValue, nil
}

// GetRepoCloneOptions creates git.CloneOptions for cloning a repo with HTTP, SSH, or TLS credentials from Kubernetes secrets.
func GetRepoCloneOptions(ctx context.Context, repoCred *apiv1.RepoCredential, kubeClient k8sClient.Client, repoUrl string) (*git.CloneOptions, error) {
	endpoint, err := transport.NewEndpoint(repoUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}
	method, skipTls, err := GetAuthMethod(ctx, repoCred, kubeClient, repoUrl)
	if err != nil {
		return nil, err
	}

	cloneOptions := &git.CloneOptions{
		URL:             endpoint.String(),
		Auth:            method,
		InsecureSkipTLS: skipTls,
	}
	return cloneOptions, nil
}

// GetRepoPullOptions creates git.PullOptions for pull updates from a repo with HTTP, SSH, or TLS credentials from Kubernetes secrets.
func GetRepoPullOptions(ctx context.Context, repoCred *apiv1.RepoCredential, kubeClient k8sClient.Client, repoUrl string, refName string) (*git.PullOptions, error) {
	// check to ensure proper repository url is passed
	_, err := transport.NewEndpoint(repoUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}
	method, skipTls, err := GetAuthMethod(ctx, repoCred, kubeClient, repoUrl)
	if err != nil {
		return nil, err
	}
	return &git.PullOptions{
		Force:           true, // for override any local changes
		Auth:            method,
		InsecureSkipTLS: skipTls,
		RemoteName:      "origin",
		ReferenceName:   plumbing.NewBranchReferenceName(refName),
	}, nil
}

// GetRepoFetchOptions creates git.FetchOptions for fetching updates from a
// repo with HTTP, SSH, or TLS credentials from Kubernetes secrets.
func GetRepoFetchOptions(
	ctx context.Context,
	repoCred *apiv1.RepoCredential,
	kubeClient k8sClient.Client,
	repoUrl string,
) (*git.FetchOptions, error) {

	method, skipTls, err := GetAuthMethod(ctx, repoCred, kubeClient, repoUrl)
	if err != nil {
		return nil, err
	}

	return &git.FetchOptions{
		RefSpecs:        []config.RefSpec{"refs/*:refs/*"},
		Force:           true,
		Auth:            method,
		InsecureSkipTLS: skipTls,
	}, nil
}

// FindCredByUrl searches for GitCredential by the specified URL within the provided GlobalConfig.
// It returns the matching GitCredential if the specified URL starts with the URL of any RepoCredentials, otherwise returns nil.
func FindCredByUrl(url string, config controllerConfig.GlobalConfig) *apiv1.RepoCredential {
	normalizedUrl := NormalizeGitUrl(url)
	for _, cred := range config.RepoCredentials {
		if strings.HasPrefix(normalizedUrl, NormalizeGitUrl(cred.URL)) {
			return &cred
		}
	}
	return nil
}

// NormalizeGitUrl function removes the protocol part and any user info from the URLs
func NormalizeGitUrl(gitUrl string) string {
	parsedUrl, err := Parse(gitUrl)
	if err != nil {
		return gitUrl
	}
	normalizedUrl := fmt.Sprintf("%s/%s", parsedUrl.Host, strings.Trim(parsedUrl.Path, "/"))
	normalizedUrl = strings.Trim(normalizedUrl, "/")
	return normalizedUrl
}
