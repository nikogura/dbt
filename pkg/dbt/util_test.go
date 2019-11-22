// Copyright © 2019 Nik Ogura <nik.ogura@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbt

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func TestSemverParse(t *testing.T) {
	testParts := exampleVersionParts()
	processedParts, err := SemverParse(exampleVersion())
	if err != nil {
		fmt.Println(fmt.Sprintf("Error parsing semver string: %s", err))
		t.Fail()
	}

	assert.Equal(t, testParts, processedParts, "Parsed semantic version string matches expectations")
}

func TestLatestVersion(t *testing.T) {
	latest := LatestVersion(testVersionList())

	assert.Equal(t, testLatestVersion(), latest, "Latest version found matches expectations.")
}

func TestSpaceship(t *testing.T) {
	assert.Equal(t, 1, Spaceship(1, 0))
	assert.Equal(t, -1, Spaceship(2, 3))
	assert.Equal(t, 0, Spaceship(1, 1))
}

func TestVersionIsNewerThan(t *testing.T) {
	assert.True(t, VersionAIsNewerThanB("1.2.3", "0.1.0"))
}

func TestFileSha256(t *testing.T) {
	fileName := fmt.Sprintf("%s/%s", tmpDir, "foo")

	err := ioutil.WriteFile(fileName, []byte(testFileContents()), 0644)

	if err != nil {
		fmt.Println(fmt.Sprintf("Error writing test file: %s", err))
		t.Fail()
	}

	checksum, err := FileSha256(fileName)

	if err != nil {
		fmt.Println(fmt.Sprintf("Couldn't get checksum for file %q: %s", fileName, err))
		t.Fail()
	}

	assert.Equal(t, testFileChecksumSha256(), checksum, "Checksum of written test file matches expectations.")

}

func TestFileSha1(t *testing.T) {
	fileName := fmt.Sprintf("%s/%s", tmpDir, "foo")

	err := ioutil.WriteFile(fileName, []byte(testFileContents()), 0644)

	if err != nil {
		fmt.Println(fmt.Sprintf("Error writing test file: %s", err))
		t.Fail()
	}

	checksum, err := FileSha1(fileName)

	if err != nil {
		fmt.Println(fmt.Sprintf("Couldn't get checksum for file %q: %s", fileName, err))
		t.Fail()
	}

	fmt.Println(fileName)
	fmt.Println(checksum)

	assert.Equal(t, testFileChecksumSha1(), checksum, "Checksum of written test file matches expectaitons.")

}

func TestStringInSlice(t *testing.T) {
	assert.True(t, StringInSlice(testStringTrue(), exampleSlice()), "Expected string found in slice")
	assert.False(t, StringInSlice(testStringFalse(), exampleSlice()), "Unexpected string not found in slice")
}
