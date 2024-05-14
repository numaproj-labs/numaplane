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

package helm_e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	. "github.com/numaproj-labs/numaplane/tests/fixtures"
)

type HelmSuite struct {
	E2ESuite
}

// GitSync testing with basic helm manifests
func (s *HelmSuite) TestHelm() {

	w := s.Given().GitSync("@testdata/helm-gitsync.yaml").InitializeGitRepo("helm/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	// verify all resources defined in helm file are created
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"gitsync-example-helm-test"})
	w.Expect().CheckCommitStatus()

	// remove deployment manifest from repo
	w.PushToGitRepo("helm/initial-commit", []string{"templates/deployment.yaml"}, true).Wait(10 * time.Second)
	w.Expect().ResourcesDontExist("apps/v1", "deployments", []string{"gitsync-example-helm-test"})
	w.Expect().CheckCommitStatus()

	// adding new template to repo
	w.PushToGitRepo("helm/modified", []string{"templates/new-deploy.yaml"}, false).Wait(10 * time.Second)
	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"new-deploy"})
	w.Expect().CheckCommitStatus()

	// modifying the values.yaml target for helm
	w.PushToGitRepo("helm/modified", []string{"values.yaml"}, false).Wait(10 * time.Second)
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

// GitSync auto healing occurs when resource does not match manifest in repo
func (s *HelmSuite) TestAutoHealing() {

	w := s.Given().GitSync("@testdata/helm-gitsync.yaml").InitializeGitRepo("helm/initial-commit").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	w.Expect().ResourcesExist("apps/v1", "deployments", []string{"gitsync-example-helm-test"})
	w.Expect().CheckCommitStatus()

	w.Expect().VerifyResourceState("apps/v1", "deployments", "gitsync-example-helm-test", "spec", "replicas", 1)

	// apply patch to resource
	w.ModifyResource("apps/v1", "deployments", "gitsync-example-helm-test", `{"spec":{"replicas":4}}`).Wait(10 * time.Second)

	// verify that resource has been "healed" by resetting replica count to 3
	w.Expect().VerifyResourceState("apps/v1", "deployments", "gitsync-example-helm-test", "spec", "replicas", 1)

	// disable autohealing
	w.UpdateAutoHealConfig(false).Wait(45 * time.Second)

	// apply patch to resource
	w.ModifyResource("apps/v1", "deployments", "gitsync-example-helm-test", `{"spec":{"replicas":4}}`).Wait(10 * time.Second)

	// verify that resource has not been healed like previous
	w.Expect().VerifyResourceState("apps/v1", "deployments", "gitsync-example-helm-test", "spec", "replicas", 4)

	// reenable autohealing for test cases left to run
	w.UpdateAutoHealConfig(true)
}

func TestHelmSuite(t *testing.T) {
	suite.Run(t, new(HelmSuite))
}
