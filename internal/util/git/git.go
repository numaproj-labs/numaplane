package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"

	"github.com/go-git/go-git/v5/plumbing/transport"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	controllerConfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/util/kubernetes"
)

// GetAuthMethod returns an authMethod  for both cloning and fetching from a repo with HTTP, SSH, or TLS credentials from Kubernetes secrets.
func GetAuthMethod(ctx context.Context, repoCred *controllerConfig.RepoCredential, kubeClient k8sClient.Client, repoUrl string) (transport.AuthMethod, bool, error) {
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
				if cred.Username == "" || cred.Password.Name == "" || cred.Password.Key == "" || cred.Password.NameSpace == "" {
					return nil, false, fmt.Errorf("incomplete HTTP credentials")
				}
				secret, err := kubernetes.GetSecret(ctx, kubeClient, cred.Password.NameSpace, cred.Password.Name)
				if err != nil {
					return nil, false, fmt.Errorf("failed to get HTTP credentials secret: %w", err)
				}
				password, ok := secret.Data[cred.Password.Key]
				if !ok {
					return nil, false, fmt.Errorf("password key %s not found in secret %s", cred.Password.Key, cred.Password.Name)
				}
				auth = &gitHttp.BasicAuth{
					Username: cred.Username,
					Password: string(password),
				}
			}

		case "ssh":
			if cred := repoCred.SSHCredential; cred != nil {
				if cred.SSHKey.Name == "" || cred.SSHKey.Key == "" || cred.SSHKey.NameSpace == "" {
					return nil, false, fmt.Errorf("incomplete SSH credentials")
				}
				secret, err := kubernetes.GetSecret(ctx, kubeClient, cred.SSHKey.NameSpace, cred.SSHKey.Name)
				if err != nil {
					return nil, false, fmt.Errorf("failed to get SSH key secret: %w", err)
				}
				sshKey, ok := secret.Data[cred.SSHKey.Key]
				if !ok {
					return nil, false, fmt.Errorf("SSH key %s not found in secret %s", cred.SSHKey.Key, cred.SSHKey.Name)
				}
				parsedUrl, err := Parse(repoUrl)
				if err != nil {
					return nil, false, err
				}
				auth, err = ssh.NewPublicKeys(parsedUrl.User.Username(), sshKey, "")
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

// GetRepoCloneOptions creates git.CloneOptions for cloning a repo with HTTP, SSH, or TLS credentials from Kubernetes secrets.
func GetRepoCloneOptions(ctx context.Context, repoCred *controllerConfig.RepoCredential, kubeClient k8sClient.Client, repoUrl string) (*git.CloneOptions, error) {
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

// GetRepoFetchOptions creates git.FetchOptions for fetching updates from  a repo with HTTP, SSH, or TLS credentials from Kubernetes secrets.
func GetRepoFetchOptions(ctx context.Context, repoCred *controllerConfig.RepoCredential, kubeClient k8sClient.Client, repoUrl string) (*git.FetchOptions, error) {
	// check to ensure proper repository url is passed
	_, err := transport.NewEndpoint(repoUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}
	method, skipTls, err := GetAuthMethod(ctx, repoCred, kubeClient, repoUrl)
	if err != nil {
		return nil, err
	}
	fetchOptions := &git.FetchOptions{
		Auth:            method,
		InsecureSkipTLS: skipTls,
		RefSpecs:        []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
		Force:           true,
	}
	return fetchOptions, nil
}

// FindCredByUrl searches for GitCredential by the specified URL within the provided GlobalConfig.
// It returns the matching GitCredential if the specified URL starts with the URL of any RepoCredentials, otherwise returns nil.
func FindCredByUrl(url string, config controllerConfig.GlobalConfig) *controllerConfig.RepoCredential {
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
