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
	"net/http"
	"os"
	"runtime"
	"testing"
)

func TestRunRepoServer(t *testing.T) {
	for _, f := range testFilesB {
		t.Run(f.Name, func(t *testing.T) {
			url := fmt.Sprintf("%s%s", testHost, f.UrlPath)

			resp, err := http.Get(url)
			if err != nil {
				fmt.Printf("Failed to fetch %s: %s", f.Name, err)
				t.Fail()
			}

			assert.True(t, resp.StatusCode < 300, "Non success error code fetching %s (%d)", url, resp.StatusCode)
		})
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

	expected := dbtConfig
	actual, err := LoadDbtConfig(tmpDir, true)
	if err != nil {
		log.Printf("Error loading config file: %s", err)
		t.Fail()
	}

	assert.Equal(t, expected, actual, "Parsed config meets expectations")

}

func TestFetchTrustStore(t *testing.T) {
	dbt := &DBT{
		Config:  dbtConfig,
		Verbose: true,
	}

	err := dbt.FetchTrustStore(tmpDir, true)
	if err != nil {
		log.Printf("Error fetching trust store: %s", err)
		t.Fail()
	}

	expected := trustfileContents
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
	fileUrl := fmt.Sprintf("http://127.0.0.1:%d/dbt/%s/%s/amd64/dbt", port, oldVersion, runtime.GOOS)
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

	fileUrl = fmt.Sprintf("http://127.0.0.1:%d/dbt/%s/%s/amd64/dbt", port, VERSION, runtime.GOOS)
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
	fileUrl := fmt.Sprintf("%s/%s/%s/amd64/dbt", testDbtUrl(port), oldVersion, runtime.GOOS)
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

	err := dbt.RunTool("", []string{"catalog", "list"}, tmpDir, true)
	if err != nil {
		fmt.Printf("Error running test tool: %s", err)
		t.Fail()
	}
}
