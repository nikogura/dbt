package dbt

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFetchDescription(t *testing.T) {
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
				Config:    s3DbtConfig,
				Verbose:   true,
				S3Session: s3Session,
			},
			homeDirS3,
		},
	}

	for _, tc := range inputs {
		desc, err := tc.obj.FetchToolDescription("catalog", VERSION)
		if err != nil {
			t.Errorf("Error fetching description for 'catalog': %s", err)
		}

		assert.Equal(t, "Tool for showing available DBT tools.", desc, "Fetched description meets expectations.")
	}
}

func TestFetchTools(t *testing.T) {
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
				Config:    s3DbtConfig,
				Verbose:   true,
				S3Session: s3Session,
			},
			homeDirS3,
		},
	}

	for _, tc := range inputs {
		actual, err := tc.obj.FetchToolNames()
		if err != nil {
			t.Errorf("Error fetching tools: %s", err)
		}

		expected := []Tool{
			{
				Name:          "catalog",
				FormattedName: "",
				Version:       "",
				Description:   "",
			},
		}

		assert.Equal(t, expected, actual, "returned list of tools meets expectations")
	}
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
				Config:    s3DbtConfig,
				Verbose:   true,
				S3Session: s3Session,
			},
			homeDirS3,
		},
	}

	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {

			err := tc.obj.FetchCatalog(true)
			if err != nil {
				t.Errorf("Error listing tools: %s\n", err)
			}

			assert.Nil(t, err, "ListCatalog produced errors.")
		})
	}
}
