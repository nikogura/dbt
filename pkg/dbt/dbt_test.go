// Copyright © 2019 Nik Ogura <nik.ogura@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbt

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/assert"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestS3List(t *testing.T) {
	svc := s3.New(s3Session)
	input := &s3.ListObjectsInput{
		Bucket:  aws.String("dbt-tools"),
		MaxKeys: aws.Int64(100),
	}

	_, err := svc.ListObjects(input)
	if err != nil {
		t.Errorf("Error listing s3 objects")
	}
}

func TestRepoGet(t *testing.T) {
	for _, f := range testFilesB {
		t.Run(f.Name, func(t *testing.T) {
			url := fmt.Sprintf("%s%s", testHost, f.UrlPath)

			resp, err := http.Get(url)
			if err != nil {
				t.Errorf("Failed to fetch %s: %s", f.Name, err)
			}

			assert.True(t, resp.StatusCode < 300, "Non success error code fetching %s (%d)", url, resp.StatusCode)

			// fetch via s3
			key := strings.TrimPrefix(f.UrlPath, fmt.Sprintf("/%s/", f.Repo))
			log.Printf("Fetching %s from s3", key)
			headOptions := &s3.HeadObjectInput{
				Bucket: aws.String(f.Repo),
				Key:    aws.String(key),
			}

			headSvc := s3.New(s3Session)

			_, err = headSvc.HeadObject(headOptions)
			if err != nil {
				t.Errorf("failed to get metadata for %s: %s", f.Name, err)
			}
		})
	}
}

func TestGenerateDbtDir(t *testing.T) {
	var inputs = []struct {
		name string
		path string
	}{
		{
			"reposerver",
			homeDirRepoServer,
		},
		{
			"s3",
			homeDirS3,
		},
	}

	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			dbtDirPath := fmt.Sprintf("%s/%s", tc.path, DbtDir)

			if _, err := os.Stat(dbtDirPath); os.IsNotExist(err) {
				t.Errorf("dbt dir %s did not create as expected", dbtDirPath)
			}

			trustPath := fmt.Sprintf("%s/%s", tc.path, TrustDir)

			if _, err := os.Stat(trustPath); os.IsNotExist(err) {
				t.Errorf("trust dir %s did not create as expected", trustPath)
			}

			toolPath := fmt.Sprintf("%s/%s", tc.path, ToolDir)
			if _, err := os.Stat(toolPath); os.IsNotExist(err) {
				t.Errorf("tool dir %s did not create as expected", toolPath)
			}

			configPath := fmt.Sprintf("%s/%s", tc.path, ConfigDir)
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Errorf("config dir %s did not create as expected", configPath)
			}

		})
	}
}

func TestLoadDbtConfig(t *testing.T) {
	var inputs = []struct {
		name     string
		path     string
		contents string
		expected Config
	}{
		{
			"reposerver",
			homeDirRepoServer,
			testDbtConfigContents(port),
			dbtConfig,
		},
		{
			"s3",
			homeDirS3,
			testDbtConfigS3Contents(),
			s3DbtConfig,
		},
	}

	for _, tc := range inputs {

		t.Run(tc.name, func(t *testing.T) {
			expected := tc.expected
			actual, err := LoadDbtConfig(tc.path, true)
			if err != nil {
				t.Errorf("Error loading config file: %s", err)
			}

			assert.Equal(t, expected, actual, "Parsed config meets expectations")
		})
	}
}

func TestFetchTrustStore(t *testing.T) {
	inputs := []struct {
		name    string
		obj     *DBT
		homedir string
	}{
		{
			"reposerver",

			&DBT{
				Config:  dbtConfig,
				Verbose: true,
			},
			homeDirRepoServer,
		},
		{
			"s3",
			&DBT{
				Config:    s3DbtConfig,
				Verbose:   true,
				S3Session: s3Session,
			},
			homeDirS3,
		},
	}

	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.obj.FetchTrustStore(tc.homedir)
			if err != nil {
				t.Errorf("Error fetching trust store: %s", err)
			}

			expected := trustfileContents
			trustPath := fmt.Sprintf("%s/%s", tc.homedir, TruststorePath)

			if _, err := os.Stat(trustPath); os.IsNotExist(err) {
				t.Errorf("File not written")
			}

			actualBytes, err := os.ReadFile(trustPath)
			if err != nil {
				t.Errorf("Error reading trust store: %s", err)
			}

			actual := string(actualBytes)

			assert.Equal(t, expected, actual, "Read truststore contents matches expectations.")

		})
	}
}

