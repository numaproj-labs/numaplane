package git

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	numaplanenumaprojiov1 "github.com/numaproj-labs/numaplane/api/v1"
)

type GitSyncProcessor struct {
}

func NewGitSyncProcessor(gitSync *numaplanenumaprojiov1.GitSync, k8sClient client.Client) (*GitSyncProcessor, error) {
	return &GitSyncProcessor{}, nil
}

func (processor *GitSyncProcessor) Update(gitSync *numaplanenumaprojiov1.GitSync) error {
	return nil
}

func (processor *GitSyncProcessor) Shutdown() error {
	return nil
}
