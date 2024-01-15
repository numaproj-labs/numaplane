package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	repoURl = "https://github.com/numaproj/numaflow-go"
	tag     = "v0.5.2"
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
