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

	// remove pipeline manifest from repo
	w.PushToGitRepo("numaflow/modified", []string{"http-pipeline.yaml"}, true).Wait(30 * time.Second)

	// verify that the pipeline has been deleted by GitSync and synced to current commit
	w.Expect().ResourcesDontExist("numaflow.numaproj.io/v1alpha1", "pipelines", []string{"http-pipeline"})
	w.Expect().CheckCommitStatus()

}

// GitSync testing with basic k8s objects
func (s *FunctionalSuite) TestBasicGitSync() {

	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	// verify basics resources are created
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"test-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"test-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"test-secret"})
	w.Expect().CheckCommitStatus()

	// update deployment manifest with {replicas: 5}
	w.PushToGitRepo("basic-resources/modified", []string{"deployment.yaml"}, false).Wait(30 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"test-deploy"})
	w.Expect().CheckCommitStatus()

	// verify that resource has received changed
	w.Expect().VerifyResourceState("apps/v1", "deployments", "test-deploy", "spec", "replicas", 5)

	w.Expect().VerifyResourceState("v1", "configmaps", "test-config", "data", "clusterName", "staging-usw2-k8s")

}

// GitSync self healing occurs when resource does not match manifest in repo
func (s *FunctionalSuite) TestSelfHealing() {

	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	// verify that test deployment is created with {replicas: 3} in spec
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"test-deploy"})
	w.Expect().CheckCommitStatus()

	w.Expect().VerifyResourceState("apps/v1", "deployments", "test-deploy", "spec", "replicas", 3)

	// apply patch to resource
	w.ModifyResource("apps/v1", "deployments", "test-deploy", `{"spec":{"replicas":4}}`).Wait(30 * time.Second)

	// verify that resource has been "healed" by resetting replica count to 3
	w.Expect().VerifyResourceState("apps/v1", "deployments", "test-deploy", "spec", "replicas", 3)

}

func (s *FunctionalSuite) TestChangeRepoUrl() {

	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"test-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"test-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"test-secret"})
	w.Expect().CheckCommitStatus()

	w = s.Given().GitSync("@testdata/revised-gitsync.yaml").InitializeGitRepo("basic-resources/second-repo").
		When().
		UpdateGitSyncAndWait()

	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"repo-two-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"repo-two-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"repo-two-secret"})
	w.Expect().CheckCommitStatus()

}

func TestFunctionalSuite(t *testing.T) {
	suite.Run(t, new(FunctionalSuite))
}
