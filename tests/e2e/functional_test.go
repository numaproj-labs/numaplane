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

var (
	initialEdges = []interface{}{
		map[interface{}]interface{}{"from": "in", "to": "cat"},
		map[interface{}]interface{}{"from": "cat", "to": "out"},
	}
	modifiedEdges = []interface{}{
		map[interface{}]interface{}{"from": "in", "to": "cat"},
		map[interface{}]interface{}{"from": "cat", "to": "out"},
		map[interface{}]interface{}{"from": "cat", "to": "another-out"},
	}
)

// GitSync testing for Numaflow with raw manifests
func (s *FunctionalSuite) TestNumaflowGitSync() {

	// create GitSync and initialize Git repo with initial manifest files
	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("numaflow/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	// verify ISBSVC and pipeline creation
	w.Expect().ResourcesExist("numaflow.numaproj.io/v1alpha1", "interstepbufferservices", []string{"default"})
	w.Expect().ResourcesExist("numaflow.numaproj.io/v1alpha1", "pipelines", []string{"simple-pipeline"})

	// verify that the CommitStatus of GitSync is currently synced with initial commit
	w.Expect().CheckCommitStatus()

	// pushing new pipeline file to repo
	// wait is necessary as controller takes time to reconcile and sync properly
	w.PushToGitRepo("numaflow/modified", []string{"http-pipeline.yaml"}, false).Wait(30 * time.Second)

	// verify that new pipeline has been created and GitSync is synced to newest commit
	w.Expect().ResourcesExist("numaflow.numaproj.io/v1alpha1", "pipelines", []string{"http-pipeline"})
	w.Expect().CheckCommitStatus()

	// edit existing pipeline in repo to have additional vertex and edge
	w.PushToGitRepo("numaflow/modified", []string{"sample_pipeline.yaml"}, false).Wait(30 * time.Second)

	// verify that spec of simple-pipeline was updated with new edge
	w.Expect().ResourcesExist("numaflow.numaproj.io/v1alpha1", "pipelines", []string{"simple-pipeline"})
	w.Expect().VerifyResourceState("numaflow.numaproj.io/v1alpha1", "pipelines", "simple-pipeline", "spec", "edges", modifiedEdges)
	w.Expect().CheckCommitStatus()

	// edit existing pipeline back to initial state
	w.PushToGitRepo("numaflow/initial-commit", []string{"sample_pipeline.yaml"}, false).Wait(30 * time.Second)

	w.Expect().ResourcesExist("numaflow.numaproj.io/v1alpha1", "pipelines", []string{"simple-pipeline"})
	w.Expect().VerifyResourceState("numaflow.numaproj.io/v1alpha1", "pipelines", "simple-pipeline", "spec", "edge", initialEdges)

	// remove pipeline manifest from repo
	w.PushToGitRepo("numaflow/modified", []string{"http-pipeline.yaml"}, true).Wait(45 * time.Second)

	// verify that the pipeline has been deleted by GitSync and synced to current commit
	w.Expect().ResourcesDontExist("numaflow.numaproj.io/v1alpha1", "pipelines", []string{"http-pipeline"})
	w.Expect().CheckCommitStatus()

	// Remove Pipeline and ISBService
	w.PushToGitRepo("numaflow/initial-commit", []string{"sample_pipeline.yaml", "isbsvc_jetstream.yaml"}, true).Wait(30 * time.Second)
	w.Expect().ResourcesDontExist("numaflow.numaproj.io/v1alpha1", "pipelines", []string{"simple-pipeline"})
	w.Expect().ResourcesDontExist("numaflow.numaproj.io/v1alpha1", "interstepbufferservices", []string{"default"})
	w.Expect().CheckCommitStatus()

}

// GitSync testing with basic k8s objects
func (s *FunctionalSuite) TestBasicGitSync() {

	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	s.testBasicGitSync(w)

}

// GitSync testing with basic k8s objects using a public repo
func (s *FunctionalSuite) TestPublicRepo() {

	w := s.Given().GitSync("@testdata/public-gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	s.testBasicGitSync(w)

}

// GitSync testing using credentials from file mounted to deployment instead of k8s secret
func (s *FunctionalSuite) TestGitCredentialFromYAMLFile() {

	// initialize repo and update config to use fromFile
	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		UpdateRepoCredentialConfig("manifests/from-yaml-config.yaml").
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	s.testBasicGitSync(w)

	w.UpdateRepoCredentialConfig("manifests/config.yaml")

}

func (s *FunctionalSuite) TestGitCredentialFromJSONFile() {

	// initialize repo and update config to use fromFile
	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		UpdateRepoCredentialConfig("manifests/from-json-config.yaml").
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	s.testBasicGitSync(w)

	w.UpdateRepoCredentialConfig("manifests/config.yaml")

}

// GitSync auto healing occurs when resource does not match manifest in repo
func (s *FunctionalSuite) TestAutoHealing() {

	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	w.Wait(30 * time.Second)

	// verify that test deployment is created with {replicas: 3} in spec
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"test-deploy"})
	w.Expect().CheckCommitStatus()

	w.Expect().VerifyResourceState("apps/v1", "deployments", "test-deploy", "spec", "replicas", 3)

	// apply patch to resource
	w.ModifyResource("apps/v1", "deployments", "test-deploy", `{"spec":{"replicas":4}}`).Wait(30 * time.Second)

	// verify that resource has been "healed" by resetting replica count to 3
	w.Expect().VerifyResourceState("apps/v1", "deployments", "test-deploy", "spec", "replicas", 3)

	// disable autohealing
	w.UpdateAutoHealConfig(false).Wait(60 * time.Second)

	// apply patch to resource
	w.ModifyResource("apps/v1", "deployments", "test-deploy", `{"spec":{"replicas":4}}`).Wait(30 * time.Second)

	// verify that resource has not been healed like previous
	w.Expect().VerifyResourceState("apps/v1", "deployments", "test-deploy", "spec", "replicas", 4)

	// reenable autohealing for test cases left to run
	w.UpdateAutoHealConfig(true)
}

