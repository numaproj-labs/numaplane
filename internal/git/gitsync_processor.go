package git

import (
	numaplanenumaprojiov1 "github.com/numaproj-labs/numaplane/api/v1"
)

type GitSyncProcessor struct {
}

func NewGitSyncProcessor(gitSync *numaplanenumaprojiov1.GitSync) (*GitSyncProcessor, error)

func (processor *GitSyncProcessor) Update(gitSync *numaplanenumaprojiov1.GitSync) error

func (processor *GitSyncProcessor) Shutdown() error
