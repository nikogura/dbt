package reposerver

import (
	"fmt"
	"github.com/nikogura/dbt/pkg/dbt"
	"github.com/nikogura/gomason/pkg/gomason"
	"github.com/phayes/freeport"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var tmpDir string
var port int
var trustedKeys map[string]string
var testFiles []testFile
var repo DBTRepo

type testFile struct {
	Name     string
	FilePath string
	UrlPath  string
}

func TestMain(m *testing.M) {
	setUp()

	code := m.Run()

	tearDown()

	os.Exit(code)
}

func setUp() {
	dir, err := ioutil.TempDir("", "dbt-server")
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

	trustedKeys = make(map[string]string)

}

func tearDown() {
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		_ = os.Remove(tmpDir)
	}

	// TODO shut down test server?
}

func buildTestRepo() (err error) {
	dbtRoot := fmt.Sprintf("%s/repo/dbt", tmpDir)
	trustFile := fmt.Sprintf("%s/truststore", dbtRoot)
	toolRoot := fmt.Sprintf("%s/repo/dbt-tools", tmpDir)
	version := "1.2.3"

	os.Setenv("GOMASON_NO_USER_CONFIG", "true")

	testFiles = []testFile{
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
	}

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

	src := strings.TrimRight(cwd, "/pkg/reposerver")
	dst := fmt.Sprintf("%s/src/github.com/nikogura/dbt", workDir)
	err = os.MkdirAll(dst, 0755)
	if err != nil {
		log.Fatalf("Failed creating directory %s: %s", dst, err)
	}

	err = dbt.DirCopy(src, dst)
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
	}

	return err
}

func TestRunRepoServer(t *testing.T) {
	err := buildTestRepo()
	if err != nil {
		fmt.Printf("Error building dbt binaries: %s\n", err)
		t.Fail()
	}

	// Set up the repo server
	repo = DBTRepo{
		Address:    "127.0.0.1",
		Port:       port,
		ServerRoot: fmt.Sprintf("%s/repo", tmpDir),
		PubkeyFunc: nil,
	}

	// Run it in the background
	go repo.RunRepoServer()

	// Give things a moment to come up.
	time.Sleep(time.Second)

	host := fmt.Sprintf("http://%s:%d", repo.Address, repo.Port)
	fmt.Sprintf("--- Serving requests on %s ---\n", host)
	for _, f := range testFiles {
		t.Run(f.Name, func(t *testing.T) {
			url := fmt.Sprintf("%s%s", host, f.UrlPath)

			resp, err := http.Get(url)
			if err != nil {
				fmt.Printf("Failed to fetch %s: %s", f.Name, err)
				t.Fail()
			}

			assert.True(t, resp.StatusCode < 300, "Non success error code fetching %s (%d)", url, resp.StatusCode)
		})
	}
}
