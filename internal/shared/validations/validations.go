package validations

import (
	"fmt"
	"net/url"
	"regexp"

	"k8s.io/apimachinery/pkg/util/validation"
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
	return true
}
func GetTransportScheme(gitURL string) (string, error) {
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
