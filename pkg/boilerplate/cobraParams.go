/*
	Copyright <2022> Nik Ogura <nik.ogura@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package boilerplate

import (
	"encoding/json"
	"github.com/pkg/errors"
	"io"
	"strings"
)

type CobraCliToolParams struct {
	ProjectName      string
	ProjectPackage   string
	ProjectShortDesc string
	ProjectLongDesc  string
	MaintainerName   string
	MaintainerEmail  string
	GolangVersion    string
	DbtRepo          string
	ProjectVersion   string
}

func (cp *CobraCliToolParams) Values() map[ParamPrompt]*string {
	return map[ParamPrompt]*string{
		GoVersion:           &cp.GolangVersion,
		ProjName:            &cp.ProjectName,
		ProjPkgName:         &cp.ProjectPackage,
		ProjShortDesc:       &cp.ProjectShortDesc,
		ProjLongDesc:        &cp.ProjectLongDesc,
		ProjMaintainerName:  &cp.MaintainerName,
		ProjMaintainerEmail: &cp.MaintainerEmail,
		DbtRepo:             &cp.DbtRepo,
		ProjectVersion:      &cp.ProjectVersion,
	}
}

func (cp CobraCliToolParams) AsMap() (output map[string]interface{}, err error) {
	data, err := json.Marshal(&cp)
	if err != nil {
		err = errors.Wrapf(err, "failed to marshal params object")
		return output, err
	}

	output = make(map[string]interface{})
	if err = json.Unmarshal(data, &output); err != nil {
		err = errors.Wrapf(err, "failed to unmarshal data just marshalled")
		return output, err
	}

	// Add a Go package-safe version of ProjectName
	output["ProjectPackageName"] = strings.ReplaceAll(cp.ProjectName, "-", "")

	return output, err
}

func GetCobraCliToolParamsPromptMessaging() map[ParamPrompt]Prompt {
	return commonPromptMessaging()
}

func CobraCliToolParamsFromPrompts(params *CobraCliToolParams, r io.Reader) (err error) {
	prompts := GetCobraCliToolParamsPromptMessaging()
	err = paramsFromPrompts(r, prompts, params)
	if err != nil {
		return err
	}

	return err
}
