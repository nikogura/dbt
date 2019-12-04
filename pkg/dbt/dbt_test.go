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
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"testing"
	"time"
)

var tmpDir string
var dbtConfig Config
var port int

func TestMain(m *testing.M) {
	setUp()

	code := m.Run()

	tearDown()

	os.Exit(code)
}

func setUp() {
	dir, err := ioutil.TempDir("", "dbt")
	if err != nil {
		fmt.Printf("Error creating temp dir %q: %s\n", tmpDir, err)
		os.Exit(1)
	}

	tmpDir = dir
	fmt.Printf("Temp dir: %s\n", tmpDir)

	freePort, err := freeport.GetFreePort()
	if err != nil {
		log.Printf("Error getting a free port: %s", err)
		os.Exit(1)
	}

	port = freePort

	dbtConfig = testDbtConfig(port)

	tr := TestRepo{}

	go tr.Run(port)

	log.Printf("Sleeping for 1 second for the test artifact server to start up.")
	time.Sleep(time.Second * 1)

	err = GenerateDbtDir(tmpDir, true)
	if err != nil {
		log.Printf("Error generating dbt dir: %s", err)
		os.Exit(1)
	}

}

func tearDown() {
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		_ = os.Remove(tmpDir)
	}

}

func TestGenerateDbtDir(t *testing.T) {
	dbtDirPath := fmt.Sprintf("%s/%s", tmpDir, DbtDir)

	if _, err := os.Stat(dbtDirPath); os.IsNotExist(err) {
		log.Printf("dbt dir %s did not create as expected", dbtDirPath)
		t.Fail()
	}

	trustPath := fmt.Sprintf("%s/%s", tmpDir, TrustDir)

	if _, err := os.Stat(trustPath); os.IsNotExist(err) {
		log.Printf("trust dir %s did not create as expected", trustPath)
		t.Fail()
	}

	toolPath := fmt.Sprintf("%s/%s", tmpDir, ToolDir)
	if _, err := os.Stat(toolPath); os.IsNotExist(err) {
		log.Printf("tool dir %s did not create as expected", toolPath)
		t.Fail()
	}

	configPath := fmt.Sprintf("%s/%s", tmpDir, ConfigDir)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("config dir %s did not create as expected", configPath)
		t.Fail()
	}

}

func TestLoadDbtConfig(t *testing.T) {
	configPath := fmt.Sprintf("%s/%s", tmpDir, ConfigDir)
	fileName := fmt.Sprintf("%s/dbt.json", configPath)

	err := ioutil.WriteFile(fileName, []byte(testDbtConfigContents(port)), 0644)
	if err != nil {
		log.Printf("Error writing config file to %s: %s", fileName, err)
		t.Fail()
	}

	expected := testDbtConfig(port)
	actual, err := LoadDbtConfig(tmpDir, true)
	if err != nil {
		log.Printf("Error loading config file: %s", err)
		t.Fail()
	}

	assert.Equal(t, expected, actual, "Parsed config meets expectations")

}

func TestDBT_FetchTrustStore(t *testing.T) {
	dbt := &DBT{
		Config:  dbtConfig,
		Verbose: true,
	}

	err := dbt.FetchTrustStore(tmpDir, true)
	if err != nil {
		log.Printf("Error fetching trust store: %s", err)
		t.Fail()
	}

	expected := testTruststore()
	trustPath := fmt.Sprintf("%s/%s", tmpDir, TruststorePath)

	if _, err := os.Stat(trustPath); os.IsNotExist(err) {
		log.Printf("File not written")
		t.Fail()
	}

	actualBytes, err := ioutil.ReadFile(trustPath)
	if err != nil {
		log.Printf("Error reading trust store: %s", err)
		t.Fail()
	}

	actual := string(actualBytes)

	assert.Equal(t, expected, actual, "Read truststore contents matches expectations.")
}

func TestDbtIsCurrent(t *testing.T) {
	dbt := &DBT{
		Config:  dbtConfig,
		Verbose: true,
	}

	targetDir := fmt.Sprintf("%s/%s", tmpDir, ToolDir)
	fileUrl := fmt.Sprintf("%s/1.2.2/%s/amd64/dbt", testDbtUrl(port), runtime.GOOS)
	fileName := fmt.Sprintf("%s/dbt", targetDir)

	err := dbt.FetchFile(fileUrl, fileName)
	if err != nil {
		fmt.Printf("Error fetching file %q: %s\n", fileUrl, err)
		t.Fail()
	}

	ok, err := dbt.IsCurrent(fileName)
	if err != nil {
		fmt.Printf("error checking to see if download file is current: %s\n", err)
		t.Fail()
	}

	assert.False(t, ok, "Old version should not show up as current.")

	fileUrl = fmt.Sprintf("%s/1.2.3/%s/amd64/dbt", testDbtUrl(port), runtime.GOOS)
	fileName = fmt.Sprintf("%s/dbt", targetDir)

	err = dbt.FetchFile(fileUrl, fileName)
	if err != nil {
		fmt.Printf("Error fetching file %q: %s\n", fileUrl, err)
		t.Fail()
	}

	ok, err = dbt.IsCurrent(fileName)
	if err != nil {
		fmt.Printf("error checking to see if download file is current: %s\n", err)
		t.Fail()
	}

	assert.True(t, ok, "Current version shows current.")
}

