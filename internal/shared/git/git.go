package git

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"regexp"

	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"k8s.io/apimachinery/pkg/util/validation"

	controllerconfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/kubernetes"
)

// Valid git transports url

var (
	Transports = NewTransportSet(
		"ssh",
		"git",
		"git+ssh",
		"http",
		"https",
		"ftp",
		"ftps",
		"rsync",
		"file",
	)
)

// Checks for valid remote repository

type TransportSet map[string]struct{}

// NewTransportSet returns a TransportSet with the items keys mapped
// to empty struct values.
func NewTransportSet(items ...string) TransportSet {
	t := make(TransportSet)
	for _, item := range items {
		t[item] = struct{}{}
	}
	return t
}

// Valid returns true if transport is a known Git URL scheme and false
// if not.
func (t TransportSet) Valid(transport string) bool {
	_, ok := t[transport] // Directly checking the key in the map
	return ok
}

func CheckGitURL(gitURL string) bool {
	u, err := url.Parse(gitURL)
	if err == nil && !Transports.Valid(u.Scheme) {
		return false
	}
	log.Println(u)
	return true
}

func IsValidKubernetesNamespace(name string) bool {
	// All namespace names must be valid RFC 1123 DNS labels.
	errs := validation.IsDNS1123Label(name)
	reservedNamesRegex := regexp.MustCompile(`^(kubernetes-|kube-)`)
	if len(errs) == 0 && !reservedNamesRegex.MatchString(name) {
		return true
	}
	return false
}

func GetAuthMethod(ctx context.Context, repoCred *controllerconfig.GitCredential, kubeClient kubernetes.Client, namespace string) (transport.AuthMethod, error) {
	var authMethod transport.AuthMethod
	switch {
	case repoCred.HTTPCredential != nil:
		cred := repoCred.HTTPCredential
		username := cred.Username
		secret, err := kubeClient.GetSecret(ctx, namespace, cred.Password.Key)
		if err != nil {
			return nil, err
		}
		password := string(secret.Data["password"])
		authMethod = &gitHttp.BasicAuth{
			Username: username,
			Password: password,
		}

	case repoCred.SSHCredential != nil:
		key := repoCred.SSHCredential.SSHKey.Key
		secret, err := kubeClient.GetSecret(ctx, namespace, key)
		if err != nil {
			return nil, err
		}
		authMethod, err = ssh.NewPublicKeys("git", secret.Data["sshKey"], "")
		if err != nil {
			return nil, err
		}

	case repoCred.TLS != nil:
		// TODO :this needs to be implemented

	default:
		return nil, fmt.Errorf("unsupported or not required authentication")
	}
	return authMethod, nil
}

// FindCredByUrl searches for GitCredential by the specified URL within the provided GlobalConfig.
// It returns the matching GitCredential if found, otherwise returns nil.
func FindCredByUrl(url string, config controllerconfig.GlobalConfig) *controllerconfig.GitCredential {
	for _, cred := range config.RepoCredentials {
		if cred.URL == url {
			return cred.Credential
		}
	}
	return nil
}
