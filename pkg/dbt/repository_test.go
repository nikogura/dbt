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
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

func TestToolExists(t *testing.T) {
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
			exists, err := tc.obj.ToolExists("boilerplate")
			if err != nil {
				t.Errorf("Failed to check repo for %q: %s\n", "boilerplate", err)
			}
			if !exists {
				t.Errorf("Tool %q does not exist in repo %s\n", "boilerplate", tc.obj.Config.Tools.Repo)
			}

			fakeToolName := "bar"

			exists, err = tc.obj.ToolExists(fakeToolName)
			if err != nil {
				t.Errorf("Failed to check repo for %q\n", fakeToolName)
			}

			if exists {
				t.Errorf("Nonexistant job shows existing in repo.\n")
			}
		})
	}
}

func TestToolVersionExists(t *testing.T) {
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
			toolName := "boilerplate"

			ok, err := tc.obj.ToolVersionExists(toolName, VERSION)
			if err != nil {
				log.Printf("Error checking if version exists: %s", err)
				t.Fail()
			}

			if !ok {
				fmt.Println(fmt.Sprintf("Tool %q version %q does not exist in repo %s", toolName, VERSION, tc.obj.Config.Tools.Repo))
				t.Fail()
			}

			ok, _ = tc.obj.ToolVersionExists("foo", "0.0.0")

			if ok {
				fmt.Println(fmt.Sprintf("Nonexistant tool version %q shows existing in repo.", "0.0.0"))
				t.Fail()
			}

		})
	}
}

func TestFetchToolVersions(t *testing.T) {
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
			toolName := "boilerplate"

			versions, err := tc.obj.FetchToolVersions(toolName)
			if err != nil {
				fmt.Println(fmt.Sprintf("Error searching for versions of tool %q in repo %q", toolName, tc.obj.Config.Tools.Repo))
			}

			assert.True(t, len(versions) == 2, "ListCatalog of versions should have 2 elements.")

		})
	}
}

func TestFetchFile(t *testing.T) {

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
			toolName := "catalog_linux_amd64"
			testFile := testFilesA[toolName]
			fileUrl := testFile.TestUrl
			fileName := fmt.Sprintf("%s/fetchfile", tc.homedir)
			checksumUrl := fmt.Sprintf("%s.sha256", fileUrl)
			checksumFile := fmt.Sprintf("%s.sha256", fileName)

			t.Logf("downloading %s", fileUrl)
			err := tc.obj.FetchFile(fileUrl, fileName)
			if err != nil {
				t.Errorf("Error fetching file %q: %s\n", fileUrl, err)
			}

			t.Logf("downloading %s", checksumUrl)
			err = tc.obj.FetchFile(checksumUrl, checksumFile)
			if err != nil {
				t.Errorf("Error fetching file %q: %s\n", fileUrl, err)
			}

			checksumBytes, err := os.ReadFile(checksumFile)
			if err != nil {
				t.Errorf("Error reading checksumfile %s.sha256: %s\n", toolName, err)
			}

			success, err := tc.obj.VerifyFileChecksum(fileName, string(checksumBytes))
			if err != nil {
				t.Errorf(fmt.Sprintf("Error checksumming test file: %s", err))
			}

			assert.True(t, success, "Checksum of downloaded file matches expectations.")

			t.Logf("Verifying version of %s", fileUrl)
			success, err = tc.obj.VerifyFileVersion(fileUrl, fileName)
			if err != nil {
				t.Errorf("Failed to verify version: %s", err)
			}

			assert.True(t, success, "Verified version of downloaded file.")

			failure, err := tc.obj.VerifyFileVersion(fmt.Sprintf("%s/dbt/1.2.3/linux/amd64/dbt", testToolUrl(port)), fileName)
			if err != nil {
				t.Errorf("Verified non-existent version: %s", err)
			}

			assert.False(t, failure, "Verified a false version does not match.")

			// download trust store
			trustStoreUrl := fmt.Sprintf("%s/truststore", testDbtUrl(port))
			trustStoreFile := fmt.Sprintf("%s/%s", tc.homedir, TruststorePath)

			t.Logf("Fetching truststore from %s", trustStoreUrl)
			err = tc.obj.FetchFile(trustStoreUrl, trustStoreFile)
			if err != nil {
				t.Errorf("Error fetching truststore %q: %s\n", fileUrl, err)
			}

			if _, err = os.Stat(trustStoreFile); os.IsNotExist(err) {
				t.Errorf("Failed to download truststore")
			}

			trustBytes, err := os.ReadFile(trustStoreFile)
			if err != nil {
				t.Errorf("Failed to read downloaded truststore: %s\n", err)
			}

			assert.False(t, string(trustBytes) == "", "Downloaded Truststore is not empty")

			// download signature
			sigUrl := fmt.Sprintf("%s.asc", fileUrl)
			sigFile := fmt.Sprintf("%s.asc", fileName)

			t.Logf("Downloading %s", sigUrl)
			err = tc.obj.FetchFile(sigUrl, sigFile)
			if err != nil {
				t.Errorf("Error fetching signature %q: %s\n", sigUrl, err)
			}

			if _, err = os.Stat(sigFile); os.IsNotExist(err) {
				t.Errorf("Failed to download signature")
			}

			sigBytes, err := os.ReadFile(sigFile)
			if err != nil {
				t.Errorf("Failed to read downloaded signature: %s\n", err)
			}

			assert.False(t, string(sigBytes) == "", "Downloaded Signature is not empty")

			// verify signature
			t.Logf("verifying signature of %s", fileName)
			success, err = tc.obj.VerifyFileSignature(tc.homedir, fileName)
			if err != nil {
				t.Errorf("Error verifying signature: %s", err)
			}

			assert.True(t, success, "Signature of downloaded file verified.")
			t.Logf("Signature Verified")

		})
	}
}