func TestDbtIsCurrent(t *testing.T) {
	inputs := []struct {
		name    string
		obj     *DBT
		homedir string
		oldUrl  string
		newUrl  string
	}{
		{
			"reposerver",

			&DBT{
				Config:  dbtConfig,
				Verbose: true,
			},
			homeDirRepoServer,
			fmt.Sprintf("http://127.0.0.1:%d/dbt/%s/%s/amd64/dbt", port, oldVersion, runtime.GOOS),
			fmt.Sprintf("http://127.0.0.1:%d/dbt/%s/%s/amd64/dbt", port, VERSION, runtime.GOOS),
		},
		{
			"s3",
			&DBT{
				Config:    s3DbtConfig,
				Verbose:   true,
				S3Session: s3Session,
			},
			homeDirS3,
			fmt.Sprintf("https://dbt.s3.us-east-1.amazonaws.com/%s/%s/amd64/dbt", oldVersion, runtime.GOOS),
			fmt.Sprintf("https://dbt.s3.us-east-1.amazonaws.com/%s/%s/amd64/dbt", VERSION, runtime.GOOS),
		},
	}

	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			targetDir := fmt.Sprintf("%s/%s", tc.homedir, ToolDir)
			fileUrl := tc.oldUrl
			fileName := fmt.Sprintf("%s/dbt", targetDir)

			err := tc.obj.FetchFile(fileUrl, fileName)
			if err != nil {
				t.Errorf("Error fetching file %q: %s\n", fileUrl, err)
			}

			ok, err := tc.obj.IsCurrent(fileName)
			if err != nil {
				t.Errorf("error checking to see if download file is current: %s\n", err)
			}

			assert.False(t, ok, "Old version should not show up as current.")

			fileUrl = tc.newUrl
			fileName = fmt.Sprintf("%s/dbt", targetDir)

			err = tc.obj.FetchFile(fileUrl, fileName)
			if err != nil {
				t.Errorf("Error fetching file %q: %s\n", fileUrl, err)
			}

			ok, err = tc.obj.IsCurrent(fileName)
			if err != nil {
				t.Errorf("error checking to see if download file is current: %s\n", err)
			}

			assert.True(t, ok, "Current version shows current.")
		})
	}
}

func TestDbtUpgradeInPlace(t *testing.T) {
	inputs := []struct {
		name    string
		obj     *DBT
		homedir string
		oldUrl  string
		newUrl  string
	}{
		{
			"reposerver",

			&DBT{
				Config:  dbtConfig,
				Verbose: true,
			},
			homeDirRepoServer,
			fmt.Sprintf("http://127.0.0.1:%d/dbt/%s/%s/amd64/dbt", port, oldVersion, runtime.GOOS),
			fmt.Sprintf("http://127.0.0.1:%d/dbt/%s/%s/amd64/dbt", port, VERSION, runtime.GOOS),
		},
		{
			"s3",
			&DBT{
				Config:    s3DbtConfig,
				Verbose:   true,
				S3Session: s3Session,
			},
			homeDirS3,
			fmt.Sprintf("https://dbt.s3.us-east-1.amazonaws.com/%s/%s/amd64/dbt", oldVersion, runtime.GOOS),
			fmt.Sprintf("https://dbt.s3.us-east-1.amazonaws.com/%s/%s/amd64/dbt", VERSION, runtime.GOOS),
		},
	}

	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			targetDir := fmt.Sprintf("%s/%s", tc.homedir, ToolDir)
			fileUrl := tc.oldUrl
			fileName := fmt.Sprintf("%s/dbt", targetDir)

			err := tc.obj.FetchFile(fileUrl, fileName)
			if err != nil {
				t.Errorf("Error fetching file %q: %s\n", fileUrl, err)
			}

			ok, err := tc.obj.IsCurrent(fileName)
			if err != nil {
				t.Errorf("error checking to see if download file is current: %s\n", err)
			}

			assert.False(t, ok, "Old version should not show up as current.")

			err = tc.obj.UpgradeInPlace(fileName)
			if err != nil {
				t.Errorf("Error upgrading in place: %s", err)
			}

			ok, err = tc.obj.IsCurrent(fileName)
			if err != nil {
				t.Errorf("error checking to see if download file is current: %s\n", err)
			}

			assert.True(t, ok, "Current version shows current.")
		})
	}
}

func TestNewDbt(t *testing.T) {
	var inputs = []struct {
		name    string
		homedir string
	}{
		{
			"reposerver",
			homeDirRepoServer,
		},
		{
			"s3",
			homeDirS3,
		},
	}

	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			configPath := fmt.Sprintf("%s/%s", tc.homedir, ConfigDir)
			fileName := fmt.Sprintf("%s/dbt.json", configPath)

			if _, err := os.Stat(fileName); os.IsNotExist(err) {
				fmt.Printf("Writing test dbt config to %s", fileName)
				err = GenerateDbtDir(tc.homedir, true)
				if err != nil {
					t.Errorf("Error generating dbt dir: %s", err)
				}

				err = os.WriteFile(fileName, []byte(testDbtConfigContents(port)), 0644)
				if err != nil {
					t.Errorf("Error writing config file to %s: %s", fileName, err)
				}
			}

			_, err := NewDbt(tc.homedir)
			if err != nil {
				t.Errorf("Error creating DBT object: %s", err)
			}
		})
	}
}

func TestGetHomeDir(t *testing.T) {
	_, err := GetHomeDir()
	if err != nil {
		t.Errorf("Error getting homedir: %s", err)
	}
}

