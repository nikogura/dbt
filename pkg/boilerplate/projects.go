/*
	Copyright <2022> Nik Ogura <nik.ogura@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package boilerplate

import (
	"embed"
	"fmt"
	"log"
	"os"
)

//go:embed project_templates/_cobraProject/*
var cobraProject embed.FS

//go:embed project_templates/_ginProject/*
var ginProject embed.FS

// GetProjectFs  Gets the embedded file system for the project of this type.
func GetProjectFs(projType string) (embed.FS, string, error) {
	switch projType {
	case "cobra":
		return cobraProject, "project_templates/_cobraProject", nil
	case "gin":
		return ginProject, "project_templates/_ginProject", nil
	}

	return embed.FS{}, "", fmt.Errorf("failed to detect embeded package: %s", projType)
}

// ValidProjectTypes  Lists the valid project types.
func ValidProjectTypes() []string {
	return []string{"cobra", "gin"}
}

// IsValidProjectType  Returns true or false depending on whether the project is a supported type.
func IsValidProjectType(v string) bool {
	switch v {
	case "cobra", "gin":
		return true
	}
	return false
}

func PromptsForProject(proj string) (data PromptValues, err error) {
	switch proj {
	case "cobra":
		data := &CobraCliToolParams{}

		for {
			err = CobraCliToolParamsFromPrompts(data, os.Stdin)
			if err != nil {
				fmt.Printf("%s\n", err)
			} else {
				break
			}
		}

		return data, err
	case "gin":
		data := &GinServiceParams{}

		for {
			err = GinServiceParamsFromPrompts(data, os.Stdin)
			if err != nil {
				fmt.Printf("%s\n", err)
			} else {
				break
			}

		}

		return data, err

	default:
		log.Fatalf("unknown or unhandled project type. options are %s", ValidProjectTypes())
	}

	return data, err
}
