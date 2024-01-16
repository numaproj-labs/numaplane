package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	repoURl    = "https://github.com/numaproj/numaflow-go"
	tag        = "v0.6.0"
	commitHash = "2903e2744139336ef2d3e3fb92662961c7e36f54"
)

func TestCloneRepository(t *testing.T) {
	repository, err := cloneRepository(repoURl)
	assert.Nil(t, err)
	assert.NotNil(t, repository)
}

func TestCheckRevision_ExpectedBranch(t *testing.T) {
	repository, err := cloneRepository(repoURl)
	assert.Nil(t, err)
	revision, err := checkRevision(repository, "main")
	assert.Nil(t, err)
	assert.Equal(t, "branch", revision)
}

func TestCheckRevision_ExpectedTag(t *testing.T) {
	repository, err := cloneRepository(repoURl)
	assert.Nil(t, err)
	revision, err := checkRevision(repository, tag)
	assert.Nil(t, err)
	assert.Equal(t, "tag", revision)
}
