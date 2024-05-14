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

package kustomize_e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	. "github.com/numaproj-labs/numaplane/tests/fixtures"
)

type KustomizeSuite struct {
	E2ESuite
}

// GitSync testing with kustomize manifests
func (s *KustomizeSuite) TestKustomize() {

	// create repo containing kustomize manifests
	w := s.Given().GitSync("@testdata/kustomize-gitsync.yaml").InitializeGitRepo("kustomize/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	// verify all resources defined in kustomization file are created
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"kustomize-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"kustomize-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"kustomize-secret"})
	// these resources are defined in the same file
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"multi-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"multi-config"})
	w.Expect().CheckCommitStatus()

	// remove deployment resource from kustomization file
	w.PushToGitRepo("kustomize/modified", []string{"kustomization.yaml"}, false).Wait(30 * time.Second)
	// verify that deployment should be deleted while other resources remain
	w.Expect().ResourcesDontExist("apps/v1", "deployments", []string{"kustomize-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"kustomize-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"kustomize-secret"})
	w.Expect().CheckCommitStatus()

	w.PushToGitRepo("kustomize/modified", []string{"multiple-resources.yaml"}, false).Wait(30 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"multi-deploy"})
	w.Expect().ResourcesExist("v1", "configmaps", []string{"multi-config"})
	w.Expect().ResourcesExist("v1", "secrets", []string{"multi-secret"})
	w.Expect().CheckCommitStatus()

}

// GitSync auto healing occurs when resource does not match manifest in repo
func (s *KustomizeSuite) TestAutoHealing() {

	w := s.Given().GitSync("@testdata/kustomize-gitsync.yaml").InitializeGitRepo("kustomize/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	// verify that test deployment is created with {replicas: 3} in spec
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"kustomize-deploy"})
	w.Expect().CheckCommitStatus()

	w.Expect().VerifyResourceState("apps/v1", "deployments", "kustomize-deploy", "spec", "replicas", 3)

	// apply patch to resource
	w.ModifyResource("apps/v1", "deployments", "kustomize-deploy", `{"spec":{"replicas":4}}`).Wait(30 * time.Second)

	// verify that resource has been "healed" by resetting replica count to 3
	w.Expect().VerifyResourceState("apps/v1", "deployments", "kustomize-deploy", "spec", "replicas", 3)

	// disable autohealing
	w.UpdateAutoHealConfig(false).Wait(30 * time.Second)

	// apply patch to resource
	w.ModifyResource("apps/v1", "deployments", "kustomize-deploy", `{"spec":{"replicas":4}}`).Wait(30 * time.Second)

	// verify that resource has not been healed like previous
	w.Expect().VerifyResourceState("apps/v1", "deployments", "kustomize-deploy", "spec", "replicas", 4)

	// reenable autohealing for test cases left to run
	w.UpdateAutoHealConfig(true)
}

func TestKustomizeSuite(t *testing.T) {
	suite.Run(t, new(KustomizeSuite))
}
