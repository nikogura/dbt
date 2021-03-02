// Copyright © 2020 Nik Ogura <nik.ogura@gmail.com>
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
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/nikogura/gomason/pkg/gomason"
	"github.com/phayes/freeport"
	"io/ioutil"
	"log"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPackageGroup Github group owning this codebase.  Used to compile itself as part of it's test suite.  If you're not me, you'll want to change this in your fork.
const TestPackageGroup = "nikogura"

var tmpDir string
var sourceDirA string
var sourceDirB string
var dbtConfig Config
var s3DbtConfig Config
var port int
var repo DBTRepoServer
var testHost string
var trustfileContents string
var dbtRoot string
var toolRoot string
var trustFile string
var setup bool
var oldVersion = "3.0.2"
var testServer *httptest.Server
var s3Config *aws.Config
var s3Session *session.Session
var s3Backend *s3mem.Backend
var faker *gofakes3.GoFakeS3
var homeDirRepoServer string
var homeDirS3 string

type testFile struct {
	Name     string
	FilePath string
	UrlPath  string
	TestUrl  string
	Repo     string
}

var testFilesA map[string]*testFile
var testFilesB map[string]*testFile

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

	NOPROGRESS = true

	tmpDir = dir
	homeDirRepoServer = fmt.Sprintf("%s/homedirReposerver", tmpDir)
	homeDirS3 = fmt.Sprintf("%s/homeDirS3", tmpDir)
	sourceDirA = fmt.Sprintf("%s/sourceA", tmpDir)
	sourceDirB = fmt.Sprintf("%s/sourceB", tmpDir)
	fmt.Printf("Temp Dir: %s\n", tmpDir)
	fmt.Printf("Homedir Reposerver: %s\n", homeDirRepoServer)
	fmt.Printf("Homedir S3: %s\n", homeDirS3)
	fmt.Printf("Source A Dir: %s\n", sourceDirA)
	fmt.Printf("Source B Dir: %s\n", sourceDirB)

	dbtRoot = fmt.Sprintf("%s/repo/dbt", tmpDir)
	trustFile = fmt.Sprintf("%s/truststore", dbtRoot)
	toolRoot = fmt.Sprintf("%s/repo/dbt-tools", tmpDir)

	freePort, err := freeport.GetFreePort()
	if err != nil {
		log.Printf("Error getting a free port: %s", err)
		os.Exit(1)
	}

	port = freePort

	fmt.Printf("-- Creating Version A Test Files ---\n")
	testFilesA = createTestFilesA(toolRoot, oldVersion, dbtRoot)
	fmt.Printf("--- Created %d Test Files ---\n", len(testFilesA))

	fmt.Printf("-- Creating Version B Test Files ---\n")
	testFilesB = createTestFilesB(toolRoot, VERSION, dbtRoot)
	fmt.Printf("--- Created %d Test Files ---\n", len(testFilesB))

	// Dbt config for the built in repo server
	dbtConfig = Config{
		Dbt: DbtConfig{
			Repo:       fmt.Sprintf("http://127.0.0.1:%d/dbt", port),
			TrustStore: fmt.Sprintf("http://127.0.0.1:%d/dbt/truststore", port),
		},
		Tools: ToolsConfig{
			Repo: fmt.Sprintf("http://127.0.0.1:%d/dbt-tools", port),
		},
		//Username: "",
		//Password: "",
	}

	// Dbt config for using s3 as a repo
	s3DbtConfig = Config{
		Dbt: DbtConfig{
			Repo:       "https://dbt.s3.us-east-1.amazonaws.com",
			TrustStore: "https://dbt.s3.us-east-1.amazonaws.com/truststore",
		},
		Tools: ToolsConfig{
			Repo: "https://dbt-tools.s3.us-east-1.amazonaws.com",
		},
	}

	if !setup {
		// Setup Fake S3
		s3Backend = s3mem.New()
		faker = gofakes3.New(s3Backend)
		testServer = httptest.NewServer(faker.Server())

		// S3 config
		s3Config = &aws.Config{
			Credentials:      credentials.NewStaticCredentials("foo", "bar", ""),
			Endpoint:         aws.String(testServer.URL),
			Region:           aws.String("us-east-1"),
			DisableSSL:       aws.Bool(true),
			S3ForcePathStyle: aws.Bool(true),
		}
		s3Session, err = session.NewSession(s3Config)
		if err != nil {
			log.Fatalf("failed creating fake aws session: %s", err)
		}

		dbtBucket := "dbt"
		toolsBucket := "dbt-tools"

		s3Client := s3.New(s3Session)
		cparams := &s3.CreateBucketInput{Bucket: aws.String(dbtBucket)}

		_, err = s3Client.CreateBucket(cparams)
		if err != nil {
			log.Fatalf("Failed to create bucket %s: %s", dbtBucket, err)
		}

		cparams = &s3.CreateBucketInput{Bucket: aws.String(toolsBucket)}
		_, err = s3Client.CreateBucket(cparams)
		if err != nil {
			log.Fatalf("Failed to create bucket %s: %s", toolsBucket, err)
		}

		// actually build the test repo
		err = buildTestRepo()
		if err != nil {
			log.Fatalf("Error building test repo: %s", err)
		}

		// Set up the repo server
		repo = DBTRepoServer{
			Address:    "127.0.0.1",
			Port:       port,
			ServerRoot: fmt.Sprintf("%s/repo", tmpDir),
		}

		// Run test server in the background
		go repo.RunRepoServer()

		// Give things a moment to come up.
		time.Sleep(time.Second)

		testHost = fmt.Sprintf("http://%s:%d", repo.Address, repo.Port)
		fmt.Printf("--- Serving requests on %s ---\n", testHost)

		log.Printf("Sleeping for 1 second for the test artifact server to start up.")
		time.Sleep(time.Second * 1)

		configs := []struct {
			homedir string
			config  string
		}{
			{
				homeDirRepoServer,
				testDbtConfigContents(port),
			},
			{
				homeDirS3,
				testDbtConfigS3Contents(),
			},
		}

		for _, c := range configs {
			err = os.MkdirAll(c.homedir, 0755)
			if err != nil {
				log.Fatalf("Error generating fake home dir: %s", err)
			}

			err = GenerateDbtDir(c.homedir, true)
			if err != nil {
				log.Fatalf("Error generating dbt dir: %s", err)
			}

			configPath := fmt.Sprintf("%s/%s", c.homedir, ConfigDir)
			fileName := fmt.Sprintf("%s/dbt.json", configPath)

			err := ioutil.WriteFile(fileName, []byte(c.config), 0644)
			if err != nil {
				log.Fatalf("Error writing config file to %s: %s", fileName, err)
			}

		}

		setup = true
	}
}

