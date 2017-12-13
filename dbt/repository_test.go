package dbt

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestToolExists(t *testing.T) {
	dbtObj := testDbtConfig(port)

	exists, err := ToolExists(dbtObj.Tools.Repo, "foo")
	if err != nil {
		fmt.Printf("Failed to check repo for %q", "foo")
		t.Fail()
	}
	if !exists {
		fmt.Println(fmt.Sprintf("Tool %q does not exist in repo %s", "dbt", dbtObj.Dbt.Repo))
		t.Fail()
	}

	fakeToolName := "bar"

	exists, err = ToolExists(dbtObj.Tools.Repo, fakeToolName)
	if err != nil {
		fmt.Printf("Failed to check artifactory for %q", fakeToolName)
		t.Fail()
	}

	if exists {
		fmt.Println("Nonexistant job shows existing in repo.")
		t.Fail()
	}
}

func TestToolVersionExists(t *testing.T) {
	dbtObj := testDbtConfig(port)

	if !ToolVersionExists(dbtObj.Tools.Repo, "foo", "1.2.3") {
		fmt.Println(fmt.Sprintf("Tool %q version %qdoes not exist in repo %s", "foo", "1.2.3", dbtObj.Tools.Repo))
		t.Fail()
	}

	if ToolVersionExists(dbtObj.Tools.Repo, "foo", "0.0.0") {
		fmt.Println(fmt.Sprintf("Nonexistant job version %q shows existing in repo.", "0.0.0"))
		t.Fail()
	}
}

func TestFetchToolVersions(t *testing.T) {
	dbtObj := testDbtConfig(port)

	versions, err := FetchToolVersions(dbtObj.Tools.Repo, "foo")
	if err != nil {
		fmt.Println(fmt.Sprintf("Error searching for versions of job %q in repo %q", "foo", dbtObj.Tools.Repo))
	}

	fmt.Printf("Versions:")
	spew.Dump(versions)

	assert.True(t, len(versions) > 0, "List of versions is non-zero.")
}
