/*
	Copyright <2022> Nik Ogura <nik.ogura@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package boilerplate

import (
	"bufio"
	"github.com/stretchr/testify/assert"
	"reflect"
	"strings"
	"testing"
)

func TestGinServiceParamsFromPrompts(t *testing.T) {
	for _, tc := range []struct {
		Name    string
		Inputs  string
		Want    map[string]interface{}
		WantErr bool
		ExpErr  string
	}{
		{
			Name: "Success",
			Inputs: `1.16
test-proj-name
test_proj_pkg
project short desc
project long desc
project maintainer
test@example.com
2345
server short desc
server long desc
`,
			Want: map[string]interface{}{
				ProjName.String():            "test-proj-name",
				ProjPkgName.String():         "test_proj_pkg",
				ProjShortDesc.String():       "project short desc",
				ProjLongDesc.String():        "project long desc",
				ProjMaintainerName.String():  "project maintainer",
				ProjMaintainerEmail.String(): "test@example.com",
				ServerDefPort.String():       "2345",
				ServerShortDesc.String():     "server short desc",
				ServerLongDesc.String():      "server long desc",
				GoVersion.String():           "1.16",
			},
			WantErr: false,
		},
		{
			Name:    "Missing Input",
			Inputs:  `1.16 test-proj-name`,
			Want:    map[string]interface{}{},
			WantErr: true,
		},
		{
			Name: "Name Validation space",
			Inputs: `1.16
test-proj name
test_proj_pkg`,
			Want:    map[string]interface{}{},
			WantErr: true,
		},
		{
			Name: "Name Validation underscore",
			Inputs: `1.16
test-proj_name
test_proj_pkg`,
			Want:    map[string]interface{}{},
			WantErr: true,
		},
		{
			Name: "Package Validation space",
			Inputs: `1.16
test-proj-name
test_proj pkg`,
			Want:    map[string]interface{}{},
			WantErr: true,
		},
		{
			Name: "Package Validation hyphen",
			Inputs: `1.16
test-proj-name
test-proj-pkg`,
			Want:    map[string]interface{}{},
			WantErr: true,
		},
		{
			Name: "Port validation numeric",
			Inputs: `1.16
test-proj-name
test_proj_pkg
project short desc
project long desc
project maintainer
test@example.com
23R5
server short desc
server long desc
`,
			Want:    map[string]interface{}{},
			WantErr: true,
		},
		{
			Name: "Port validation small number",
			Inputs: `1.16
test-proj-name
test_proj_pkg
project short desc
project long desc
project maintainer
test@example.com
235
server short desc
server long desc
`,
			Want:    map[string]interface{}{},
			WantErr: true,
		},
		{
			Name: "Port validation large number",
			Inputs: `1.16
test-proj-name
test_proj_pkg
project short desc
project long desc
project maintainer
test@example.com
235456
server short desc
server long desc
`,
			Want:    map[string]interface{}{},
			WantErr: true,
		},
		{
			Name: "Port validation tcp reserved port",
			Inputs: `1.16
test-proj-name
test_proj_pkg
project short desc
project long desc
project maintainer
test@example.com
1010
server short desc
server long desc
`,
			Want:    map[string]interface{}{},
			WantErr: true,
		},
		{
			Name: "Port validation tcp range high",
			Inputs: `1.16
test-proj-name
test_proj_pkg
project short desc
project long desc
project maintainer
test@example.com
65536
server short desc
server long desc
`,
			Want:    map[string]interface{}{},
			WantErr: true,
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if tc.WantErr {
						t.Logf("got expected err: %v", r)
					} else {
						t.Errorf("got unexpected parsing error: %v", r)
					}
				} else if tc.WantErr {
					t.Errorf("missing expected error")
				}
			}()

			stdin := bufio.NewReader(strings.NewReader(tc.Inputs))
			data := &GinServiceParams{}
			GinServiceParamsFromPrompts(data, stdin)

			dataMap, err := data.AsMap()
			if err != nil {
				t.Errorf("error expressing params as map: %s", err)
			}

			for k, exp := range tc.Want {
				if act, ok := dataMap[k]; !ok {
					t.Errorf("expected key(%s) not in act results", k)
				} else if !reflect.DeepEqual(exp, act) {
					t.Errorf("exp value mismatch for key(%s): exp(%s) act(%s)", k, exp, act)
				}
			}
		})
	}

}

func TestGinServiceParamsFromPrompts_Defaults(t *testing.T) {
	for _, tc := range []struct {
		Name    string
		Inputs  string
		Want    map[string]interface{}
		WantErr bool
		ExpErr  string
	}{
		{
			Name: "All Defaults",
			Inputs: `
test-proj-name
test_proj_pkg


tester
tester@foo.com



`,
			Want: map[string]interface{}{
				ProjName.String():            "test-proj-name",
				ProjPkgName.String():         "test_proj_pkg",
				ProjShortDesc.String():       "boilerplate autogen project",
				ProjLongDesc.String():        "boilerplate autogen project",
				ProjMaintainerName.String():  "tester",
				ProjMaintainerEmail.String(): "tester@foo.com",
				ServerDefPort.String():       "2345",
				ServerShortDesc.String():     "boilerplate server",
				ServerLongDesc.String():      "boilerplate server",
				GoVersion.String():           "1.19.4",
			},
			WantErr: false,
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			//defer func() {
			//	if r := recover(); r != nil {
			//		if tc.WantErr {
			//			t.Logf("got expected err: %v", r)
			//		} else {
			//			t.Errorf("got unexpected parsing error: %v", r)
			//		}
			//	} else if tc.WantErr {
			//		t.Errorf("missing expected error")
			//	}
			//}()

			stdin := bufio.NewReader(strings.NewReader(tc.Inputs))
			data := &GinServiceParams{}
			GinServiceParamsFromPrompts(data, stdin)

			dataMap, err := data.AsMap()
			if err != nil {
				t.Errorf("error expressing params as map: %s", err)
			}

			for k, exp := range tc.Want {
				if act, ok := dataMap[k]; !ok {
					t.Errorf("expected key(%s) not in act results", k)
				} else if !reflect.DeepEqual(exp, act) {
					if k != ServerDefPort.String() {
						t.Errorf("exp value mismatch for key(%s): exp(%s) act(%s)", k, exp, act)
					}
				}
			}
		})
	}
}

func TestGoMajorAndMinor(t *testing.T) {
	version := goMajorAndMinor()

	assert.True(t, version != "", "Go version not generating")
}
