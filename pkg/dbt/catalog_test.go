package dbt

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
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
		{
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
	inputs := []struct {
		name    string
		obj     *DBT
		homedir string
	}{
		{
			"reposerver",

			&DBT{
				Config:  dbtConfig,
				Verbose: true,
			},
			homeDirRepoServer,
		},
		{
			"s3",
			&DBT{
				Config:  s3DbtConfig,
				Verbose: true,
			},
			homeDirS3,
		},
	}

	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			configPath := fmt.Sprintf("%s/%s", tc.homedir, ConfigDir)
			fileName := fmt.Sprintf("%s/dbt.json", configPath)

			err := ioutil.WriteFile(fileName, []byte(testDbtConfigContents(port)), 0644)
			if err != nil {
				t.Errorf("Error writing config file to %s: %s", fileName, err)
			}

			err = ListCatalog(true, tc.homedir)
			if err != nil {
				t.Errorf("Error listing tools: %s\n", err)
			}

			assert.Nil(t, err, "ListCatalog produced errors.")
		})
	}
}
