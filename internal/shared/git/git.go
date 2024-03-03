package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	controllerConfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/shared/kubernetes"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// GetRepoCloneOptions creates git.CloneOptions for cloning a repo with HTTP, SSH, or TLS credentials from Kubernetes secrets.
func GetRepoCloneOptions(ctx context.Context, repoCred *controllerConfig.RepoCredential, kubeClient k8sClient.Client, namespace string, repoUrl string) (*git.CloneOptions, error) {
	endpoint, err := transport.NewEndpoint(repoUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}
	scheme, err := GetURLScheme(repoUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL scheme: %w", err)
	}

	cloneOptions := &git.CloneOptions{
		URL: endpoint.String(),
	}

	// Assuming the CRD git url is public url
	if repoCred == nil {
		return cloneOptions, nil
	}

	// Configure TLS if applicable
	if repoCred.TLS != nil {
		cloneOptions.InsecureSkipTLS = repoCred.TLS.InsecureSkipVerify
	}

	switch scheme {
	case "http", "https":
		if cred := repoCred.HTTPCredential; cred != nil {
			if cred.Username == "" || cred.Password.Name == "" || cred.Password.Key == "" {
				return nil, fmt.Errorf("incomplete HTTP credentials")
			}
			secret, err := kubernetes.GetSecret(ctx, kubeClient, namespace, cred.Password.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to get HTTP credentials secret: %w", err)
			}
			password, ok := secret.Data[cred.Password.Key]
			if !ok {
				return nil, fmt.Errorf("password key %s not found in secret %s", cred.Password.Key, cred.Password.Name)
			}
			cloneOptions.Auth = &gitHttp.BasicAuth{
				Username: cred.Username,
				Password: string(password),
			}
		}

	case "ssh":
		if cred := repoCred.SSHCredential; cred != nil {
			if cred.SSHKey.Name == "" || cred.SSHKey.Key == "" {
				return nil, fmt.Errorf("incomplete SSH credentials")
			}
			secret, err := kubernetes.GetSecret(ctx, kubeClient, namespace, cred.SSHKey.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to get SSH key secret: %w", err)
			}
			sshKey, ok := secret.Data[cred.SSHKey.Key]
			if !ok {
				return nil, fmt.Errorf("SSH key %s not found in secret %s", cred.SSHKey.Key, cred.SSHKey.Name)
			}
			// this is important as for only git urls [git@github] git will be the username for others we need to identify
			parsedUrl, err := Parse(repoUrl)
			if err != nil {
				return nil, err
			}
			authMethod, err := ssh.NewPublicKeys(parsedUrl.User.Username(), sshKey, "")
			if err != nil {
				return nil, fmt.Errorf("failed to create SSH public keys: %w", err)
			}
			cloneOptions.Auth = authMethod
		}
		// TODO : should we support ftp ?
	default:
		return nil, fmt.Errorf("unsupported URL scheme: %s", scheme)
	}

	return cloneOptions, nil
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
