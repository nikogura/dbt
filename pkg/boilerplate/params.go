/*
	Copyright <2022> Nik Ogura <nik.ogura@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package boilerplate

import (
	"github.com/pkg/errors"
	"io"
	"net/mail"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type ParamPrompt string

const (
	GoVersion           ParamPrompt = "GolangVersion"
	DockerRegistry      ParamPrompt = "DockerRegistry"
	DockerProject       ParamPrompt = "DockerProject"
	ProjName            ParamPrompt = "ProjectName"
	ProjPkgName         ParamPrompt = "ProjectPackage"
	ProjEnvPrefix       ParamPrompt = "EnvPrefix"
	ProjShortDesc       ParamPrompt = "ProjectShortDesc"
	ProjLongDesc        ParamPrompt = "ProjectLongDesc"
	ProjMaintainerName  ParamPrompt = "MaintainerName"
	ProjMaintainerEmail ParamPrompt = "MaintainerEmail"
	ServerDefPort       ParamPrompt = "DefaultServerPort"
	ServerShortDesc     ParamPrompt = "ServerShortDesc"
	ServerLongDesc      ParamPrompt = "ServerLongDesc"
	OwnerName           ParamPrompt = "OwnerName"
	OwnerEmail          ParamPrompt = "OwnerEmail"
)

func (p ParamPrompt) String() string {
	return string(p)
}

type PromptValues interface {
	Values() map[ParamPrompt]*string
	AsMap() (data map[string]interface{}, err error)
}

var nameValidations = []PromptValidation{
	{
		IsValid: func(val string) bool {
			return !strings.ContainsRune(val, ' ')
		},
		InvalidMsg: "Cannot contain a space",
	},
	{
		IsValid: func(val string) bool {
			return !strings.ContainsRune(val, '_')
		},
		InvalidMsg: "Cannot contain an underscore",
	},
}

var pkgValidations = []PromptValidation{
	{
		IsValid: func(val string) bool {
			return !strings.ContainsRune(val, ' ')
		},
		InvalidMsg: "Cannot contain a space",
	},
	{
		IsValid: func(val string) bool {
			return !strings.ContainsRune(val, '-')
		},
		InvalidMsg: "Cannot contain a hyphen",
	},
}

var envPrefix = []PromptValidation{
	{
		IsValid: func(val string) bool {
			isAlphaCap := regexp.MustCompile(`^[A-Z]+$`).MatchString
			return isAlphaCap(val)
		},
		InvalidMsg: "Must contain capitalized letters only",
	},
}

var emailValidation = []PromptValidation{
	{
		IsValid: func(val string) bool {
			_, err := mail.ParseAddress(val)
			return err == nil
		},
		InvalidMsg: "Must be a valid email address.",
	},
}

var portValidation = []PromptValidation{
	{
		IsValid: func(val string) bool {
			isNumeric := regexp.MustCompile(`^[0-9]+$`).MatchString
			return isNumeric(val)
		},
		InvalidMsg: "Must contain numeric digits only",
	},
	{
		IsValid: func(val string) bool {
			return len(val) >= 4 && len(val) <= 5
		},
		InvalidMsg: "Must be a 4 or 5 digit number",
	},
	{
		IsValid: func(val string) bool {
			port, err := strconv.Atoi(val)
			return err == nil && port >= 1025 && port <= 65535
		},
		InvalidMsg: "Must be in range 1025-65535",
	},
}

func commonPromptMessaging() map[ParamPrompt]Prompt {
	return map[ParamPrompt]Prompt{
		ProjName: {
			PromptMsg:    "Enter the git repo name for your new tool.",
			InputFailMsg: "failed to read project name",
			Validations:  nameValidations,
		},
		ProjPkgName: {
			PromptMsg:    "Enter the go package name for your new tool.",
			InputFailMsg: "failed to read package name",
			Validations:  pkgValidations,
		},
		ProjShortDesc: {
			PromptMsg:    "Enter a short project description.",
			InputFailMsg: "failed to read project description",
			DefaultValue: "boilerplate autogen project",
		},
		ProjLongDesc: {
			PromptMsg:    "Enter a long project description.",
			InputFailMsg: "failed to read project description",
			DefaultValue: "boilerplate autogen project",
		},
		ProjMaintainerName: {
			PromptMsg:    "Enter the project maintainer name.",
			InputFailMsg: "failed to read project maintainer name",
			DefaultValue: "",
		},
		ProjMaintainerEmail: {
			PromptMsg:    "Enter the project maintainer email address.",
			InputFailMsg: "failed to read project maintainer email address",
			Validations:  emailValidation,
			DefaultValue: "",
		},
		GoVersion: {
			PromptMsg:    "Enter a golang semver.",
			InputFailMsg: "failed to read project description",
			DefaultValue: strings.TrimLeft(runtime.Version(), "go"),
		},
	}
}

func paramsFromPrompts(r io.Reader, prompts map[ParamPrompt]Prompt, pvals PromptValues) (err error) {
	values := pvals.Values()
	for _, p := range []ParamPrompt{
		GoVersion,
		ProjName,
		ProjPkgName,
		ProjShortDesc,
		ProjLongDesc,
		ProjMaintainerName,
		ProjMaintainerEmail,
		ServerDefPort,
		ServerShortDesc,
		ServerLongDesc,
		OwnerName,
		OwnerEmail,
	} {
		if _, exists := prompts[p]; !exists {
			continue
		}

		v := prompts[p]
		v.From = r

		dataVar, ok := values[p]
		if !ok {
			err = errors.New("datamap and prompts don't contain the same keys")
			return err
		}

		if dataVar != nil && *dataVar != "" {
			continue
		}

		*dataVar, err = PromptForInput(v)
		if err != nil {
			// clear out the value, so we prompt for it again
			*dataVar = ""
			// return the specific gripe about the input
			return err
		}
	}

	return err
}
