package validations

import (
	"fmt"
	"regexp"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
)

// CheckGitURL checks whether the given git url is a valid git url or not by fetching remote references
// TODO: for private repository
func CheckGitURL(gitURL string) bool {
	rem := gogit.NewRemote(nil, &config.RemoteConfig{
		Name: "origin",
		URLs: []string{gitURL},
	})

	// running git ls-remote
	_, err := rem.List(&gogit.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing references: %s\n", err)
		return false
	}
	return true
}

func IsValidName(name string) bool {
	// names can only contain lowercase alphanumeric characters, '-', and '.', but must start and end with an alphanumeric
	validNameRegex := regexp.MustCompile(`^[a-z0-9][a-z0-9\-.]*[a-z0-9]$`)

	//cant contain names starting with kubernetes-, kube-, as these are reserved names
	reservedNamesRegex := regexp.MustCompile(`^(kubernetes-|kube-)`)

	return validNameRegex.MatchString(name) && !reservedNamesRegex.MatchString(name)
}
