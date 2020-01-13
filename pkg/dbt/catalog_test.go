package dbt

import (
	"fmt"
	"github.com/stretchr/testify/assert"
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

func TestList(t *testing.T) {
	err := List(true, tmpDir)
	if err != nil {
		fmt.Printf("Error listing tools: %s", err)
		t.Fail()
	}
}