func TestDbtUpgradeInPlace(t *testing.T) {
	dbt := &DBT{
		Config:  dbtConfig,
		Verbose: true,
	}

	targetDir := fmt.Sprintf("%s/%s", tmpDir, ToolDir)
	fileUrl := fmt.Sprintf("%s/1.2.2/%s/amd64/dbt", testDbtUrl(port), runtime.GOOS)
	fileName := fmt.Sprintf("%s/dbt", targetDir)

	err := dbt.FetchFile(fileUrl, fileName)
	if err != nil {
		fmt.Printf("Error fetching file %q: %s\n", fileUrl, err)
		t.Fail()
	}

	ok, err := dbt.IsCurrent(fileName)
	if err != nil {
		fmt.Printf("error checking to see if download file is current: %s\n", err)
		t.Fail()
	}

	assert.False(t, ok, "Old version should not show up as current.")

	err = dbt.UpgradeInPlace(fileName)
	if err != nil {
		fmt.Printf("Error upgrading in place: %s", err)
		t.Fail()
	}

	ok, err = dbt.IsCurrent(fileName)
	if err != nil {
		fmt.Printf("error checking to see if download file is current: %s\n", err)
		t.Fail()
	}

	assert.True(t, ok, "Current version shows current.")
}

func TestNewDbt(t *testing.T) {
	homedir, err := GetHomeDir()
	if err != nil {
		fmt.Printf("Error getting homedir: %s", err)
		t.Fail()
	}

	configPath := fmt.Sprintf("%s/%s", homedir, ConfigDir)
	fileName := fmt.Sprintf("%s/dbt.json", configPath)

	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		fmt.Printf("Writing test dbt config to %s", fileName)
		err = GenerateDbtDir("", true)
		if err != nil {
			fmt.Printf("Error generating dbt dir: %s", err)
			t.Fail()
		}

		err = ioutil.WriteFile(fileName, []byte(testDbtConfigContents(port)), 0644)
		if err != nil {
			log.Printf("Error writing config file to %s: %s", fileName, err)
			t.Fail()
		}
	}

	_, err = NewDbt()
	if err != nil {
		fmt.Printf("Error creating DBT object: %s", err)
		t.Fail()
	}
}

func TestGetHomeDir(t *testing.T) {
	_, err := GetHomeDir()
	if err != nil {
		fmt.Printf("Error getting homedir: %s", err)
		t.Fail()
	}
}

func TestRunTool(t *testing.T) {
	dbt := &DBT{
		Config:  dbtConfig,
		Verbose: true,
	}

	script := `#!/bin/bash
echo "foo"
`

	sig := `-----BEGIN PGP SIGNATURE-----
  
iQFABAABCAAqFiEE3Ww86tgfSQ9lgLSizmhGNf2l0x8FAl3nXyUMHGRidEBkYnQu
Y29tAAoJEM5oRjX9pdMf49cIAKXlHna+QX8NZirDmqJkHg/SQXfSSwSpSVBxtD/B
lcgiERJLRy9yUUOxj9mF7uY+0l2Q0N9tqH+ZsqI8T0o6rOw3m9fpRymWhtvZkn/3
TUGYqXtllm9N5H/XCXm/GmRhS/nwSU/dxt8uEOMxbOGeNoEnSvRLX6UUBe5lzdbQ
p05JqgbJHm7Im/xjqvXeiCkhO6LsiH44PA7fn82XczUExiFf29YbqSxoaTFbNUml
EAIt0IfO16Jj6BfZiqlAdklK6gvyRyMIkQrSwXG0Umb2dPlJjz1x+DCbruUqnQX7
CP+c4NMnm7ZH7Ap+pII6ZPHdc5KxJNWh6ZVioY7EUINJKZk=
=/zev
-----END PGP SIGNATURE-----`

	fileName := fmt.Sprintf("%s/%s/bar", tmpDir, ToolDir)
	checksumFile := fmt.Sprintf("%s/%s/bar.sha256", tmpDir, ToolDir)
	sigFile := fmt.Sprintf("%s/%s/bar.asc", tmpDir, ToolDir)

	err := ioutil.WriteFile(fileName, []byte(script), 0755)
	if err != nil {
		fmt.Printf("Error writing test file: %s", err)
		t.Fail()
	}

	checksum, err := FileSha256(fileName)
	if err != nil {
		fmt.Printf("Error checksumming test file: %s", err)
		t.Fail()
	}

	err = ioutil.WriteFile(checksumFile, []byte(checksum), 0644)
	if err != nil {
		fmt.Printf("Error writing checksum file: %s", err)
		t.Fail()
	}

	err = ioutil.WriteFile(sigFile, []byte(sig), 0644)

	testExec = true

	err = dbt.RunTool("", []string{"bar"}, tmpDir, true)
	if err != nil {
		fmt.Printf("Error running test tool: %s", err)
		t.Fail()
	}

}
