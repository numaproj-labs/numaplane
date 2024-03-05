package kubernetes

import (
	"regexp"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

func IsValidKubernetesNamespace(name string) bool {
	// All namespace names must be valid RFC 1123 DNS labels.
	errs := validation.IsDNS1123Label(name)
	reservedNamesRegex := regexp.MustCompile(`^(kubernetes-|kube-)`)
	if len(errs) == 0 && !reservedNamesRegex.MatchString(name) {
		return true
	}
	return false
}

func IsValidKubernetesManifestFile(fileName string) bool {
	fileExt := strings.Split(fileName, ".")
	validExtName := []string{"yaml", "yml", "json"}
	return slices.Contains(validExtName, fileExt[1])
}
