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
	"github.com/nikogura/gomason/pkg/gomason"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var tmpDir string
var dbtConfig Config
var port int
var repo DBTRepoServer
var testHost string
var trustfileContents string

type testFile struct {
	Name     string
	FilePath string
	UrlPath  string
	TestUrl  string
}

var testFiles []*testFile
var testFileMap map[string]*testFile

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

	//tr := TestRepo{}

	//go tr.Run(port)

	err = buildTestRepo()
	if err != nil {
		log.Fatalf("Error building test repo: %s", err)
	}

	// Set up the repo server
	repo = DBTRepoServer{
		Address:    "127.0.0.1",
		Port:       port,
		ServerRoot: fmt.Sprintf("%s/repo", tmpDir),
		PubkeyFunc: nil,
	}

	// Run it in the background
	go repo.RunRepoServer()

	// Give things a moment to come up.
	time.Sleep(time.Second)

	testHost = fmt.Sprintf("http://%s:%d", repo.Address, repo.Port)
	fmt.Sprintf("--- Serving requests on %s ---\n", testHost)

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

func testDbtConfigContents(port int) string {
	return fmt.Sprintf(`{
  "dbt": {
    "repository": "http://127.0.0.1:%d/dbt",
    "truststore": "http://127.0.0.1:%d/dbt/truststore"
  },
  "tools": {
    "repository": "http://127.0.0.1:%d/dbt-tools"
  }
}`, port, port, port)
}

func testDbtConfig(port int) Config {
	return Config{
		Dbt: DbtConfig{
			Repo:       fmt.Sprintf("http://127.0.0.1:%d/dbt", port),
			TrustStore: fmt.Sprintf("http://127.0.0.1:%d/dbt/truststore", port),
		},
		Tools: ToolsConfig{
			Repo: fmt.Sprintf("http://127.0.0.1:%d/dbt-tools", port),
		},
		Username: "",
		Password: "",
	}
}

func testDbtUrl(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/dbt", port)
}

func testToolUrl(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/dbt-tools", port)
}

