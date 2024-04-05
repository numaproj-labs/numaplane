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

	"github.com/stretchr/testify/suite"

	. "github.com/numaproj-labs/numaplane/tests/fixtures"
)

type FunctionalSuite struct {
	E2ESuite
}

// TODO
func (s *FunctionalSuite) TestCreateGitSync() {
	w := s.Given().GitSync("@testdata/gitsync.yaml").
		When().
		CreateGitSyncAndWait()
	defer w.DeleteGitSyncAndWait()

	w.Expect().ResourcesExist("pipelines", []string{"simple-pipeline"})
}

// potentially this can all be in one case

func (s *FunctionalSuite) TestDeleteGitSyncAndResources() {
	w := s.Given().GitSync("@testdata/gitsync.yaml").
		When().
		CreateGitSyncAndWait()

	// verify that all resources are deleted when GitSync is deleted
	w.DeleteGitSyncAndWait().
		Expect().
		ResourcesDontExist("pipelines", []string{"simple-pipeline"})
}

// func (s *FunctionalSuite) TestSelfHealing() {

// }

func TestFunctionalSuite(t *testing.T) {
	suite.Run(t, new(FunctionalSuite))
}
