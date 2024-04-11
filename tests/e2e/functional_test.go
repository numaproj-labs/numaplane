//go:build test

/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	. "github.com/numaproj-labs/numaplane/tests/fixtures"
)

type FunctionalSuite struct {
	E2ESuite
}

// GitSync testing for Numaflow with raw manifests
func (s *FunctionalSuite) TestSimpleNumaflowGitSync() {

	// create GitSync and initialize Git repo with initial manifest files
	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("numaflow/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	// verify Numaflow installation and pipeline creation
	w.Expect().ResourcesExist("numaflow.numaproj.io/v1alpha1", "interstepbufferservices", []string{"default"})
	w.Expect().ResourcesExist("numaflow.numaproj.io/v1alpha1", "pipelines", []string{"simple-pipeline"})

	// verify that the CommitStatus of GitSync is currently synced with initial commit
	w.Expect().CheckCommitStatus()

	// pushing new pipeline file to repo
	// wait currently necessary as controller takes time to reconcile and sync properly
	w.PushToGitRepo("numaflow/modified", []string{"http-pipeline.yaml"}, false).Wait(30 * time.Second)

	// verify that new pipeline has been created and GitSync is synced to newest commit
	w.Expect().ResourcesExist("numaflow.numaproj.io/v1alpha1", "pipelines", []string{"http-pipeline"})
	w.Expect().CheckCommitStatus()

	// edit existing pipeline in repo to have additional vertex
	w.PushToGitRepo("numaflow/modified", []string{"sample_pipeline.yaml"}, false).Wait(45 * time.Second)

	// verify that spec of simple-pipeline was updated with new vertex
	w.Expect().ResourcesExist("numaflow.numaproj.io/v1alpha1", "pipelines", []string{"simple-pipeline"})
	// w.Expect().VerifyResourceSpec("numaflow.numaproj.io/v1alpha1", "pipelines", "simple-pipeline", "vertices", "")
	w.Expect().CheckCommitStatus()

	// edit existing pipeline back to initial state
	w.PushToGitRepo("numaflow/initial-commit", []string{"sample_pipeline.yaml"}, false).Wait(30 * time.Second)

	w.Expect().ResourcesExist("numaflow.numaproj.io/v1alpha1", "pipelines", []string{"simple-pipeline"})
	w.Expect().ResourcesDontExist("numaflow.numaproj.io/v1alpha1", "vertices", []string{"simple-pipeline-another-out"})
	w.Expect().ResourcesExist("numaflow.numaproj.io/v1alpha1", "vertices", []string{"simple-pipeline-in", "simple-pipeline-cat", "simple-pipeline-out"})

}

// GitSync testing with basic k8s objects
func (s *FunctionalSuite) TestBasicGitSync() {

	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"nginx-deployment"})
	w.Expect().CheckCommitStatus()

	w.PushToGitRepo("basic-resources/modified", []string{"test.yaml"}, false).Wait(30 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"nginx-deployment"})
	w.Expect().CheckCommitStatus()

	w.Expect().VerifyResourceSpec("apps/v1", "deployments", "nginx-deployment", "replicas", 5)

}

// GitSync self healing occurs when resource does not match manifest in repo
func (s *FunctionalSuite) TestSelfHealing() {

	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"nginx-deployment"})
	w.Expect().CheckCommitStatus()

	w.Expect().VerifyResourceSpec("apps/v1", "deployments", "nginx-deployment", "replicas", 3)

	// apply patch to resource
	w.ModifyResource("apps/v1", "deployments", "nginx-deployment", `{"spec":{"replicas":4}}`).Wait(30 * time.Second)

	w.Expect().VerifyResourceSpec("apps/v1", "deployments", "nginx-deployment", "replicas", 3)

}

func TestFunctionalSuite(t *testing.T) {
	suite.Run(t, new(FunctionalSuite))
}
