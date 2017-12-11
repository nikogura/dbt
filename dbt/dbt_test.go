package dbt

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

var tmpDir string

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
}

func tearDown() {
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		os.Remove(tmpDir)
	}

}

func TestGenerateDbtDir(t *testing.T) {
	err := generateDbtDir(tmpDir, true)
	if err != nil {
		log.Printf("Error generating dbt dir: %s", err)
		t.Fail()
	}

	dbtDirPath := fmt.Sprintf("%s%s", tmpDir, dbtDir)

	if _, err := os.Stat(dbtDirPath); os.IsNotExist(err) {
		log.Printf("dbt dir %s did not create as expected", dbtDirPath)
		t.Fail()
	}

	trustPath := fmt.Sprintf("%s%s", tmpDir, trustDir)

	if _, err := os.Stat(trustPath); os.IsNotExist(err) {
		log.Printf("trust dir %s did not create as expected", trustPath)
		t.Fail()
	}

	toolPath := fmt.Sprintf("%s%s", tmpDir, toolDir)
	if _, err := os.Stat(toolPath); os.IsNotExist(err) {
		log.Printf("tool dir %s did not create as expected", toolPath)
		t.Fail()
	}
}