func createTestFiles(toolRoot string, version string, dbtRoot string) {
	testFileMap = make(map[string]*testFile)
	testFiles = []*testFile{
		{
			Name:     "boilerplate-description.txt",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/description.txt", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/description.txt", version),
		},
		{
			Name:     "boilerplate-description.txt.asc",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/description.txt.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/description.txt.asc", version),
		},
		{
			Name:     "boilerplate_darwin_amd64",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/darwin/amd64/boilerplate", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/darwin/amd64/boilerplate", version),
		},
		{
			Name:     "boilerplate_darwin_amd64.asc",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/darwin/amd64/boilerplate.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/darwin/amd64/boilerplate.asc", version),
		},
		{
			Name:     "boilerplate_linux_amd64",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/linux/amd64/boilerplate", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/linux/amd64/boilerplate", version),
		},
		{
			Name:     "boilerplate_linux_amd64.asc",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/linux/amd64/boilerplate.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/darwin/amd64/boilerplate.asc", version),
		},
		{
			Name:     "catalog-description.txt",
			FilePath: fmt.Sprintf("%s/catalog/%s/description.txt", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/description.txt", version),
		},
		{
			Name:     "catalog-description.txt.asc",
			FilePath: fmt.Sprintf("%s/catalog/%s/description.txt.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/description.txt.asc", version),
		},
		{
			Name:     "catalog_darwin_amd64",
			FilePath: fmt.Sprintf("%s/catalog/%s/darwin/amd64/catalog", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/darwin/amd64/catalog", version),
		},
		{
			Name:     "catalog_darwin_amd64.asc",
			FilePath: fmt.Sprintf("%s/catalog/%s/darwin/amd64/catalog.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/darwin/amd64/catalog.asc", version),
		},
		{
			Name:     "catalog_linux_amd64",
			FilePath: fmt.Sprintf("%s/catalog/%s/linux/amd64/catalog", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/linux/amd64/catalog", version),
		},
		{
			Name:     "catalog_linux_amd64.asc",
			FilePath: fmt.Sprintf("%s/catalog/%s/linux/amd64/catalog.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/linux/amd64/catalog.asc", version),
		},
		{
			Name:     "reposerver-description.txt",
			FilePath: fmt.Sprintf("%s/reposerver/%s/description.txt", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/description.txt", version),
		},
		{
			Name:     "reposerver-description.txt.asc",
			FilePath: fmt.Sprintf("%s/reposerver/%s/description.txt.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/description.txt.asc", version),
		},
		{
			Name:     "reposerver_darwin_amd64",
			FilePath: fmt.Sprintf("%s/reposerver/%s/darwin/amd64/reposerver", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/darwin/amd64/reposerver", version),
		},
		{
			Name:     "reposerver_darwin_amd64.asc",
			FilePath: fmt.Sprintf("%s/reposerver/%s/darwin/amd64/reposerver.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/darwin/amd64/reposerver.asc", version),
		},
		{
			Name:     "reposerver_linux_amd64",
			FilePath: fmt.Sprintf("%s/reposerver/%s/linux/amd64/reposerver", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/linux/amd64/reposerver", version),
		},
		{
			Name:     "reposerver_linux_amd64.asc",
			FilePath: fmt.Sprintf("%s/reposerver/%s/linux/amd64/reposerver.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/darwin/amd64/reposerver.asc", version),
		},
		{
			Name:     "dbt_darwin_amd64",
			FilePath: fmt.Sprintf("%s/%s/darwin/amd64/dbt", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/darwin/amd64/dbt", version),
		},
		{
			Name:     "dbt_darwin_amd64.asc",
			FilePath: fmt.Sprintf("%s/%s/darwin/amd64/dbt.asc", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/darwin/amd64/dbt.asc", version),
		},
		{
			Name:     "dbt_linux_amd64",
			FilePath: fmt.Sprintf("%s/%s/linux/amd64/dbt", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/linux/amd64/dbt", version),
		},
		{
			Name:     "dbt_linux_amd64.asc",
			FilePath: fmt.Sprintf("%s/%s/linux/amd64/dbt.asc", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/linux/amd64/dbt.asc", version),
		},
		{
			Name:     "install_dbt.sh",
			FilePath: fmt.Sprintf("%s/install_dbt.sh", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh",
		},
		{
			Name:     "install_dbt.sh.asc",
			FilePath: fmt.Sprintf("%s/install_dbt.sh.asc", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh.asc",
		},
		{
			Name:     "install_dbt_mac_keychain.sh",
			FilePath: fmt.Sprintf("%s/install_dbt_mac_keychain.sh", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh",
		},
		{
			Name:     "install_dbt_mac_keychain.sh.asc",
			FilePath: fmt.Sprintf("%s/install_dbt_mac_keychain.sh.asc", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh.asc",
		},
	}

	hostname := "127.0.0.1"
	for _, f := range testFiles {
		f.TestUrl = fmt.Sprintf("http://%s:%d%s", hostname, port, f.UrlPath)
		testFileMap[f.Name] = f
	}

	fmt.Printf("--- Created %d Test Files ---\n", len(testFiles))
}

func buildTestRepo() (err error) {
	dbtRoot := fmt.Sprintf("%s/repo/dbt", tmpDir)
	trustFile := fmt.Sprintf("%s/truststore", dbtRoot)
	toolRoot := fmt.Sprintf("%s/repo/dbt-tools", tmpDir)
	version := VERSION

	os.Setenv("GOMASON_NO_USER_CONFIG", "true")

	createTestFiles(toolRoot, version, dbtRoot)

	// set up test keys
	keyring := filepath.Join(tmpDir, "keyring.gpg")
	trustdb := filepath.Join(tmpDir, "trustdb.gpg")

	// write gpg batch file
	defaultKeyText := `%echo Generating a default key
%no-protection
%transient-key
Key-Type: default
Subkey-Type: default
Name-Real: Gomason Tester
Name-Comment: with no passphrase
Name-Email: tester@nikogura.com
Expire-Date: 0
%commit
%echo done
`

	err = os.MkdirAll(dbtRoot, 0755)
	if err != nil {
		log.Fatalf("Error building %s: %s", dbtRoot, err)
	}

	err = os.MkdirAll(toolRoot, 0755)
	if err != nil {
		log.Fatalf("Error building %s: %s", toolRoot, err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %s", err)
	}

	meta, err := gomason.ReadMetadata("../../metadata.json")
	if err != nil {
		log.Fatalf("couldn't read package information from metadata.json: %s", err)
	}

	meta.Options = make(map[string]interface{})
	meta.Options["keyring"] = keyring
	meta.Options["trustdb"] = trustdb
	meta.SignInfo = gomason.SignInfo{
		Program: "gpg",
		Email:   "tester@nikogura.com",
	}

	gpg, err := exec.LookPath("gpg")
	if err != nil {
		log.Fatalf("Failed to check if gpg is installed:%s", err)
	}
	keyFile := filepath.Join(tmpDir, "testkey")
	err = ioutil.WriteFile(keyFile, []byte(defaultKeyText), 0644)
	if err != nil {
		log.Fatalf("Error writing test key generation file: %s", err)
	}

	log.Printf("Keyring file: %s", keyring)
	log.Printf("Trustdb file: %s", trustdb)
	log.Printf("DBT truststore: %s", trustFile)
	log.Printf("Test key generation file: %s", keyFile)

	// generate a test key
	cmd := exec.Command(gpg, "--trustdb", trustdb, "--no-default-keyring", "--keyring", keyring, "--batch", "--generate-key", keyFile)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("****** Error creating test key: %s *****", err)
	}

	// write out truststore
	cmd = exec.Command(gpg, "--keyring", meta.Options["keyring"].(string), "--export", "-a", "tester@nikogura.com")

	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("Error exporting public key: %s", err)
	}

	trustfileContents = string(out)

	err = ioutil.WriteFile(trustFile, out, 0644)
	if err != nil {
		log.Fatalf("Error writing truststore file %s: %s", trustFile, err)
	}

	log.Printf("Done creating keyring and test keys")

	lang, err := gomason.GetByName(meta.GetLanguage())
	if err != nil {
		log.Fatalf("Invalid language: %v", err)
	}

	workDir, err := lang.CreateWorkDir(tmpDir)
	if err != nil {
		log.Fatalf("Failed to create ephemeral workDir: %s", err)
	}

	// Normally with gomason we'd check it out from VCS.  In this case, I want to build *THIS* version

	src := strings.TrimRight(cwd, "/pkg/dbt")
	dst := fmt.Sprintf("%s/src/github.com/nikogura", workDir)
	err = os.MkdirAll(dst, 0755)
	if err != nil {
		log.Fatalf("Failed creating directory %s: %s", dst, err)
	}

	err = DirCopy(src, dst)
	if err != nil {
		log.Fatalf("Failed copying directory %s to %s: %s", src, dst, err)
	}

	err = lang.Build(workDir, meta)
	if err != nil {
		log.Fatalf("build failed: %s", err)
	}

	err = gomason.HandleArtifacts(meta, workDir, cwd, true, false, true)
	if err != nil {
		log.Fatalf("signing failed: %s", err)
	}

	err = gomason.HandleExtras(meta, workDir, cwd, true, false)
	if err != nil {
		log.Fatalf("Extra artifact processing failed: %s", err)
	}

	// Write the files into place
	for _, f := range testFiles {
		src := fmt.Sprintf("%s/%s", cwd, f.Name)
		dir := filepath.Dir(f.FilePath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err = os.MkdirAll(dir, 0755)
			if err != nil {
				log.Fatalf("Error creating dir %s: %s", dir, err)
			}
		}

		input, err := ioutil.ReadFile(src)
		if err != nil {
			log.Fatalf("Failed to read file %s: %s", src, err)
		}

		err = ioutil.WriteFile(f.FilePath, input, 0644)
		if err != nil {
			log.Fatalf("Failed to write file %s: %s", f.FilePath, err)
		}

		err = os.Remove(src)
		if err != nil {
			log.Fatalf("Failed to remove file %s: %s", src, err)
		}

		checksum, err := FileSha256(f.FilePath)
		if err != nil {
			log.Fatalf("Failed to checksum file %s: %s", f.FilePath, err)
		}

		checksumFile := fmt.Sprintf("%s.sha256", f.FilePath)

		err = ioutil.WriteFile(checksumFile, []byte(checksum), 0644)
		if err != nil {
			log.Fatalf("Failed to write %s: %s", checksumFile, err)
		}
	}

	return err
}

func TestRunRepoServer(t *testing.T) {
	for _, f := range testFiles {
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

	expected := testDbtConfig(port)
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

//func TestDbtIsCurrent(t *testing.T) {
//	dbt := &DBT{
//		Config:  dbtConfig,
//		Verbose: true,
//	}
//
//	targetDir := fmt.Sprintf("%s/%s", tmpDir, ToolDir)
//	fileUrl := fmt.Sprintf("http://127.0.0.1:%d/dbt/1.2.2/%s/amd64/dbt", port, runtime.GOOS)
//	fileName := fmt.Sprintf("%s/dbt", targetDir)
//
//	err := dbt.FetchFile(fileUrl, fileName)
//	if err != nil {
//		fmt.Printf("Error fetching file %q: %s\n", fileUrl, err)
//		t.Fail()
//	}
//
//	ok, err := dbt.IsCurrent(fileName)
//	if err != nil {
//		fmt.Printf("error checking to see if download file is current: %s\n", err)
//		t.Fail()
//	}
//
//	assert.False(t, ok, "Old version should not show up as current.")
//
//	fileUrl = fmt.Sprintf("http://127.0.0.1:%d/dbt/%s/%s/amd64/dbt", port, VERSION, runtime.GOOS)
//	fileName = fmt.Sprintf("%s/dbt", targetDir)
//
//	err = dbt.FetchFile(fileUrl, fileName)
//	if err != nil {
//		fmt.Printf("Error fetching file %q: %s\n", fileUrl, err)
//		t.Fail()
//	}
//
//	ok, err = dbt.IsCurrent(fileName)
//	if err != nil {
//		fmt.Printf("error checking to see if download file is current: %s\n", err)
//		t.Fail()
//	}
//
//	assert.True(t, ok, "Current version shows current.")
//}

//func TestDbtUpgradeInPlace(t *testing.T) {
//	dbt := &DBT{
//		Config:  dbtConfig,
//		Verbose: true,
//	}
//
//	targetDir := fmt.Sprintf("%s/%s", tmpDir, ToolDir)
//	fileUrl := fmt.Sprintf("%s/1.2.2/%s/amd64/dbt", testDbtUrl(port), runtime.GOOS)
//	fileName := fmt.Sprintf("%s/dbt", targetDir)
//
//	err := dbt.FetchFile(fileUrl, fileName)
//	if err != nil {
//		fmt.Printf("Error fetching file %q: %s\n", fileUrl, err)
//		t.Fail()
//	}
//
//	ok, err := dbt.IsCurrent(fileName)
//	if err != nil {
//		fmt.Printf("error checking to see if download file is current: %s\n", err)
//		t.Fail()
//	}
//
//	assert.False(t, ok, "Old version should not show up as current.")
//
//	err = dbt.UpgradeInPlace(fileName)
//	if err != nil {
//		fmt.Printf("Error upgrading in place: %s", err)
//		t.Fail()
//	}
//
//	ok, err = dbt.IsCurrent(fileName)
//	if err != nil {
//		fmt.Printf("error checking to see if download file is current: %s\n", err)
//		t.Fail()
//	}
//
//	assert.True(t, ok, "Current version shows current.")
//}

//func TestNewDbt(t *testing.T) {
//	homedir, err := GetHomeDir()
//	if err != nil {
//		fmt.Printf("Error getting homedir: %s", err)
//		t.Fail()
//	}
//
//	configPath := fmt.Sprintf("%s/%s", homedir, ConfigDir)
//	fileName := fmt.Sprintf("%s/dbt.json", configPath)
//
//	if _, err := os.Stat(fileName); os.IsNotExist(err) {
//		fmt.Printf("Writing test dbt config to %s", fileName)
//		err = GenerateDbtDir("", true)
//		if err != nil {
//			fmt.Printf("Error generating dbt dir: %s", err)
//			t.Fail()
//		}
//
//		err = ioutil.WriteFile(fileName, []byte(testDbtConfigContents(port)), 0644)
//		if err != nil {
//			log.Printf("Error writing config file to %s: %s", fileName, err)
//			t.Fail()
//		}
//	}
//
//	_, err = NewDbt()
//	if err != nil {
//		fmt.Printf("Error creating DBT object: %s", err)
//		t.Fail()
//	}
//}

func TestGetHomeDir(t *testing.T) {
	_, err := GetHomeDir()
	if err != nil {
		fmt.Printf("Error getting homedir: %s", err)
		t.Fail()
	}
}

//func TestRunTool(t *testing.T) {
//	dbt := &DBT{
//		Config:  dbtConfig,
//		Verbose: true,
//	}
//
//	script := `#!/bin/bash
//echo "foo"
//`
//
//	sig := `-----BEGIN PGP SIGNATURE-----
//
//iQFABAABCAAqFiEE3Ww86tgfSQ9lgLSizmhGNf2l0x8FAl3nXyUMHGRidEBkYnQu
//Y29tAAoJEM5oRjX9pdMf49cIAKXlHna+QX8NZirDmqJkHg/SQXfSSwSpSVBxtD/B
//lcgiERJLRy9yUUOxj9mF7uY+0l2Q0N9tqH+ZsqI8T0o6rOw3m9fpRymWhtvZkn/3
//TUGYqXtllm9N5H/XCXm/GmRhS/nwSU/dxt8uEOMxbOGeNoEnSvRLX6UUBe5lzdbQ
//p05JqgbJHm7Im/xjqvXeiCkhO6LsiH44PA7fn82XczUExiFf29YbqSxoaTFbNUml
//EAIt0IfO16Jj6BfZiqlAdklK6gvyRyMIkQrSwXG0Umb2dPlJjz1x+DCbruUqnQX7
//CP+c4NMnm7ZH7Ap+pII6ZPHdc5KxJNWh6ZVioY7EUINJKZk=
//=/zev
//-----END PGP SIGNATURE-----`
//
//	fileName := fmt.Sprintf("%s/%s/bar", tmpDir, ToolDir)
//	checksumFile := fmt.Sprintf("%s/%s/bar.sha256", tmpDir, ToolDir)
//	sigFile := fmt.Sprintf("%s/%s/bar.asc", tmpDir, ToolDir)
//
//	err := ioutil.WriteFile(fileName, []byte(script), 0755)
//	if err != nil {
//		fmt.Printf("Error writing test file: %s", err)
//		t.Fail()
//	}
//
//	checksum, err := FileSha256(fileName)
//	if err != nil {
//		fmt.Printf("Error checksumming test file: %s", err)
//		t.Fail()
//	}
//
//	err = ioutil.WriteFile(checksumFile, []byte(checksum), 0644)
//	if err != nil {
//		fmt.Printf("Error writing checksum file: %s", err)
//		t.Fail()
//	}
//
//	err = ioutil.WriteFile(sigFile, []byte(sig), 0644)
//
//	testExec = true
//
//	err = dbt.RunTool("", []string{"bar"}, tmpDir, true)
//	if err != nil {
//		fmt.Printf("Error running test tool: %s", err)
//		t.Fail()
//	}
//
//}
