/*
	Copyright <2022> Nik Ogura <nik.ogura@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package boilerplate

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"math/rand"
)

type GinServiceParams struct {
	DockerRegistry    string
	DockerProject     string
	ProjectName       string
	ProjectPackage    string
	EnvPrefix         string
	ProjectShortDesc  string
	ProjectLongDesc   string
	MaintainerName    string
	MaintainerEmail   string
	DefaultServerPort string
	ServerShortDesc   string
	ServerLongDesc    string
	GolangVersion     string
}

func (gp *GinServiceParams) Values() map[ParamPrompt]*string {
	return map[ParamPrompt]*string{
		DockerRegistry:      &gp.DockerRegistry,
		DockerProject:       &gp.DockerProject,
		ProjName:            &gp.ProjectName,
		ProjPkgName:         &gp.ProjectPackage,
		ProjEnvPrefix:       &gp.EnvPrefix,
		ProjShortDesc:       &gp.ProjectShortDesc,
		ProjLongDesc:        &gp.ProjectLongDesc,
		ProjMaintainerName:  &gp.MaintainerName,
		ProjMaintainerEmail: &gp.MaintainerEmail,
		ServerDefPort:       &gp.DefaultServerPort,
		ServerShortDesc:     &gp.ServerShortDesc,
		ServerLongDesc:      &gp.ServerLongDesc,
		GoVersion:           &gp.GolangVersion,
	}
}

func (p GinServiceParams) AsMap() (output map[string]interface{}, err error) {
	data, err := json.Marshal(&p)
	if err != nil {
		err = errors.Wrapf(err, "failed to marshal params object")
		return output, err
	}

	output = make(map[string]interface{})
	if err = json.Unmarshal(data, &output); err != nil {
		err = errors.Wrapf(err, "failed to unmarshal data just marshalled")
		return output, err
	}

	return output, err
}

func GetGinServiceParamsPromptMessaging() map[ParamPrompt]Prompt {
	ret := map[ParamPrompt]Prompt{
		ServerDefPort: {
			PromptMsg:    "Enter the default server port",
			InputFailMsg: "failed to read the server port",
			Validations:  portValidation,
			DefaultValue: fmt.Sprintf("%d", (rand.Int31n(65535)+1024)%65535),
		},
		ServerShortDesc: {
			PromptMsg:    "Enter a short server service description",
			InputFailMsg: "failed to read project description",
			DefaultValue: "boilerplate server",
		},
		ServerLongDesc: {
			PromptMsg:    "Enter a long server service description",
			InputFailMsg: "failed to read project description",
			DefaultValue: "boilerplate server",
		},
	}

	for k, v := range commonPromptMessaging() {
		ret[k] = v
	}
	return ret
}

func GinServiceParamsFromPrompts(params *GinServiceParams, r io.Reader) (err error) {
	prompts := GetGinServiceParamsPromptMessaging()
	err = paramsFromPrompts(r, prompts, params)
	if err != nil {
		err = errors.Wrapf(err, "failed prompting for gin params")
		return err
	}

	return err
}
