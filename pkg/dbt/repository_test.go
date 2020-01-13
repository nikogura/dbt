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

	assert.True(t, len(versions) == 1, "List of versions has 1 element.")
}

func TestFetchFile(t *testing.T) {
	toolName := "catalog_darwin_amd64"
	testFile := testFileMap[toolName]
	fileUrl := testFile.TestUrl
	fileName := testFile.FilePath

	dbtObj := &DBT{
		Config:  dbtConfig,
		Verbose: false,
		Logger:  log.New(os.Stderr, "", 0),
	}

	// TODO this won't work, you're clobbering the file in the repo.  Need a temp dir somewhere to download the file to
	err := dbtObj.FetchFile(fileUrl, fileName)
	if err != nil {
		fmt.Printf("Error fetching file %q: %s\n", fileUrl, err)
		t.Fail()
	}

	checksumBytes, err := ioutil.ReadFile(fmt.Sprintf("%s.sha256", fileName))
	if err != nil {
		fmt.Printf("Error reading checksumfile %s.sha256: %s\n", toolName, err)
		t.Fail()
	}

	fmt.Printf("--- checksum: %s ---\n", checksumBytes)

	success, err := dbtObj.VerifyFileChecksum(fileName, string(checksumBytes))
	if err != nil {
		fmt.Println(fmt.Sprintf("Error checksumming test file: %s", err))
		t.Fail()
	}

	assert.True(t, success, "Checksum of downloaded file matches expectations.")

	success, err = dbtObj.VerifyFileVersion(fileUrl, fileName)
	if err != nil {
		fmt.Printf("Failed to verify version: %s", err)
		t.Fail()
	}

	assert.True(t, success, "Verified version of downloaded file.")

	failure, err := dbtObj.VerifyFileVersion(fmt.Sprintf("%s/dbt/1.2.3/linux/amd64/dbt", testToolUrl(port)), fileName)
	if err != nil {
		fmt.Printf("Verified non-existent version: %s", err)
		t.Fail()
	}

	assert.False(t, failure, "Verified a false version does not match.")

	// download trust store
	trustStoreUrl := fmt.Sprintf("%s/truststore", testDbtUrl(port))
	trustStoreFile := fmt.Sprintf("%s/%s", tmpDir, TruststorePath)

	err = dbtObj.FetchFile(trustStoreUrl, trustStoreFile)
	if err != nil {
		fmt.Printf("Error fetching truststore %q: %s\n", fileUrl, err)
		t.Fail()
	}

	if _, err = os.Stat(trustStoreFile); os.IsNotExist(err) {
		fmt.Printf("Failed to download truststore")
		t.Fail()
	}

	trustBytes, err := ioutil.ReadFile(trustStoreFile)
	if err != nil {
		fmt.Printf("Failed to read downloaded truststore: %s\n", err)
		t.Fail()
	}

	assert.False(t, string(trustBytes) == "", "Downloaded Truststore is not empty")

	// download signature
	sigUrl := fmt.Sprintf("%s.asc", fileUrl)
	sigFile := fmt.Sprintf("%s.asc", fileName)

	err = dbtObj.FetchFile(sigUrl, sigFile)
	if err != nil {
		fmt.Printf("Error fetching signature %q: %s\n", sigUrl, err)
		t.Fail()
	}

	if _, err = os.Stat(sigFile); os.IsNotExist(err) {
		fmt.Printf("Failed to download signature")
		t.Fail()
	}

	sigBytes, err := ioutil.ReadFile(sigFile)
	if err != nil {
		fmt.Printf("Failed to read downloaded signature: %s\n", err)
		t.Fail()
	}

	assert.False(t, string(sigBytes) == "", "Downloaded Signature is not empty")

	// verify signature
	success, err = dbtObj.VerifyFileSignature(tmpDir, fileName)
	if err != nil {
		fmt.Printf("Error verifying signature: %s", err)
		t.Fail()
	}

	assert.True(t, success, "Signature of downloaded file verified.")
}

func TestFindLatestVersion(t *testing.T) {
	dbtObj := &DBT{
		Config:  dbtConfig,
		Verbose: true,
	}

	toolName := "catalog"

	latest, err := dbtObj.FindLatestVersion(toolName)
	if err != nil {
		fmt.Printf("Error finding latest version: %s", err)
		t.Fail()
	}

	assert.Equal(t, VERSION, latest, "Latest version meets expectations.")

}

// TODO Make old versions of tools so we can test that we're getting the latest