func TestFindLatestVersion(t *testing.T) {
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
			toolName := "catalog"

			latest, err := tc.obj.FindLatestVersion(toolName)
			if err != nil {
				t.Errorf("Error finding latest version: %s", err)
			}

			assert.Equal(t, VERSION, latest, "Latest version meets expectations.")

		})
	}
}

func TestDefaultSession(t *testing.T) {
	_, err := DefaultSession(nil)
	if err != nil {
		t.Errorf("Failed to get an AWS Session")
	}
}

func TestS3Url(t *testing.T) {
	inputs := []struct {
		url    string
		result bool
		bucket string
		region string
		key    string
	}{
		{
			"https://www.nikogura.com",
			false,
			"",
			"",
			"",
		},
		{
			"https://dbt-tools.s3.us-east-1.amazonaws.com/catalog/1.2.3/linux/amd64/catalog",
			true,
			"dbt-tools",
			"us-east-1",
			"catalog/1.2.3/linux/amd64/catalog",
		},
	}

	for _, tc := range inputs {
		t.Run(tc.url, func(t *testing.T) {
			fmt.Printf("Testing %s\n", tc.url)
			ok, meta := S3Url(tc.url)

			assert.True(t, ok == tc.result, fmt.Sprintf("%s does not meet expectations", tc.url))
			assert.True(t, tc.bucket == meta.Bucket, fmt.Sprintf("Bucket %q doesn't look right", meta.Bucket))
			assert.True(t, tc.region == meta.Region, fmt.Sprintf("Region %q doesn't look right.", meta.Region))
			assert.True(t, tc.key == meta.Key, fmt.Sprintf("Key %q doesn't look right.", meta.Key))
		})
	}
}

func TestDirsForPath(t *testing.T) {
	inputs := []struct {
		name   string
		input  string
		output []string
	}{
		{
			"s3 reposerver url",
			"https://foo.com/dbt-tools/catalog/1.2.3/linux/amd64/catalog",
			[]string{
				"dbt-tools",
				"dbt-tools/catalog",
				"dbt-tools/catalog/1.2.3",
				"dbt-tools/catalog/1.2.3/linux",
				"dbt-tools/catalog/1.2.3/linux/amd64",
			},
		},
		{
			"s3 catalog url",
			"https://dbt-tools.s3.us-east-1.amazonaws.com/catalog/1.2.3/linux/amd64/catalog",
			[]string{
				"catalog",
				"catalog/1.2.3",
				"catalog/1.2.3/linux",
				"catalog/1.2.3/linux/amd64",
			},
		},
	}

	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			dirs, err := DirsForURL(tc.input)
			if err != nil {
				t.Error(err)
			}

			assert.Equal(t, tc.output, dirs, "Parsed directories meet expectations")
		})
	}
}
