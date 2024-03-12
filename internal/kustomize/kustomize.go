package kustomize

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	numaExec "github.com/numaproj-labs/numaplane/internal/util/exec"
)

// TODO: This file is copied from argo-cd (https://github.com/argoproj/argo-cd/blob/master/util/kustomize/kustomize.go)
// Due to version conflict issue with client-go package with kubernetes/patcher.go changes. Once we move to gitops-engine
// then we will be able to use this code directly from argo-cd.

// Kustomize provides wrapper functionality around the `kustomize` command.
type Kustomize interface {
	// Build returns a list of unstructured objects from a `kustomize build` command and extract supported parameters
	Build(kustomizeOptions *KustomizeOptions) (string, error)
}

type kustomize struct {
	// path inside the checked out tree
	path string
	// the Git repository URL where we checked out
	repo string
	// optional kustomize binary path
	binaryPath string
}

// KustomizeOptions are options for kustomize to use when building manifests
type KustomizeOptions struct {
	// BuildOptions is a string of build parameters to use when calling `kustomize build`
	BuildOptions string `protobuf:"bytes,1,opt,name=buildOptions"`
	// BinaryPath holds an optional path to kustomize binary
	BinaryPath string `protobuf:"bytes,2,opt,name=binaryPath"`
}

// NewKustomizeApp create a new wrapper to run commands on the `kustomize` command-line tool.
func NewKustomizeApp(path string, fromRepo string, binaryPath string) Kustomize {
	return &kustomize{
		path:       path,
		repo:       fromRepo,
		binaryPath: binaryPath,
	}
}

var _ Kustomize = &kustomize{}

func (k *kustomize) getBinaryPath() string {
	if k.binaryPath != "" {
		return k.binaryPath
	}
	return "kustomize"
}

func (k *kustomize) Build(kustomizeOptions *KustomizeOptions) (string, error) {
	var cmd *exec.Cmd
	if kustomizeOptions != nil && kustomizeOptions.BuildOptions != "" {
		params := parseKustomizeBuildOptions(k.path, kustomizeOptions.BuildOptions)
		cmd = exec.Command(k.getBinaryPath(), params...)
	} else {
		cmd = exec.Command(k.getBinaryPath(), "build", k.path)
	}

	out, err := numaExec.Run(cmd)
	if err != nil {
		return "", err
	}

	return out, nil
}

func parseKustomizeBuildOptions(path, buildOptions string) []string {
	return append([]string{"build", path}, strings.Split(buildOptions, " ")...)
}

var KustomizationNames = []string{"kustomization.yaml", "kustomization.yml", "Kustomization"}

func IsKustomizationRepository(path string) bool {
	for _, file := range KustomizationNames {
		kustomization := filepath.Join(path, file)
		if _, err := os.Stat(kustomization); err == nil {
			return true
		}
	}

	return false
}
