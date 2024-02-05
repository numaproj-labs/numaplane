package validations

import (
	"context"
	"net/url"
	"regexp"
	"unicode/utf8"
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

func CheckGitURL(gitURL string, ctx context.Context) bool {
	u, err := url.Parse(gitURL)
	if err == nil && !Transports.Valid(u.Scheme) {
		return false
	}
	return true
}

func IsValidName(name string) bool {
	if utf8.RuneCountInString(name) > 253 { // to avoid special characters length
		return false
	}
	// names can only contain lowercase alphanumeric characters, '-', and '.', but must start and end with an alphanumeric
	validNameRegex := regexp.MustCompile(`^[a-z0-9][a-z0-9\-.]*[a-z0-9]$`)
	return validNameRegex.MatchString(name)
}

func IsReservedName(name string) bool {
	reservedNamesRegex := regexp.MustCompile(`^(kubernetes-|kube-)`)
	return reservedNamesRegex.MatchString(name)

}
