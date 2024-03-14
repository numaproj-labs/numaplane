package git

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
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

// GitUrlParser converts a string into a URL.
type GitUrlParser func(string) (*url.URL, error)

// Parse parses rawurl into a URL structure. Parse first attempts to
// find a standard URL with a valid Git transport as its scheme. If
// that cannot be found, it then attempts to find a SCP-like URL. And
// if that cannot be found, it assumes rawurl is a local path. If none
// of these rules apply, Parse returns an error.
func Parse(rawurl string) (u *url.URL, err error) {
	parsers := []GitUrlParser{
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

func GetURLScheme(rawUrl string) (string, error) {
	parsedUrl, err := Parse(rawUrl)
	if err != nil {
		return "", err
	}
	scheme := parsedUrl.Scheme
	return scheme, nil
}