func tearDown() {
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		_ = os.Remove(tmpDir)
	}

	testServer.Close()
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

func testDbtConfigS3Contents() string {
	return `{
  "dbt": {
    "repository": "https://dbt.s3.us-east-1.amazonaws.com",
    "truststore": "https://dbt.s3.us-east-1.amazonaws.com/truststore"
  },
  "tools": {
    "repository": "https://dbt-tools.s3.us-east-1.amazonaws.com"
  }
}`
}

func testDbtUrl(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/dbt", port)
}

func testToolUrl(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/dbt-tools", port)
}

func createTestFilesA(toolRoot string, version string, dbtRoot string) (testFiles map[string]*testFile) {
	testFiles = make(map[string]*testFile)
	files := []*testFile{
		{
			Name:     "boilerplate-description.txt",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/description.txt", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/description.txt", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "boilerplate-description.txt.asc",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/description.txt.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/description.txt.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "boilerplate_darwin_amd64",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/darwin/amd64/boilerplate", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/darwin/amd64/boilerplate", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "boilerplate_darwin_amd64.asc",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/darwin/amd64/boilerplate.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/darwin/amd64/boilerplate.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "boilerplate_linux_amd64",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/linux/amd64/boilerplate", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/linux/amd64/boilerplate", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "boilerplate_linux_amd64.asc",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/linux/amd64/boilerplate.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/darwin/amd64/boilerplate.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog-description.txt",
			FilePath: fmt.Sprintf("%s/catalog/%s/description.txt", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/description.txt", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog-description.txt.asc",
			FilePath: fmt.Sprintf("%s/catalog/%s/description.txt.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/description.txt.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog_darwin_amd64",
			FilePath: fmt.Sprintf("%s/catalog/%s/darwin/amd64/catalog", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/darwin/amd64/catalog", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog_darwin_amd64.asc",
			FilePath: fmt.Sprintf("%s/catalog/%s/darwin/amd64/catalog.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/darwin/amd64/catalog.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog_linux_amd64",
			FilePath: fmt.Sprintf("%s/catalog/%s/linux/amd64/catalog", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/linux/amd64/catalog", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog_linux_amd64.asc",
			FilePath: fmt.Sprintf("%s/catalog/%s/linux/amd64/catalog.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/linux/amd64/catalog.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "dbt_darwin_amd64",
			FilePath: fmt.Sprintf("%s/%s/darwin/amd64/dbt", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/darwin/amd64/dbt", version),
			Repo:     "dbt",
		},
		{
			Name:     "dbt_darwin_amd64.asc",
			FilePath: fmt.Sprintf("%s/%s/darwin/amd64/dbt.asc", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/darwin/amd64/dbt.asc", version),
			Repo:     "dbt",
		},
		{
			Name:     "dbt_linux_amd64",
			FilePath: fmt.Sprintf("%s/%s/linux/amd64/dbt", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/linux/amd64/dbt", version),
			Repo:     "dbt",
		},
		{
			Name:     "dbt_linux_amd64.asc",
			FilePath: fmt.Sprintf("%s/%s/linux/amd64/dbt.asc", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/linux/amd64/dbt.asc", version),
			Repo:     "dbt",
		},
		{
			Name:     "install_dbt.sh",
			FilePath: fmt.Sprintf("%s/install_dbt.sh", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh",
			Repo:     "dbt",
		},
		{
			Name:     "install_dbt.sh.asc",
			FilePath: fmt.Sprintf("%s/install_dbt.sh.asc", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh.asc",
			Repo:     "dbt",
		},
		{
			Name:     "install_dbt_mac_keychain.sh",
			FilePath: fmt.Sprintf("%s/install_dbt_mac_keychain.sh", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh",
			Repo:     "dbt",
		},
		{
			Name:     "install_dbt_mac_keychain.sh.asc",
			FilePath: fmt.Sprintf("%s/install_dbt_mac_keychain.sh.asc", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh.asc",
			Repo:     "dbt",
		},
	}

	hostname := "127.0.0.1"
	for _, f := range files {
		f.TestUrl = fmt.Sprintf("http://%s:%d%s", hostname, port, f.UrlPath)
		testFiles[f.Name] = f
	}

	return testFiles
}
func createTestFilesB(toolRoot string, version string, dbtRoot string) (testFiles map[string]*testFile) {
	testFiles = make(map[string]*testFile)
	files := []*testFile{
		{
			Name:     "boilerplate-description.txt",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/description.txt", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/description.txt", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "boilerplate-description.txt.asc",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/description.txt.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/description.txt.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "boilerplate_darwin_amd64",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/darwin/amd64/boilerplate", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/darwin/amd64/boilerplate", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "boilerplate_darwin_amd64.asc",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/darwin/amd64/boilerplate.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/darwin/amd64/boilerplate.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "boilerplate_linux_amd64",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/linux/amd64/boilerplate", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/linux/amd64/boilerplate", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "boilerplate_linux_amd64.asc",
			FilePath: fmt.Sprintf("%s/boilerplate/%s/linux/amd64/boilerplate.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/boilerplate/%s/darwin/amd64/boilerplate.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog-description.txt",
			FilePath: fmt.Sprintf("%s/catalog/%s/description.txt", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/description.txt", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog-description.txt.asc",
			FilePath: fmt.Sprintf("%s/catalog/%s/description.txt.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/description.txt.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog_darwin_amd64",
			FilePath: fmt.Sprintf("%s/catalog/%s/darwin/amd64/catalog", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/darwin/amd64/catalog", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog_darwin_amd64.asc",
			FilePath: fmt.Sprintf("%s/catalog/%s/darwin/amd64/catalog.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/darwin/amd64/catalog.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog_linux_amd64",
			FilePath: fmt.Sprintf("%s/catalog/%s/linux/amd64/catalog", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/linux/amd64/catalog", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "catalog_linux_amd64.asc",
			FilePath: fmt.Sprintf("%s/catalog/%s/linux/amd64/catalog.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/catalog/%s/linux/amd64/catalog.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "reposerver-description.txt",
			FilePath: fmt.Sprintf("%s/reposerver/%s/description.txt", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/description.txt", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "reposerver-description.txt.asc",
			FilePath: fmt.Sprintf("%s/reposerver/%s/description.txt.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/description.txt.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "reposerver_darwin_amd64",
			FilePath: fmt.Sprintf("%s/reposerver/%s/darwin/amd64/reposerver", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/darwin/amd64/reposerver", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "reposerver_darwin_amd64.asc",
			FilePath: fmt.Sprintf("%s/reposerver/%s/darwin/amd64/reposerver.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/darwin/amd64/reposerver.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "reposerver_linux_amd64",
			FilePath: fmt.Sprintf("%s/reposerver/%s/linux/amd64/reposerver", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/linux/amd64/reposerver", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "reposerver_linux_amd64.asc",
			FilePath: fmt.Sprintf("%s/reposerver/%s/linux/amd64/reposerver.asc", toolRoot, version),
			UrlPath:  fmt.Sprintf("/dbt-tools/reposerver/%s/darwin/amd64/reposerver.asc", version),
			Repo:     "dbt-tools",
		},
		{
			Name:     "dbt_darwin_amd64",
			FilePath: fmt.Sprintf("%s/%s/darwin/amd64/dbt", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/darwin/amd64/dbt", version),
			Repo:     "dbt",
		},
		{
			Name:     "dbt_darwin_amd64.asc",
			FilePath: fmt.Sprintf("%s/%s/darwin/amd64/dbt.asc", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/darwin/amd64/dbt.asc", version),
			Repo:     "dbt",
		},
		{
			Name:     "dbt_linux_amd64",
			FilePath: fmt.Sprintf("%s/%s/linux/amd64/dbt", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/linux/amd64/dbt", version),
			Repo:     "dbt",
		},
		{
			Name:     "dbt_linux_amd64.asc",
			FilePath: fmt.Sprintf("%s/%s/linux/amd64/dbt.asc", dbtRoot, version),
			UrlPath:  fmt.Sprintf("/dbt/%s/linux/amd64/dbt.asc", version),
			Repo:     "dbt",
		},
		{
			Name:     "install_dbt.sh",
			FilePath: fmt.Sprintf("%s/install_dbt.sh", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh",
			Repo:     "dbt",
		},
		{
			Name:     "install_dbt.sh.asc",
			FilePath: fmt.Sprintf("%s/install_dbt.sh.asc", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh.asc",
			Repo:     "dbt",
		},
		{
			Name:     "install_dbt_mac_keychain.sh",
			FilePath: fmt.Sprintf("%s/install_dbt_mac_keychain.sh", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh",
			Repo:     "dbt",
		},
		{
			Name:     "install_dbt_mac_keychain.sh.asc",
			FilePath: fmt.Sprintf("%s/install_dbt_mac_keychain.sh.asc", dbtRoot),
			UrlPath:  "/dbt/install_dbt.sh.asc",
			Repo:     "dbt",
		},
	}

	hostname := "127.0.0.1"
	for _, f := range files {
		f.TestUrl = fmt.Sprintf("http://%s:%d%s", hostname, port, f.UrlPath)
		testFiles[f.Name] = f
	}

	return testFiles
}

func createTestKeys(keyring string, trustdb string) {
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
	cmd = exec.Command(gpg, "--keyring", keyring, "--export", "-a", "tester@nikogura.com")

	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("Error exporting public key: %s", err)
	}

	trustfileContents = string(out)

	err = ioutil.WriteFile(trustFile, out, 0644)
	if err != nil {
		log.Fatalf("Error writing truststore file %s: %s", trustFile, err)
	}

	s3Client := s3.New(s3Session)

	// upload the file to the fake s3 endpoint
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String("dbt"),
		Key:    aws.String("truststore"),
		Body:   bytes.NewReader(out),
	})
	if err != nil {
		log.Fatalf("Failed to put truststore into fake s3: %s", err)
	}

	log.Printf("Done creating keyring and test keys")

}

func buildSource(meta gomason.Metadata, version string, sourceDir string, testfiles map[string]*testFile) {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %s", err)
	}

	gm := gomason.Gomason{Config: gomason.UserConfig{}}

	lang, err := gomason.GetByName(meta.GetLanguage())
	if err != nil {
		log.Fatalf("Invalid language: %v", err)
	}

	workDir, err := lang.CreateWorkDir(sourceDir)
	if err != nil {
		log.Fatalf("Failed to create ephemeral workDir: %s", err)
	}

	src := strings.TrimSuffix(cwd, "/dbt/pkg/dbt")
	dst := fmt.Sprintf("%s/src/github.com/%s", workDir, TestPackageGroup)
	err = os.MkdirAll(dst, 0755)
	if err != nil {
		log.Fatalf("Failed creating directory %s: %s", dst, err)
	}

	err = DirCopy(src, dst)
	if err != nil {
		log.Fatalf("Failed copying directory %s to %s: %s", src, dst, err)
	}

	if version != "" {
		_ = lang.Checkout(workDir, meta, version)
	}

	err = lang.Build(workDir, meta, "")
	if err != nil {
		log.Fatalf("build failed: %s", err)
	}

	err = gm.HandleArtifacts(meta, workDir, cwd, true, false, true, "")
	if err != nil {
		log.Fatalf("signing failed: %s", err)
	}

	err = gm.HandleExtras(meta, workDir, cwd, true, false)
	if err != nil {
		log.Fatalf("Extra artifact processing failed: %s", err)
	}

	fmt.Printf("--- Moving %d Test Files into repository ---\n", len(testFilesB))

	s3Client := s3.New(s3Session)

	// Write the files into place
	for _, f := range testfiles {
		fmt.Printf("Processing %s\n", f.Name)
		src := fmt.Sprintf("%s/%s", cwd, f.Name)
		dir := filepath.Dir(f.FilePath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err = os.MkdirAll(dir, 0755)
			if err != nil {
				log.Fatalf("Error creating dir %s: %s", dir, err)
			}
		}

		// read the file we compiled
		input, err := ioutil.ReadFile(src)
		if err != nil {
			log.Fatalf("Failed to read file %s: %s", src, err)
		}

		key := strings.TrimPrefix(f.UrlPath, fmt.Sprintf("/%s/", f.Repo))

		// upload the file to the fake s3 endpoint
		_, err = s3Client.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(f.Repo),
			Key:    aws.String(key),
			Body:   bytes.NewReader(input),
		})
		if err != nil {
			log.Fatalf("Failed to put %s into fake s3: %s", key, err)
		}

		// verify it got into the fake s3
		headOptions := &s3.HeadObjectInput{
			Bucket: aws.String(f.Repo),
			Key:    aws.String(key),
		}

		headSvc := s3.New(s3Session)

		_, err = headSvc.HeadObject(headOptions)
		if err != nil {
			log.Fatalf("failed to get metadata for %s: %s", f.Name, err)
		}

		// make the directory paths in s3
		dirs, err := DirsForURL(key)
		if err != nil {
			log.Fatalf("failed to parse dirs for %s", key)
		}

		// create the 'folders' (0 byte objects) in s3
		for _, d := range dirs {
			if d != "." {
				path := fmt.Sprintf("%s/", d)
				// check to see if it doesn't already exist
				headOptions = &s3.HeadObjectInput{
					Bucket: aws.String(f.Repo),
					Key:    aws.String(path),
				}

				_, err = headSvc.HeadObject(headOptions)
				// if there's an error, it doesn't exist
				if err != nil {
					// so create it
					_, err = s3Client.PutObject(&s3.PutObjectInput{
						Bucket: aws.String(f.Repo),
						Key:    aws.String(path),
					})
					if err != nil {
						log.Fatalf("Failed to put %s into fake s3: %s", key, err)
					}
				}
			}
		}

		// write the file into place in the test repo
		err = ioutil.WriteFile(f.FilePath, input, 0644)
		if err != nil {
			log.Fatalf("Failed to write file %s: %s", f.FilePath, err)
		}

		// clean up after ourselves
		err = os.Remove(src)
		if err != nil {
			log.Fatalf("Failed to remove file %s: %s", src, err)
		}

		// checksum the file
		checksum, err := FileSha256(f.FilePath)
		if err != nil {
			log.Fatalf("Failed to checksum file %s: %s", f.FilePath, err)
		}

		checksumFile := fmt.Sprintf("%s.sha256", f.FilePath)

		// write the file's checksum into the test repo
		err = ioutil.WriteFile(checksumFile, []byte(checksum), 0644)
		if err != nil {
			log.Fatalf("Failed to write %s: %s", checksumFile, err)
		}

		// upload the checksum to the fake s3 endpoint
		_, err = s3Client.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(f.Repo),
			Key:    aws.String(fmt.Sprintf("%s.sha256", key)),
			Body:   bytes.NewReader([]byte(checksum)),
		})
		if err != nil {
			log.Fatalf("Failed to put %s into fake s3: %s", key, err)
		}

		// verify it got into the fake s3
		headOptions = &s3.HeadObjectInput{
			Bucket: aws.String(f.Repo),
			Key:    aws.String(key),
		}

		_, err = headSvc.HeadObject(headOptions)
		if err != nil {
			log.Fatalf("failed to get metadata for %s: %s", f.Name, err)
		}
	}
}

func buildTestRepo() (err error) {
	_ = os.Setenv("GOMASON_NO_USER_CONFIG", "true")

	// set up test keys
	keyring := filepath.Join(tmpDir, "keyring.gpg")
	trustdb := filepath.Join(tmpDir, "trustdb.gpg")

	fmt.Printf("Creating dbt repo root at %s\n", dbtRoot)
	err = os.MkdirAll(dbtRoot, 0755)
	if err != nil {
		log.Fatalf("Error building %s: %s", dbtRoot, err)
	}

	fmt.Printf("Creating tool repo root at %s\n", toolRoot)
	err = os.MkdirAll(toolRoot, 0755)
	if err != nil {
		log.Fatalf("Error building %s: %s", toolRoot, err)
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

	createTestKeys(keyring, trustdb)

	buildSource(meta, "", sourceDirB, testFilesB)

	oldMetadataFile := fmt.Sprintf("pkg/dbt/testfixtures/metadata.3.0.2.json")

	oldMeta, err := gomason.ReadMetadata(oldMetadataFile)
	if err != nil {
		log.Fatalf("couldn't read package information from old metadatafile %s: %s", oldMetadataFile, err)
	}

	oldMeta.Options = make(map[string]interface{})
	oldMeta.Options["keyring"] = keyring
	oldMeta.Options["trustdb"] = trustdb
	oldMeta.SignInfo = gomason.SignInfo{
		Program: "gpg",
		Email:   "tester@nikogura.com",
	}

	buildSource(oldMeta, fmt.Sprintf("v%s", oldVersion), sourceDirA, testFilesA)

	return err
}
