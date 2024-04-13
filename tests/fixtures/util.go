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

package fixtures

import (
	"io"
	"os"
	"strings"
)

func CopyFile(src, dst string) error {
	// read source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}

	// create dest file, overwrite if already exists
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}

	err = destFile.Sync()
	if err != nil {
		return err
	}

	err = srcFile.Close()
	if err != nil {
		return err
	}

	err = destFile.Close()
	if err != nil {
		return err
	}

	return nil
}

// TODO: extend to trim URLs for ssh/https
func TrimRepoUrl(repoUrl string) string {

	return strings.TrimPrefix(repoUrl, "http://localgitserver-service.numaplane-system.svc.cluster.local/git/")

}
