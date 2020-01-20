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
				t.Errorf("Failed to fetch %s: %s", f.Name, err)
			}

			assert.True(t, resp.StatusCode < 300, "Non success error code fetching %s (%d)", url, resp.StatusCode)
		})
	}
}

func TestGenerateDbtDir(t *testing.T) {
	dbtDirPath := fmt.Sprintf("%s/%s", tmpDir, DbtDir)

	if _, err := os.Stat(dbtDirPath); os.IsNotExist(err) {
		t.Errorf("dbt dir %s did not create as expected", dbtDirPath)
	}

	trustPath := fmt.Sprintf("%s/%s", tmpDir, TrustDir)

	if _, err := os.Stat(trustPath); os.IsNotExist(err) {
		t.Errorf("trust dir %s did not create as expected", trustPath)
	}

	toolPath := fmt.Sprintf("%s/%s", tmpDir, ToolDir)
	if _, err := os.Stat(toolPath); os.IsNotExist(err) {
		t.Errorf("tool dir %s did not create as expected", toolPath)
	}

	configPath := fmt.Sprintf("%s/%s", tmpDir, ConfigDir)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config dir %s did not create as expected", configPath)
	}

}

func TestLoadDbtConfig(t *testing.T) {
	configPath := fmt.Sprintf("%s/%s", tmpDir, ConfigDir)
	fileName := fmt.Sprintf("%s/dbt.json", configPath)

	err := ioutil.WriteFile(fileName, []byte(testDbtConfigContents(port)), 0644)
	if err != nil {
		t.Errorf("Error writing config file to %s: %s", fileName, err)
	}

	expected := dbtConfig
	actual, err := LoadDbtConfig(tmpDir, true)
	if err != nil {
		t.Errorf("Error loading config file: %s", err)
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
		t.Errorf("Error fetching trust store: %s", err)
	}

	expected := trustfileContents
	trustPath := fmt.Sprintf("%s/%s", tmpDir, TruststorePath)

	if _, err := os.Stat(trustPath); os.IsNotExist(err) {
		t.Errorf("File not written")
	}

	actualBytes, err := ioutil.ReadFile(trustPath)
	if err != nil {
		t.Errorf("Error reading trust store: %s", err)
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
		t.Errorf("Error fetching file %q: %s\n", fileUrl, err)
	}

	ok, err := dbt.IsCurrent(fileName)
	if err != nil {
		t.Errorf("error checking to see if download file is current: %s\n", err)
	}

	assert.False(t, ok, "Old version should not show up as current.")

	fileUrl = fmt.Sprintf("http://127.0.0.1:%d/dbt/%s/%s/amd64/dbt", port, VERSION, runtime.GOOS)
	fileName = fmt.Sprintf("%s/dbt", targetDir)

	err = dbt.FetchFile(fileUrl, fileName)
	if err != nil {
		t.Errorf("Error fetching file %q: %s\n", fileUrl, err)
	}

	ok, err = dbt.IsCurrent(fileName)
	if err != nil {
		t.Errorf("error checking to see if download file is current: %s\n", err)
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
		t.Errorf("Error fetching file %q: %s\n", fileUrl, err)
	}

	ok, err := dbt.IsCurrent(fileName)
	if err != nil {
		t.Errorf("error checking to see if download file is current: %s\n", err)
	}

	assert.False(t, ok, "Old version should not show up as current.")

	err = dbt.UpgradeInPlace(fileName)
	if err != nil {
		t.Errorf("Error upgrading in place: %s", err)
	}

	ok, err = dbt.IsCurrent(fileName)
	if err != nil {
		t.Errorf("error checking to see if download file is current: %s\n", err)
	}

	assert.True(t, ok, "Current version shows current.")
}

func TestNewDbt(t *testing.T) {
	homedir, err := GetHomeDir()
	if err != nil {
		t.Errorf("Error getting homedir: %s", err)
	}

	configPath := fmt.Sprintf("%s/%s", homedir, ConfigDir)
	fileName := fmt.Sprintf("%s/dbt.json", configPath)

	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		fmt.Printf("Writing test dbt config to %s", fileName)
		err = GenerateDbtDir("", true)
		if err != nil {
			t.Errorf("Error generating dbt dir: %s", err)
		}

		err = ioutil.WriteFile(fileName, []byte(testDbtConfigContents(port)), 0644)
		if err != nil {
			t.Errorf("Error writing config file to %s: %s", fileName, err)
		}
	}

	_, err = NewDbt()
	if err != nil {
		t.Errorf("Error creating DBT object: %s", err)
	}
}

func TestGetHomeDir(t *testing.T) {
	_, err := GetHomeDir()
	if err != nil {
		t.Errorf("Error getting homedir: %s", err)
	}
}

func TestRunTool(t *testing.T) {
	dbt := &DBT{
		Config:  dbtConfig,
		Verbose: true,
		Logger:  log.New(os.Stderr, "", 0),
	}

	err := dbt.RunTool("", []string{"catalog", "list"}, tmpDir, false)
	if err != nil {
		t.Errorf("Error running test tool: %s", err)
	}
}
