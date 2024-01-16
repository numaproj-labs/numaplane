package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	repoURl    = "https://github.com/numaproj/numaflow-go"
	tag        = "v0.5.2"
	commitHash = "2903e2744139336ef2d3e3fb92662961c7e36f54"
)

func TestCloneRepositoryForBranch(t *testing.T) {
	repository, err := cloneRepository(repoURl, "protof")
	assert.Nil(t, err)
	assert.NotNil(t, repository)
}

func TestCloneRepositoryForTag(t *testing.T) {
	repository, err := cloneRepository(repoURl, tag)
	assert.Nil(t, err)
	assert.NotNil(t, repository)
}

func TestCloneRepositoryForCommitHash(t *testing.T) {
	repository, err := cloneRepository(repoURl, commitHash)
	assert.Nil(t, err)
	assert.NotNil(t, repository)
}