// test behavior when changing the repoUrl of a GitSync
func (s *FunctionalSuite) TestChangeRepoUrl() {

	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	// first repo's manifests should be applied
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"test-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"test-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"test-secret"})
	w.Expect().CheckCommitStatus()

	// changing repoUrl to point to repo2.git
	w = s.Given().GitSync("@testdata/revised-gitsync.yaml").InitializeGitRepo("basic-resources/second-repo").
		When().
		UpdateGitSyncAndWait()

	// verify that resources in new repository are created
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"repo-two-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"repo-two-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"repo-two-secret"})
	w.Expect().CheckCommitStatus()

	// old resources are deleted as their manifests do not exist in new repository
	w.Expect().ResourcesDontExist("apps/v1", "deployments", []string{"test-deploy"})
	w.Expect().ResourcesDontExist("v1", "configmaps", []string{"test-config"})
	w.Expect().ResourcesDontExist("v1", "secrets", []string{"test-secret"})

}

// GitSync testing with kustomize manifests
func (s *FunctionalSuite) TestKustomize() {

	// create repo containing kustomize manifests
	w := s.Given().GitSync("@testdata/kustomize-gitsync.yaml").InitializeGitRepo("kustomize/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	// verify all resources defined in kustomization file are created
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"kustomize-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"kustomize-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"kustomize-secret"})
	w.Expect().CheckCommitStatus()

	// remove deployment resource from kustomization file
	w.PushToGitRepo("kustomize/modified", []string{"kustomization.yaml"}, false).Wait(30 * time.Second)
	// verify that deployment should be deleted while other resources remain
	w.Expect().ResourcesDontExist("apps/v1", "deployments", []string{"kustomize-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"kustomize-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"kustomize-secret"})
	w.Expect().CheckCommitStatus()

}

