package catalog

import (
	"fmt"
	"github.com/nikogura/dbt/pkg/dbt"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"
)

var tmpDir string
var port int
var dbtConfig dbt.Config

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
		log.Printf("Error getting a free port: %s\n", err)
		os.Exit(1)
	}

	port = freePort

	configContents := testDbtConfigContents(port)

	err = os.MkdirAll(fmt.Sprintf("%s/.dbt/conf", tmpDir), 0755)
	if err != nil {
		fmt.Printf("Error creating config dir: %s", err)
		os.Exit(1)
	}

	configFileName := fmt.Sprintf("%s/.dbt/conf/dbt.json", tmpDir)

	fmt.Printf("Writing dbt config file to %s\n", configFileName)
	err = ioutil.WriteFile(configFileName, []byte(configContents), 0644)
	if err != nil {
		fmt.Printf("Error writing dbt config file: %s\n", err)
		os.Exit(1)
	}

	tr := TestRepo{}

	go tr.Run(port)

	log.Printf("Sleeping for 1 second for the test artifact server to start up.")
	time.Sleep(time.Second * 1)

	dbtConfig = testDbtConfig(port)

}

func tearDown() {
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		os.Remove(tmpDir)
	}

}

func TestFetchDescription(t *testing.T) {
	desc, err := FetchDescription(dbtConfig, "foo", "1.2.3")
	if err != nil {
		fmt.Printf("Error fetching description: %s", err)
		t.Fail()
	}

	assert.Equal(t, fooDescription(), desc, "Fetched description meets expectations.")

}

func TestFetchTools(t *testing.T) {
	tools, err := FetchTools(dbtConfig)
	if err != nil {
		fmt.Printf("Error fetching tools: %s", err)
		t.Fail()
	}

	assert.Equal(t, testTools(), tools, "returned list of tools meets expectations")
}

func TestList(t *testing.T) {
	err := List(true, tmpDir)
	if err != nil {
		fmt.Printf("Error listing tools: %s", err)
		t.Fail()
	}
}
