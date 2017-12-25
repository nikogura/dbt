package dbt

import (
	"fmt"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"os"
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
		os.Remove(tmpDir)
	}

}

func TestGenerateDbtDir(t *testing.T) {
	dbtDirPath := fmt.Sprintf("%s/%s", tmpDir, dbtDir)

	if _, err := os.Stat(dbtDirPath); os.IsNotExist(err) {
		log.Printf("dbt dir %s did not create as expected", dbtDirPath)
		t.Fail()
	}

	trustPath := fmt.Sprintf("%s/%s", tmpDir, trustDir)

	if _, err := os.Stat(trustPath); os.IsNotExist(err) {
		log.Printf("trust dir %s did not create as expected", trustPath)
		t.Fail()
	}

	toolPath := fmt.Sprintf("%s/%s", tmpDir, toolDir)
	if _, err := os.Stat(toolPath); os.IsNotExist(err) {
		log.Printf("tool dir %s did not create as expected", toolPath)
		t.Fail()
	}

	configPath := fmt.Sprintf("%s/%s", tmpDir, configDir)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("config dir %s did not create as expected", configPath)
		t.Fail()
	}

}

func TestLoadDbtConfig(t *testing.T) {
	configPath := fmt.Sprintf("%s/%s", tmpDir, configDir)
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
	trustPath := fmt.Sprintf("%s/%s", tmpDir, truststorePath)

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