// GitSync testing with basic helm manifests
func (s *FunctionalSuite) TestHelm() {

	w := s.Given().GitSync("@testdata/helm-gitsync.yaml").InitializeGitRepo("helm/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	// verify all resources defined in helm file are created
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"gitsync-example-helm-test"})
	w.Expect().CheckCommitStatus()

	// remove deployment manifest from repo
	w.PushToGitRepo("helm/initial-commit", []string{"templates/deployment.yaml"}, true).Wait(30 * time.Second)
	w.Expect().ResourcesDontExist("apps/v1", "deployments", []string{"gitsync-example-helm-test"})
	w.Expect().CheckCommitStatus()

	// adding new template to repo
	w.PushToGitRepo("helm/modified", []string{"templates/new-deploy.yaml"}, false).Wait(30 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"new-deploy"})
	w.Expect().CheckCommitStatus()

	// modifying the values.yaml target for helm
	w.PushToGitRepo("helm/modified", []string{"values.yaml"}, false).Wait(30 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"new-deploy"})
	w.Expect().CheckCommitStatus()

	w.Expect().VerifyResourceState("apps/v1", "deployments", "new-deploy", "spec", "replicas", 5)

	// changing the GitSync spec to override values.yaml with ReplicaCount = 3
	s.Given().GitSync("@testdata/helm-gitsync-params.yaml").
		When().
		UpdateGitSyncAndWait().
		Wait(30 * time.Second)

	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"new-deploy"})
	w.Expect().CheckCommitStatus()

	// verify change has occurred
	w.Expect().VerifyResourceState("apps/v1", "deployments", "new-deploy", "spec", "replicas", 3)

}

// test behavior when changing targetRevision of existing GitSync
func (s *FunctionalSuite) TestNonMainBranch() {

	// start at master branch
	w := s.Given().GitSync("@testdata/gitsync.yaml").InitializeGitRepo("basic-resources/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	// verify basics resources are created
	w.Wait(30 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"test-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"test-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"test-secret"})
	// these resources are defined in the same file
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"multi-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"multi-config"})
	w.Expect().CheckCommitStatus()

	// switch to test branch
	w = s.Given().GitSync("@testdata/branch-gitsync.yaml").ChangeBranch().
		When().
		UpdateGitSyncAndWait()

	w.Wait(30 * time.Second)

	// update deployment manifest with {replicas: 5} on test branch
	w.PushToGitRepo("basic-resources/modified", []string{"deployment.yaml"}, false).Wait(30 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"test-deploy"})
	w.Expect().CheckCommitStatus()

	// verify change has occurred
	w.Expect().VerifyResourceState("apps/v1", "deployments", "test-deploy", "spec", "replicas", 5)

	// switch back to master branch
	w = s.Given().GitSync("@testdata/gitsync.yaml").ChangeBranch().
		When().
		UpdateGitSyncAndWait()

	w.Wait(30 * time.Second)

	// verify that resource has been healed back to the master branch state
	w.Expect().VerifyResourceState("apps/v1", "deployments", "test-deploy", "spec", "replicas", 3)

}

func (s *FunctionalSuite) testBasicGitSync(w *When) {

	// verify basics resources are created
	w.Wait(30 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"test-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"test-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"test-secret"})
	// these resources are defined in the same file
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"multi-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"multi-config"})

	w.Expect().CheckCommitStatus()

	// update deployment manifest with {replicas: 5}
	w.PushToGitRepo("basic-resources/modified", []string{"deployment.yaml"}, false).Wait(45 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"test-deploy"})
	w.Expect().CheckCommitStatus()

	// verify that resource has received changed and others have not been modified
	w.Expect().VerifyResourceState("apps/v1", "deployments", "test-deploy", "spec", "replicas", 5)
	w.Expect().VerifyResourceState("v1", "configmaps", "test-config", "data", "clusterName", "staging-usw2-k8s")

	// add another resource to multiple-resources file
	w.PushToGitRepo("basic-resources/modified", []string{"multiple-resources.yaml"}, false).Wait(45 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"multi-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"multi-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"multi-secret"})
	w.Expect().CheckCommitStatus()

	// removing secret from multiple-resources file should cause it to be deleted
	w.PushToGitRepo("basic-resources/initial-commit", []string{"multiple-resources.yaml"}, false).Wait(30 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"multi-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"multi-config"})
	w.Expect().ResourcesDontExist("v1", "secrets", []string{"multi-secret"})
	w.Expect().CheckCommitStatus()

	// deleting file with multiple resources will delete all resources in it
	w.PushToGitRepo("basic-resources/initial-commit", []string{"multiple-resources.yaml"}, true).Wait(30 * time.Second)
	w.Expect().ResourcesDontExist("apps/v1", "deployments", []string{"multi-deploy"})
	w.Expect().ResourcesDontExist("v1", "configmaps", []string{"multi-config"})
	w.Expect().CheckCommitStatus()

}

func TestFunctionalSuite(t *testing.T) {
	suite.Run(t, new(FunctionalSuite))
}
