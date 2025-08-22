/*
	Copyright <2022> Nik Ogura <nik.ogura@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package boilerplate

import (
	"fmt"
	"github.com/nikogura/dbt/pkg/dbt"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	"io"
	"net/mail"
	"net/url"
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
	DbtRepo             ParamPrompt = "DbtRepo"
	ProjectVersion      ParamPrompt = "ProjectVersion"
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
		InvalidMsg: "Error: Tool name cannot contain a space",
	},
	{
		IsValid: func(val string) bool {
			return !strings.ContainsRune(val, '_')
		},
		InvalidMsg: "Error: Tool name cannot contain an underscore",
	},
}

var moduleValidations = []PromptValidation{
	{
		IsValid: func(val string) bool {
			return !strings.ContainsRune(val, ' ')
		},
		InvalidMsg: "Error: Module name cannot contain a space",
	},
	{
		IsValid: func(val string) bool {
			// Basic validation for module path format (host/path)
			parts := strings.Split(val, "/")
			if len(parts) < 2 {
				return false
			}
			// Check if first part looks like a hostname
			hostname := parts[0]
			if strings.Contains(hostname, ".") || hostname == "localhost" {
				return true
			}
			return false
		},
		InvalidMsg: "Error: Module name must be in format 'host.com/path' (e.g., github.com/user/project)",
	},
}

var envPrefix = []PromptValidation{
	{
		IsValid: func(val string) bool {
			isAlphaCap := regexp.MustCompile(`^[A-Z]+$`).MatchString
			return isAlphaCap(val)
		},
		InvalidMsg: "Error: Must contain capitalized letters only",
	},
}

var emailValidation = []PromptValidation{
	{
		IsValid: func(val string) bool {
			_, err := mail.ParseAddress(val)
			return err == nil
		},
		InvalidMsg: "Error: Email must be a valid email address.",
	},
}

var portValidation = []PromptValidation{
	{
		IsValid: func(val string) bool {
			isNumeric := regexp.MustCompile(`^[0-9]+$`).MatchString
			return isNumeric(val)
		},
		InvalidMsg: "Error: Port must contain numeric digits only",
	},
	{
		IsValid: func(val string) bool {
			return len(val) >= 4 && len(val) <= 5
		},
		InvalidMsg: "Error: Port must be a 4 or 5 digit number",
	},
	{
		IsValid: func(val string) bool {
			port, err := strconv.Atoi(val)
			return err == nil && port >= 1025 && port <= 65535
		},
		InvalidMsg: "Error: Port must be in range 1025-65535",
	},
}

var urlValidation = []PromptValidation{
	{
		IsValid: func(val string) bool {
			u, err := url.ParseRequestURI(val)
			return err == nil && u != nil
		},
		InvalidMsg: "Error: DBT URL must be a valid URL",
	},
}

var semVerValidation = []PromptValidation{
	{
		IsValid: func(val string) bool {
			ok := semver.IsValid(val)

			if !ok {
				val = fmt.Sprintf("v%s", val)
			}

			return semver.IsValid(val)
		},
		InvalidMsg: "Error: Version must be a valid semantic version.",
	},
}

func commonPromptMessaging() map[ParamPrompt]Prompt {
	return map[ParamPrompt]Prompt{
		ProjName: {
			PromptMsg:    "Enter a name for your new tool.",
			InputFailMsg: "failed to read project name",
			Validations:  nameValidations,
		},
		ProjPkgName: {
			PromptMsg:    "Enter the go module name for your new tool.",
			InputFailMsg: "failed to read module name",
			Validations:  moduleValidations,
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
			DefaultValue: "you@example.com",
		},
		ProjMaintainerEmail: {
			PromptMsg:    "Enter the project maintainer email address.",
			InputFailMsg: "failed to read project maintainer email address",
			Validations:  emailValidation,
			DefaultValue: "code@example.com",
		},
		GoVersion: {
			PromptMsg:    "Enter a golang semver.",
			InputFailMsg: "failed to read project description",
			DefaultValue: goMajorAndMinor(),
		},
		DbtRepo: {
			PromptMsg:    "Enter your DBT Repository URL.",
			InputFailMsg: "failed to read dbt repo url",
			Validations:  urlValidation,
			DefaultValue: installedDbtRepo(),
		},
		ProjectVersion: {
			PromptMsg:    "Enter a semantic version.",
			InputFailMsg: "failed to read semantic version",
			Validations:  semVerValidation,
			DefaultValue: "0.1.0",
		},
	}
}

func installedDbtRepo() (repoUrl string) {
	homedir, err := dbt.GetHomeDir()
	if err != nil {
		err = errors.Wrapf(err, "failed to discover user homedir")
	}

	config, err := dbt.LoadDbtConfig(homedir, false)
	if err != nil {
		err = errors.Wrapf(err, "failed loading dbt config")
		//fmt.Printf("error: %s\n", err)
	}

	repoUrl = config.Tools.Repo
	return repoUrl
}

func goMajorAndMinor() (goMajMin string) {
	parts := strings.Split(strings.TrimLeft(runtime.Version(), "go"), ".")
	goMajMin = fmt.Sprintf("%s.%s", parts[0], parts[1])
	return goMajMin
}

func paramsFromPrompts(r io.Reader, prompts map[ParamPrompt]Prompt, pvals PromptValues) (err error) {
	values := pvals.Values()
	for _, p := range []ParamPrompt{
		ProjName,
		GoVersion,
		ProjPkgName,
		ProjShortDesc,
		ProjLongDesc,
		DbtRepo,
		ProjectVersion,
		ProjMaintainerName,
		ProjMaintainerEmail,
		ServerDefPort,
		OwnerName,
		OwnerEmail,
	} {
		if _, exists := prompts[p]; !exists {
			continue
		}

		v := prompts[p]
		v.From = r

		// Set dynamic default for package name based on project name
		if p == ProjPkgName {
			if projectName, exists := values[ProjName]; exists && projectName != nil && *projectName != "" {
				v.DefaultValue = fmt.Sprintf("github.com/something/%s", *projectName)
			}
		}

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
