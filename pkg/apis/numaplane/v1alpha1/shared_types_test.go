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

func Test_GetSecretValue(t *testing.T) {

	testCases := []struct {
		name          string
		filePath      string
		key           string
		expectedError bool
		expectedValue string
	}{
		{
			name:          "Valid JSON",
			filePath:      "testdata/file.json",
			key:           "key2",
			expectedError: false,
			expectedValue: "value2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fks := FileKeySelector{
				JSONFilePath: &tc.filePath,
				Key:          tc.key,
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
