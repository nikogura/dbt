package dbt

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"testing"
)

func TestFetchDescription(t *testing.T) {
	desc, err := FetchDescription(dbtConfig, "catalog", VERSION)
	if err != nil {
		fmt.Printf("Error fetching description: %s", err)
		t.Fail()
	}

	assert.Equal(t, "Tool for showing available DBT tools.", desc, "Fetched description meets expectations.")
}

func TestFetchTools(t *testing.T) {
	actual, err := FetchTools(dbtConfig)
	if err != nil {
		fmt.Printf("Error fetching tools: %s", err)
		t.Fail()
	}

	expected := []Tool{
		Tool{
			Name:          "boilerplate",
			FormattedName: "",
			Version:       "",
			Description:   "",
		},
		{
			Name:          "catalog",
			FormattedName: "",
			Version:       "",
			Description:   "",
		},
		{
			Name:          "reposerver",
			FormattedName: "",
			Version:       "",
			Description:   "",
		},
	}

	assert.Equal(t, expected, actual, "returned list of tools meets expectations")
}

func TestListCatalog(t *testing.T) {
	configPath := fmt.Sprintf("%s/%s", tmpDir, ConfigDir)
	fileName := fmt.Sprintf("%s/dbt.json", configPath)

	err := ioutil.WriteFile(fileName, []byte(testDbtConfigContents(port)), 0644)
	if err != nil {
		log.Printf("Error writing config file to %s: %s", fileName, err)
		t.Fail()
	}

	err = ListCatalog(true, tmpDir)
	if err != nil {
		fmt.Printf("Error listing tools: %s\n", err)
		t.Fail()
	}

	assert.Nil(t, err, "ListCatalog produced errors.")
}
