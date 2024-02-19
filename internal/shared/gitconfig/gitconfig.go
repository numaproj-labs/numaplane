package gitconfig

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"regexp"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
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

type TransportSet struct {
	Transports map[string]struct{}
}

// NewTransportSet returns a TransportSet with the items keys mapped
// to empty struct values.
func NewTransportSet(items ...string) *TransportSet {
	t := &TransportSet{
		Transports: map[string]struct{}{},
	}
	for _, i := range items {
		t.Transports[i] = struct{}{}
	}
	return t
}

// Valid returns true if transport is a known Git URL scheme and false
// if not.
func (t *TransportSet) Valid(transport string) bool {
	_, ok := t.Transports[transport]
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
func GetTransportScheme(gitURL string) (string, error) {

	if matched, _ := regexp.MatchString(`^[a-zA-Z]+@[\w\.]+:[\w\/\-\.~]+`, gitURL); matched {
		return "ssh", nil
	}
	u, err := url.Parse(gitURL)
	if err != nil {
		return u.Scheme, fmt.Errorf("invalid git url %s", err.Error())
	}
	return u.Scheme, err
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

func GetAuthMethod(ctx context.Context, repoUrl string, kubeClient kubernetes.Client, namespace string, repoCred *controllerconfig.GitCredential) (transport.AuthMethod, error) {
	scheme, err := GetTransportScheme(repoUrl)
	if err != nil {
		return nil, err
	}
	var authMethod transport.AuthMethod
	switch scheme {
	case "ssh", "git+ssh":
		key := repoCred.SSHCredential.SSHKey.Key

		secret, err := kubeClient.GetSecret(ctx, namespace, key)
		if err != nil {
			return nil, err
		}
		authMethod, err = ssh.NewPublicKeys("git", secret.Data["sshKey"], "")
		if err != nil {
			return nil, err
		}

	case "http", "https":
		if repoCred.HTTPCredential != nil {
			cred := repoCred.HTTPCredential
			username := cred.Username
			secret, err := kubeClient.GetSecret(ctx, namespace, cred.Password.Key)
			if err != nil {
				return nil, err
			}
			password := string(secret.Data["password"])
			authMethod = &http.BasicAuth{
				Username: username,
				Password: password,
			}
		}
	case "git", "file":
		authMethod = nil

	default:
		return nil, fmt.Errorf("unsupported or not required authentication")
	}

	return authMethod, nil
}
