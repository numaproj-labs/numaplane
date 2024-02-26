package git

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"

	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	controllerconfig "github.com/numaproj-labs/numaplane/internal/controller/config"
	"github.com/numaproj-labs/numaplane/internal/kubernetes"
)

// Valid git transports url
// Source -https://pkg.go.dev/github.com/whilp/git-urls@v1.0.0
var (
	// scpSyntax was modified from https://golang.org/src/cmd/go/vcs.go.
	scpSyntax  = regexp.MustCompile(`^([a-zA-Z0-9-._~]+@)?([a-zA-Z0-9._-]+):([a-zA-Z0-9./._-]+)(?:\?||$)(.*)$`)
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

// NewTransportSet returns a TransportSet with the items' keys mapped
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
	return true
}

// Parser converts a string into a URL.
type Parser func(string) (*url.URL, error)

// Parse parses rawurl into a URL structure. Parse first attempts to
// find a standard URL with a valid Git transport as its scheme. If
// that cannot be found, it then attempts to find a SCP-like URL. And
// if that cannot be found, it assumes rawurl is a local path. If none
// of these rules apply, Parse returns an error.
func Parse(rawurl string) (u *url.URL, err error) {
	parsers := []Parser{
		ParseTransport,
		ParseScp,
		ParseLocal,
	}

	// Apply each parser in turn; if the parser succeeds, accept its
	// result and return.
	for _, p := range parsers {
		u, err = p(rawurl)
		if err == nil {
			return u, err
		}
	}

	// It's unlikely that none of the parsers will succeed, since
	// ParseLocal is very forgiving.
	return new(url.URL), fmt.Errorf("failed to parse %q", rawurl)
}

// ParseTransport parses rawurl into a URL object. Unless the URL's
// scheme is a known Git transport, ParseTransport returns an error.
func ParseTransport(rawurl string) (*url.URL, error) {
	u, err := url.Parse(rawurl)
	if err == nil && !Transports.Valid(u.Scheme) {
		err = fmt.Errorf("scheme %q is not a valid transport", u.Scheme)
	}
	return u, err
}

// ParseScp parses rawurl into a URL object. The rawurl must be
// an SCP-like URL, otherwise ParseScp returns an error.
func ParseScp(rawurl string) (*url.URL, error) {
	match := scpSyntax.FindAllStringSubmatch(rawurl, -1)
	if len(match) == 0 {
		return nil, fmt.Errorf("no scp URL found in %q", rawurl)
	}
	m := match[0]
	user := strings.TrimRight(m[1], "@")
	var userinfo *url.Userinfo
	if user != "" {
		userinfo = url.User(user)
	}
	rawquery := ""
	if len(m) > 3 {
		rawquery = m[4]
	}
	return &url.URL{
		Scheme:   "ssh",
		User:     userinfo,
		Host:     m[2],
		Path:     m[3],
		RawQuery: rawquery,
	}, nil
}

// ParseLocal parses rawurl into a URL object with a "file"
// scheme. This will effectively never return an error.
func ParseLocal(rawurl string) (*url.URL, error) {
	return &url.URL{
		Scheme: "file",
		Host:   "",
		Path:   rawurl,
	}, nil
}

// GetRepoCloneOptions creates git.CloneOptions for cloning a repo with HTTP, SSH, or TLS credentials from Kubernetes secrets.
func GetRepoCloneOptions(ctx context.Context, repoCred *controllerconfig.RepoCredential, kubeClient kubernetes.Client, namespace string, repo *v1alpha1.RepositoryPath) (*git.CloneOptions, error) {
	endpoint, err := transport.NewEndpoint(repo.RepoUrl)
	if err != nil {
		return nil, err
	}
	var cloneOptions *git.CloneOptions
	switch {
	case repoCred.TLS != nil && repoCred.HTTPCredential != nil:
		cred := repoCred.HTTPCredential
		username := cred.Username
		secret, err := kubeClient.GetSecret(ctx, namespace, cred.Password.Name)
		if err != nil {
			return nil, err
		}
		password := string(secret.Data[cred.Password.Key])
		authMethod := &gitHttp.BasicAuth{
			Username: username,
			Password: password,
		}
		cloneOptions = &git.CloneOptions{
			URL:             endpoint.String(),
			Auth:            authMethod,
			InsecureSkipTLS: repoCred.TLS.InsecureSkipVerify,
		}
		return cloneOptions, nil

	case repoCred.HTTPCredential != nil:
		cred := repoCred.HTTPCredential
		username := cred.Username
		secret, err := kubeClient.GetSecret(ctx, namespace, cred.Password.Name)
		if err != nil {
			return nil, err
		}
		password := string(secret.Data[cred.Password.Key])
		authMethod := &gitHttp.BasicAuth{
			Username: username,
			Password: password,
		}
		cloneOptions = &git.CloneOptions{
			URL:  endpoint.String(),
			Auth: authMethod,
		}
		return cloneOptions, nil

	case repoCred.SSHCredential != nil:
		secret, err := kubeClient.GetSecret(ctx, namespace, repoCred.SSHCredential.SSHKey.Name)
		if err != nil {
			return nil, err
		}
		// Parsing url is necessary to get the username from the url
		parsedUrl, err := Parse(repo.RepoUrl)
		if err != nil {
			return nil, err
		}
		authMethod, err := ssh.NewPublicKeys(parsedUrl.User.Username(), secret.Data[repoCred.SSHCredential.SSHKey.Key], "")
		if err != nil {
			return nil, err
		}

		cloneOptions = &git.CloneOptions{
			URL:  endpoint.String(),
			Auth: authMethod,
		}
		return cloneOptions, nil

	default:
		return nil, nil
	}

}

// FindCredByUrl searches for GitCredential by the specified URL within the provided GlobalConfig.
// It returns the matching GitCredential if the specified URL starts with the URL of any RepoCredentials, otherwise returns nil.
func FindCredByUrl(url string, config controllerconfig.GlobalConfig) *controllerconfig.RepoCredential {
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
