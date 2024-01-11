package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	repoURl    = "https://github.com/numaproj/numaflow-go"
	commitHash = "5424b351d679bc894d1666aac1d8778443314ffc"
	tag        = "v0.5.2"
)

func TestCloneRepositoryForBranch(t *testing.T) {
	repository, err := cloneRepository(repoURl, "main")
	assert.Nil(t, err)
	assert.NotNil(t, repository)
}

func TestCloneRepositoryForCommitHash(t *testing.T) {
	repository, err := cloneRepository(repoURl, commitHash)
	assert.Nil(t, err)
	assert.NotNil(t, repository)
}

func TestCloneRepositoryForTag(t *testing.T) {
	repository, err := cloneRepository(repoURl, tag)
	assert.Nil(t, err)
	assert.NotNil(t, repository)
}
