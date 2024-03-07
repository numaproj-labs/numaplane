package helm

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/numaproj-labs/numaplane/api/v1alpha1"
	numaExec "github.com/numaproj-labs/numaplane/internal/shared/exec"
)

var (
	re = regexp.MustCompile(`([^\\]),`)
)

type helm struct {
	binaryPath           string
	chartPath            string
	templateNameArg      string
	showCommand          string
	pullCommand          string
	kubeVersionSupported bool
}

func New(chartPath, binaryPath string) Helm {
	return &helm{
		binaryPath:           binaryPath,
		chartPath:            chartPath,
		templateNameArg:      "--name-template",
		kubeVersionSupported: true,
		showCommand:          "show",
		pullCommand:          "pull",
	}
}

// Helm provides wrapper functionality around the `helm` command.
type Helm interface {
	// Build returns a list of yaml string from a `helm template` command
	Build(string, string, []v1alpha1.HelmParameter, []string) (string, error)
}

func (h *helm) getBinaryPath() string {
	if h.binaryPath != "" {
		return h.binaryPath
	}
	return "helm"
}

func (h *helm) Build(name, namespace string, parameters []v1alpha1.HelmParameter, values []string) (string, error) {
	args := []string{"template", h.chartPath, h.templateNameArg, name}

	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	for _, parameter := range parameters {
		args = append(args, "--set", parameter.Name+"="+cleanSetParameters(parameter.Value))
	}
	for _, val := range values {
		args = append(args, "--values", val)
	}

	cmd := exec.Command(h.getBinaryPath(), args...)

	out, err := numaExec.Run(cmd)
	if err != nil {
		return "", err
	}

	return out, nil
}

func cleanSetParameters(val string) string {
	// `{}` equal helm list parameters format, so don't escape `,`.
	if strings.HasPrefix(val, `{`) && strings.HasSuffix(val, `}`) {
		return val
	}
	return re.ReplaceAllString(val, `$1\,`)
}
