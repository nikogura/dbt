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

func exampleVersion() string {
	return "1.2.3"
}

func exampleVersionParts() []int {

	return []int{1, 2, 3}
}

func testVersionList() (versions []string) {
	versions = []string{
		"1.2.4",
		"1.1.3",
		"1.2.2",
		"0.1.0",
		"2.0.0",
		"2.0.1",
	}

	return versions
}

func testLatestVersion() string {
	return "2.0.1"
}

func testFileContents() string {
	return "The quick fox jumped over the lazy brown dog."
}

func testFileChecksumSha256() string {
	return "1b47f99f277cad8c5e31f21e688e4d0b8803cb591b0383e2319869b520d061a1"
}

func testFileChecksumSha1() string {
	return "5b7c9753dd9800a16969bf65e2330b40e657277b"
}

func exampleSlice() []string {
	return []string{"foo", "bar", "baz", "wip", "zoz"}
}

func testStringTrue() string {
	return "bar"
}

func testStringFalse() string {
	return "fargle"
}
