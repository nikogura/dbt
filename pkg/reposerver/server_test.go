package reposerver

import (
	"fmt"
	"github.com/nikogura/gomason/pkg/gomason"
	"github.com/phayes/freeport"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var tmpDir string
var port int
var trustedKeys map[string]string

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

	// setup the file tree for dbt repo
	// truststore
	// install_dbt.sh
	// instal_dbt.sh.asc

	err = buildDbt(tmpDir)
	if err != nil {
		log.Fatalf("Error building dbt binaries: %s", err)
	}

	// setup the file tree for dbt tools

	// Set up the repo server
	repo := DBTRepo{
		Address:    "127.0.0.1",
		Port:       port,
		ServerRoot: tmpDir,
		PubkeyFunc: nil,
	}

	// Run it in the background
	go repo.RunRepoServer()
}

func tearDown() {
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		_ = os.Remove(tmpDir)
	}

	// TODO shut down test server?
}

func buildDbt(repoRoot string) (err error) {
	buildPath := fmt.Sprintf("%s/go", tmpDir)
	dbtRoot := fmt.Sprintf("%s/repo/dbt", tmpDir)
	trustFile := fmt.Sprintf("%s/truststore", dbtRoot)
	toolRoot := fmt.Sprintf("%s/repo/dbt-tools", tmpDir)
	version := "1.2.3"

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

	workDir, err := lang.CreateWorkDir(buildPath)
	if err != nil {
		log.Fatalf("Failed to create ephemeral workDir: %s", err)
	}

	err = lang.Build(workDir, meta, "")
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

	// TODO write truststore file (get pubkey out of gpg-agent)

	targets := map[string]string{
		"boilerplate-description.txt":     fmt.Sprintf("%s/boilerplate/%s/description.txt", toolRoot, version),
		"boilerplate-description.txt.asc": fmt.Sprintf("%s/boilerplate/%s/description.txt.asc", toolRoot, version),
		"boilerplate_darwin_amd64":        fmt.Sprintf("%s/boilerplate/%s/darwin/amd64/boilerplate", toolRoot, version),
		"boilerplate_darwin_amd64.asc":    fmt.Sprintf("%s/boilerplate/%s/darwin/amd64/boilerplate.asc", toolRoot, version),
		"boilerplate_linux_amd64":         fmt.Sprintf("%s/boilerplate/%s/linux/amd64/boilerplate", toolRoot, version),
		"boilerplate_linux_amd64.asc":     fmt.Sprintf("%s/boilerplate/%s/linux/amd64/boilerplate.asc", toolRoot, version),
		"catalog-description.txt":         fmt.Sprintf("%s/catalog/%s/description.txt", toolRoot, version),
		"catalog-description.txt.asc":     fmt.Sprintf("%s/catalog/%s/description.txt.asc", toolRoot, version),
		"catalog_darwin_amd64":            fmt.Sprintf("%s/catalog/%s/darwin/amd64/catalog", toolRoot, version),
		"catalog_darwin_amd64.asc":        fmt.Sprintf("%s/catalog/%s/darwin/amd64/catalog.asc", toolRoot, version),
		"catalog_linux_amd64":             fmt.Sprintf("%s/catalog/%s/linux/amd64/catalog", toolRoot, version),
		"catalog_linux_amd64.asc":         fmt.Sprintf("%s/catalog/%s/linux/amd64/catalog.asc", toolRoot, version),
		"dbt_darwin_amd64":                fmt.Sprintf("%s/%s/darwin/amd64/dbt", dbtRoot, version),
		"dbt_darwin_amd64.asc":            fmt.Sprintf("%s/%s/darwin/amd64/dbt.asc", dbtRoot, version),
		"dbt_linux_amd64":                 fmt.Sprintf("%s/%s/linux/amd64/dbt", dbtRoot, version),
		"dbt_linux_amd64.asc":             fmt.Sprintf("%s/%s/linux/amd64/dbt.asc", dbtRoot, version),
		"install_dbt.sh":                  fmt.Sprintf("%s/install_dbt.sh", dbtRoot),
		"install_dbt.sh.asc":              fmt.Sprintf("%s/install_dbt.sh.asc", dbtRoot),
	}

	// Write the files into place
	for k, v := range targets {
		src := fmt.Sprintf("%s/%s", cwd, k)
		dir := filepath.Dir(v)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err = os.MkdirAll(dir, 0755)
			if err != nil {
				log.Fatalf("Error creating dir %s: %s", v, err)
			}
		}

		input, err := ioutil.ReadFile(src)
		if err != nil {
			log.Fatalf("Failed to read file %s: %s", src, err)
		}

		err = ioutil.WriteFile(v, input, 0644)
		if err != nil {
			log.Fatalf("Failed to write file %s: %s", v, err)
		}

		err = os.Remove(src)
		if err != nil {
			log.Fatalf("Failed to remove file %s: %s", src, err)
		}
	}

	return err
}
