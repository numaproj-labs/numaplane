package v1alpha1

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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_FileKeySelector_GetSecretValue(t *testing.T) {

	testCases := []struct {
		name          string
		filePathJSON  string
		filePathYAML  string
		key           string
		expectedError bool
		expectedValue string
	}{
		{
			name:          "Valid JSON",
			filePathJSON:  "testdata/file.json",
			key:           "key2",
			expectedError: false,
			expectedValue: "value2",
		},
		{
			name:          "Valid YAML",
			filePathYAML:  "testdata/file.yaml",
			key:           "key2",
			expectedError: false,
			expectedValue: "value2",
		},
		{
			name:          "Invalid JSON",
			filePathJSON:  "testdata/unexpectedFormat.json",
			key:           "key2",
			expectedError: true,
		},
		{
			name:          "Invalid YAML",
			filePathYAML:  "testdata/unexpectedFormat.yaml",
			key:           "key2",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var fks FileKeySelector
			if tc.filePathJSON != "" {
				fks = FileKeySelector{
					JSONFilePath: &tc.filePathJSON,
					Key:          tc.key,
				}
			}
			if tc.filePathYAML != "" {
				fks = FileKeySelector{
					YAMLFilePath: &tc.filePathYAML,
					Key:          tc.key,
				}
			}

			value, err := fks.GetSecretValue()
			if tc.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expectedValue, value)
			}
		})
	}
}
