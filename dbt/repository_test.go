package dbt

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func TestToolExists(t *testing.T) {
	dbtObj := testDbtConfig(port)

	exists, err := ToolExists(dbtObj.Tools.Repo, "foo")
	if err != nil {
		fmt.Printf("Failed to check repo for %q", "foo")
		t.Fail()
	}
	if !exists {
		fmt.Println(fmt.Sprintf("Tool %q does not exist in repo %s", "dbt", dbtObj.Dbt.Repo))
		t.Fail()
	}

	fakeToolName := "bar"

	exists, err = ToolExists(dbtObj.Tools.Repo, fakeToolName)
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
	dbtObj := testDbtConfig(port)

	if !ToolVersionExists(dbtObj.Tools.Repo, "foo", "1.2.3") {
		fmt.Println(fmt.Sprintf("Tool %q version %q does not exist in repo %s", "foo", "1.2.3", dbtObj.Tools.Repo))
		t.Fail()
	}

	if ToolVersionExists(dbtObj.Tools.Repo, "foo", "0.0.0") {
		fmt.Println(fmt.Sprintf("Nonexistant tool version %q shows existing in repo.", "0.0.0"))
		t.Fail()
	}
}

func TestFetchToolVersions(t *testing.T) {
	dbtObj := testDbtConfig(port)

	versions, err := FetchToolVersions(dbtObj.Tools.Repo, "foo")
	if err != nil {
		fmt.Println(fmt.Sprintf("Error searching for versions of tool %q in repo %q", "foo", dbtObj.Tools.Repo))
	}

	assert.True(t, len(versions) == 2, "List of versions has 2 elements.")
}

func TestFetchFile(t *testing.T) {

	targetDir := fmt.Sprintf("%s/%s", tmpDir, toolDir)
	fileUrl := fmt.Sprintf("%s/foo/1.2.2/linux/x86_64/foo", testToolUrl(port))
	fileName := fmt.Sprintf("%s/foo", targetDir)

	err := FetchFile(fileUrl, fileName)
	if err != nil {
		fmt.Printf("Error fetching file %q: %s\n", fileUrl, err)
		t.Fail()
	}

	success, err := VerifyFileChecksum(fileName, dbtVersionASha256())
	if err != nil {
		fmt.Println(fmt.Sprintf("Error checksumming test file: %s", err))
		t.Fail()
	}

	assert.True(t, success, "Checksum of downloaded file matches expectations.")

	success, err = VerifyFileVersion(fileUrl, fileName)
	if err != nil {
		fmt.Printf("Failed to verify version: %s", err)
		t.Fail()
	}

	assert.True(t, success, "Verified version of downloaded file.")

	failure, err := VerifyFileVersion(fmt.Sprintf("%s/dbt/1.2.3/linux/x86_64/dbt", testToolUrl(port)), fileName)
	if err != nil {
		fmt.Printf("Verified non-existent version: %s", err)
		t.Fail()
	}

	assert.False(t, failure, "Verified a false version does not match.")

	// download trust store
	trustStoreUrl := fmt.Sprintf("%s/truststore", testDbtUrl(port))
	trustStoreFile := fmt.Sprintf("%s/%s", tmpDir, truststorePath)

	err = FetchFile(trustStoreUrl, trustStoreFile)
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

	err = FetchFile(sigUrl, sigFile)
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
	success, err = VerifyFileSignature(tmpDir, fileName)
	if err != nil {
		fmt.Printf("Error verifying signature: %s", err)
		t.Fail()
	}

	assert.True(t, success, "Signature of downloaded file verified.")
}
