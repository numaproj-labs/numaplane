package exec

import (
	"os/exec"
	"time"

	argoexec "github.com/argoproj/pkg/exec"
)

const (
	defaultTimeout = 10 * time.Second
)

func Run(cmd *exec.Cmd) (string, error) {
	cmdOpts := argoexec.CmdOpts{Timeout: defaultTimeout}

	return argoexec.RunCommandExt(cmd, cmdOpts)
}
