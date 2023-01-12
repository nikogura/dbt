/*
	Copyright <2022> Nik Ogura <nik.ogura@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package boilerplate

import (
	"fmt"
	"github.com/spf13/afero"
	"testing"
)

func MapOnly(hash map[string]interface{}, err error) map[string]interface{} {

	return hash
}
func TestNewTmplWriter_BuildProject(t *testing.T) {
	for _, tc := range []struct {
		Name     string
		ProjType string
		Params   map[string]interface{}
		ExpStat  []string
		ExpError bool
	}{
		{
			Name:     "Basic Gin Project",
			ProjType: "gin",
			Params: MapOnly(GinServiceParams{
				ProjectName:       "test-proj-github-name",
				ProjectPackage:    "test_proj_pkg",
				ProjectShortDesc:  "proj short",
				ProjectLongDesc:   "proj long",
				MaintainerName:    "test",
				MaintainerEmail:   "test@example.com",
				DefaultServerPort: "7465",
				ServerShortDesc:   "svr short",
				ServerLongDesc:    "svr long",
				GolangVersion:     "1.16",
			}.AsMap()),
			ExpStat: []string{
				"test-proj-github-name",
				"test-proj-github-name/go.mod",
			},
			ExpError: false,
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {

			afs := afero.NewMemMapFs()
			w, err := NewTmplWriter(afs, tc.ProjType, tc.Params)

			if err != nil {
				t.Fatalf("tc(%s) failed to build template writer: %v", tc.Name, err)
			}

			testDir, err := afero.TempDir(afs, ".", "writer_test")
			if err != nil {
				t.Fatalf("cannot create temp dir for test: %v", err)
			}

			// Sanity check to make sure the files aren't already there
			for _, e := range tc.ExpStat {
				expFile := fmt.Sprintf("%s/%s", testDir, e)
				fi, err := afs.Stat(expFile)
				if err == nil && !tc.ExpError {
					t.Fatalf("tc(%s) expected dir to not exist: %+v", tc.Name, fi)
				}
			}

			if err = w.BuildProject(testDir); err != nil {
				t.Fatalf("tc(%s) failed to write template project: %v", tc.Name, err)
			}

			for _, e := range tc.ExpStat {
				expFile := fmt.Sprintf("%s/%s", testDir, e)
				_, err := afs.Stat(expFile)
				if err != nil && !tc.ExpError {
					t.Fatalf("tc(%s) expected file doesn't exist: file(%s)", tc.Name, expFile)
				}
			}
		})
	}
}
