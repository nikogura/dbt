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
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func TestToolExists(t *testing.T) {
	dbtObj := &DBT{
		Config:  dbtConfig,
		Verbose: false,
		Logger:  log.New(os.Stderr, "", 0),
	}

	exists, err := dbtObj.ToolExists("boilerplate")
	if err != nil {
		fmt.Printf("Failed to check repo for %q", "boilerplate")
		t.Fail()
	}
	if !exists {
		fmt.Println(fmt.Sprintf("Tool %q does not exist in repo %s", "dbt", dbtObj.Config.Dbt.Repo))
		t.Fail()
	}

	fakeToolName := "bar"

	exists, err = dbtObj.ToolExists(fakeToolName)
	if err != nil {
		fmt.Printf("Failed to check artifactory for %q", fakeToolName)
		t.Fail()
	}

	if exists {
		fmt.Println("Nonexistant job shows existing in repo.")
		t.Fail()
	}
}

func TestToolVersionExists(t *testing.T) {
	dbtObj := &DBT{
		Config:  dbtConfig,
		Verbose: false,
		Logger:  log.New(os.Stderr, "", 0),
	}

	toolName := "boilerplate"

	ok, err := dbtObj.ToolVersionExists(toolName, VERSION)
	if err != nil {
		log.Printf("Error checking if version exists: %s", err)
		t.Fail()
	}

	if !ok {
		fmt.Println(fmt.Sprintf("Tool %q version %q does not exist in repo %s", toolName, VERSION, dbtObj.Config.Tools.Repo))
		t.Fail()
	}

	ok, _ = dbtObj.ToolVersionExists("foo", "0.0.0")

	if ok {
		fmt.Println(fmt.Sprintf("Nonexistant tool version %q shows existing in repo.", "0.0.0"))
		t.Fail()
	}
}

func TestFetchToolVersions(t *testing.T) {
	dbtObj := &DBT{
		Config:  dbtConfig,
		Verbose: false,
		Logger:  log.New(os.Stderr, "", 0),
	}

	toolName := "boilerplate"

	versions, err := dbtObj.FetchToolVersions(toolName)
	if err != nil {
		fmt.Println(fmt.Sprintf("Error searching for versions of tool %q in repo %q", toolName, dbtObj.Config.Tools.Repo))
	}

	assert.True(t, len(versions) == 2, "ListCatalog of versions should have 2 elements.")
}

func TestFetchFile(t *testing.T) {
	toolName := "catalog_darwin_amd64"
	testFile := testFilesA[toolName]
	fileUrl := testFile.TestUrl
	fileName := fmt.Sprintf("%s/fetchfile", tmpDir)
	checksumUrl := fmt.Sprintf("%s.sha256", fileUrl)
	checksumFile := fmt.Sprintf("%s.sha256", fileName)

	dbtObj := &DBT{
		Config:  dbtConfig,
		Verbose: false,
		Logger:  log.New(os.Stderr, "", 0),
	}

	t.Logf("downloading %s", fileUrl)
	err := dbtObj.FetchFile(fileUrl, fileName)
	if err != nil {
		t.Errorf("Error fetching file %q: %s\n", fileUrl, err)
	}

	t.Logf("downloading %s", checksumUrl)
	err = dbtObj.FetchFile(checksumUrl, checksumFile)
	if err != nil {
		t.Errorf("Error fetching file %q: %s\n", fileUrl, err)
	}
	//
	checksumBytes, err := ioutil.ReadFile(checksumFile)
	if err != nil {
		t.Errorf("Error reading checksumfile %s.sha256: %s\n", toolName, err)
	}

	success, err := dbtObj.VerifyFileChecksum(fileName, string(checksumBytes))
	if err != nil {
		t.Errorf(fmt.Sprintf("Error checksumming test file: %s", err))
	}

	assert.True(t, success, "Checksum of downloaded file matches expectations.")

	t.Logf("Verifying version of %s", fileUrl)
	success, err = dbtObj.VerifyFileVersion(fileUrl, fileName)
	if err != nil {
		t.Errorf("Failed to verify version: %s", err)
	}

	assert.True(t, success, "Verified version of downloaded file.")

	failure, err := dbtObj.VerifyFileVersion(fmt.Sprintf("%s/dbt/1.2.3/linux/amd64/dbt", testToolUrl(port)), fileName)
	if err != nil {
		t.Errorf("Verified non-existent version: %s", err)
	}

	assert.False(t, failure, "Verified a false version does not match.")

	// download trust store
	trustStoreUrl := fmt.Sprintf("%s/truststore", testDbtUrl(port))
	trustStoreFile := fmt.Sprintf("%s/%s", tmpDir, TruststorePath)

	t.Logf("Fetching truststore from %s", trustStoreUrl)
	err = dbtObj.FetchFile(trustStoreUrl, trustStoreFile)
	if err != nil {
		t.Errorf("Error fetching truststore %q: %s\n", fileUrl, err)
	}

	if _, err = os.Stat(trustStoreFile); os.IsNotExist(err) {
		t.Errorf("Failed to download truststore")
	}

	trustBytes, err := ioutil.ReadFile(trustStoreFile)
	if err != nil {
		t.Errorf("Failed to read downloaded truststore: %s\n", err)
	}

	assert.False(t, string(trustBytes) == "", "Downloaded Truststore is not empty")

	// download signature
	sigUrl := fmt.Sprintf("%s.asc", fileUrl)
	sigFile := fmt.Sprintf("%s.asc", fileName)

	t.Logf("Downloading %s", sigUrl)
	err = dbtObj.FetchFile(sigUrl, sigFile)
	if err != nil {
		t.Errorf("Error fetching signature %q: %s\n", sigUrl, err)
	}

	if _, err = os.Stat(sigFile); os.IsNotExist(err) {
		t.Errorf("Failed to download signature")
	}

	sigBytes, err := ioutil.ReadFile(sigFile)
	if err != nil {
		t.Errorf("Failed to read downloaded signature: %s\n", err)
	}

	assert.False(t, string(sigBytes) == "", "Downloaded Signature is not empty")

	// verify signature
	t.Logf("verifying signature of %s", fileName)
	success, err = dbtObj.VerifyFileSignature(tmpDir, fileName)
	if err != nil {
		t.Errorf("Error verifying signature: %s", err)
	}

	assert.True(t, success, "Signature of downloaded file verified.")
	t.Logf("Signature Verified")
}

func TestFindLatestVersion(t *testing.T) {
	dbtObj := &DBT{
		Config:  dbtConfig,
		Verbose: true,
	}

	toolName := "catalog"

	latest, err := dbtObj.FindLatestVersion(toolName)
	if err != nil {
		t.Errorf("Error finding latest version: %s", err)
	}

	assert.Equal(t, VERSION, latest, "Latest version meets expectations.")
}
