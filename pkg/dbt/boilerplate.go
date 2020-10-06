package dbt

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
	"time"
)

// ERR_TEMPLATE_NOT_FOUND default error prefix for when templates are not found
const ERR_TEMPLATE_NOT_FOUND = "template not found"

// GITIGNORE_TEMPLATE_NAME internal name for template that produces .gitignore
const GITIGNORE_TEMPLATE_NAME = "gitignore"

// METADATA_TEMPLATE_NAME internal name template that produces metadata.json
const METADATA_TEMPLATE_NAME = "metadata"

// PRECOMMIT_TEMPLATE_NAME internal name for template that produces pre-commit-hook.sh
const PRECOMMIT_TEMPLATE_NAME = "precommithook"

// GOMOD_TEMPLATE_NAME internal name for template that produces go.mod
const GOMOD_TEMPLATE_NAME = "gomod"

// MAINGO_TEMPLATE_NAME internal name for template that produces main.go
const MAINGO_TEMPLATE_NAME = "maingo"

// ROOTGO_TEMPLATE_NAME internal name for template that produces root.go
const ROOTGO_TEMPLATE_NAME = "rootgo"

// LICENSE_TEMPLATE_NAME internal name for template that produces LICENSE
const LICENSE_TEMPLATE_NAME = "license"

// EMPTYGO_TEMPLATE_NAME internal name for template that produces <pkgname>.go
const EMPTYGO_TEMPLATE_NAME = "emptygo"

// DESCRIPTION_TEMPLATE_NAME internal name for template that produces description.tmpl
const DESCRIPTION_TEMPLATE_NAME = "description"

// README_TEMPLATE_NAME internal name for template that produces README.md
const README_TEMPLATE_NAME = "readme"

// TEMPLATES_FROM_FILESYSTEM_ENV_VAR ENV var telling boilerplate to load templates from the filesystem rather than use the defaults.
const TEMPLATES_FROM_FILESYSTEM_ENV_VAR = "BOILERPLATE_TEMPLATES_FROM_FILESYSTEM"

// TemplatesFromFilesystem  Switch to tell Boilerplate to read templates from the filesystem rather than using the defaults.  Set to true if BOILERPLATE_TEMPLATES_FROM_FILESYSTEM is undefined, or set to anything other than 'false', or '0'.
var TemplatesFromFilesystem bool

var DefaultTemplateByName map[string]string

func init() {
	tffs := os.Getenv(TEMPLATES_FROM_FILESYSTEM_ENV_VAR)
	if tffs != "" {
		if tffs != "false" {
			if tffs != "0" {
				TemplatesFromFilesystem = true
			}
		}
	}

	DefaultTemplateByName = map[string]string{
		GITIGNORE_TEMPLATE_NAME:   DEFAULT_GITIGNORE_TEMPLATE,
		METADATA_TEMPLATE_NAME:    DEFAULT_METADATA_TEMPLATE,
		PRECOMMIT_TEMPLATE_NAME:   DEFAULT_PREHOOK_TEMPLATE,
		GOMOD_TEMPLATE_NAME:       DEFAULT_GOMODULE_TEMPLATE,
		MAINGO_TEMPLATE_NAME:      DEFAULT_MAIN_GO_TEMPLATE,
		ROOTGO_TEMPLATE_NAME:      DEFAULT_ROOT_GO_TEMPLATE,
		LICENSE_TEMPLATE_NAME:     DEFAULT_LICENSE_TEMPLATE,
		EMPTYGO_TEMPLATE_NAME:     DEFAULT_EMPTY_GO_TEMPLATE,
		DESCRIPTION_TEMPLATE_NAME: DEFAULT_DESCRIPTION_TEMPLATE,
		README_TEMPLATE_NAME:      DEFAULT_README_TEMPLATE,
	}
}

// ToolFile contains information about a boilerplate file to be written
type ToolFile struct {
	Name    string
	Content string
	Mode    os.FileMode
}

// ToolAuthor carries information about the tool's author
type ToolAuthor struct {
	Name  string
	Email string
}

// ToolInfo carries information about the tool we're about to create
type ToolInfo struct {
	PackageName        string
	ToolName           string
	PackageDescription string
	Author             ToolAuthor
	CopyrightYear      int
	Repository         string
}

// GetTemplate returns a template string either from the default, or from the filesystem.
func GetTemplate(filename string) (template string, err error) {
	if TemplatesFromFilesystem {
		// TODO implement load templates from filesystem
		err = errors.New("Templates from filesystem not implemented yet.")
		return template, err
	} else {
		tmpl, ok := DefaultTemplateByName[filename]
		if ok {
			return tmpl, err
		}
		err = errors.New(fmt.Sprintf("%s:%s", ERR_TEMPLATE_NOT_FOUND, filename))
		return tmpl, err
	}
}

// WriteConfigFiles writes the various files for the tool we're creating
func WriteConfigFiles(toolName string, packageName string, packageDescription string, author ToolAuthor, repository string) (err error) {
	location, err := os.Getwd()
	if err != nil {
		err = errors.Wrap(err, "failed to get current working directory")
	}

	location = fmt.Sprintf("%s/%s", location, toolName)

	files, err := FilesForTool(location, toolName, packageName, packageDescription, author, repository)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate file list")
		return err
	}

	dirs := DirsForPackageName(location, toolName)

	for _, dir := range dirs {
		fmt.Println("Creating " + dir)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			fmt.Println(fmt.Sprintf("Error creating directory %q: %s", dir, err))
			return err
		}
	}

	for _, file := range files {
		fmt.Println("Writing " + file.Name)
		mode := file.Mode
		name := file.Name
		content := []byte(file.Content)

		err := ioutil.WriteFile(name, content, mode)

		if err != nil {
			fmt.Println(fmt.Sprintf("Error writing file %q: %s", name, err))
			return err
		}
	}

	err = os.Chdir(location)

	return err
}

// DirsForPackageName produces the required subdirs for the new tool
func DirsForPackageName(location string, toolName string) (dirs []string) {
	dirs = make([]string, 0)

	dirs = append(dirs, fmt.Sprintf("%s/pkg/%s", location, toolName))
	dirs = append(dirs, fmt.Sprintf("%s/templates", location))
	dirs = append(dirs, fmt.Sprintf("%s/cmd", location))

	return dirs
}

// FilesForTool produces the file contents for each boilerplate file
func FilesForTool(location string, toolName string, packageName string, packageDescription string, author ToolAuthor, repository string) (files []ToolFile, err error) {
	files = make([]ToolFile, 0)
	timestamp := time.Now()

	info := ToolInfo{packageName, toolName, packageDescription, author, timestamp.Year(), repository}

	// .gitignore
	tmpl, err := GetTemplate(GITIGNORE_TEMPLATE_NAME)
	if err != nil {
		err = errors.Wrapf(err, fmt.Sprintf("%s: %s", ERR_TEMPLATE_NOT_FOUND, GITIGNORE_TEMPLATE_NAME))
		return files, err
	}

	files = append(files, ToolFile{Name: fmt.Sprintf("%s/.gitignore", location), Content: tmpl, Mode: 0644})

	// pre-commit hook
	tmpl, err = GetTemplate(PRECOMMIT_TEMPLATE_NAME)
	if err != nil {
		err = errors.Wrapf(err, fmt.Sprintf("%s: %s", ERR_TEMPLATE_NOT_FOUND, PRECOMMIT_TEMPLATE_NAME))
		return files, err
	}

	files = append(files, ToolFile{Name: fmt.Sprintf("%s/pre-commit-hook.sh", location), Content: tmpl, Mode: 0755})

	// Description Template
	tmpl, err = GetTemplate(DESCRIPTION_TEMPLATE_NAME)
	if err != nil {
		err = errors.Wrapf(err, fmt.Sprintf("%s: %s", ERR_TEMPLATE_NOT_FOUND, DESCRIPTION_TEMPLATE_NAME))
		return files, err
	}

	files = append(files, ToolFile{Name: fmt.Sprintf("%s/templates/description.tmpl", location), Content: tmpl, Mode: 0644})

	// metadata.json
	fileName := "metadata.json"

	tmpl, err = GetTemplate(METADATA_TEMPLATE_NAME)
	if err != nil {
		err = errors.Wrapf(err, fmt.Sprintf("%s: %s", ERR_TEMPLATE_NOT_FOUND, METADATA_TEMPLATE_NAME))
		return files, err
	}

	content, err := fillTemplate(fileName, tmpl, info)
	content = strings.Replace(content, "__VERSION__", "{{.Version}}", -1)
	content = strings.Replace(content, "__REPOSITORY__", "{{.Repository}}", -1)
	content = strings.Replace(content, "__TOOLNAME__", "{{.Name}}", -1)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}

	files = append(files, ToolFile{Name: fmt.Sprintf("%s/metadata.json", location), Content: content, Mode: 0644})

	// Apache License
	fileName = "LICENSE"
	tmpl, err = GetTemplate(LICENSE_TEMPLATE_NAME)
	if err != nil {
		err = errors.Wrapf(err, fmt.Sprintf("%s: %s", ERR_TEMPLATE_NOT_FOUND, LICENSE_TEMPLATE_NAME))
		return files, err
	}

	content, err = fillTemplate(fileName, tmpl, info)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}
	files = append(files, ToolFile{Name: fmt.Sprintf("%s/LICENSE", location), Content: content, Mode: 0644})

	// go.mod
	fileName = "go.mod"
	tmpl, err = GetTemplate(GOMOD_TEMPLATE_NAME)
	if err != nil {
		err = errors.Wrapf(err, fmt.Sprintf("%s: %s", ERR_TEMPLATE_NOT_FOUND, GOMOD_TEMPLATE_NAME))
		return files, err
	}

	content, err = fillTemplate(fileName, tmpl, info)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}

	files = append(files, ToolFile{Name: fmt.Sprintf("%s/go.mod", location), Content: content, Mode: 0644})

	// main.go
	fileName = "main.go"
	tmpl, err = GetTemplate(MAINGO_TEMPLATE_NAME)
	if err != nil {
		err = errors.Wrapf(err, fmt.Sprintf("%s: %s", ERR_TEMPLATE_NOT_FOUND, MAINGO_TEMPLATE_NAME))
		return files, err
	}

	content, err = fillTemplate(fileName, tmpl, info)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}

	files = append(files, ToolFile{Name: fmt.Sprintf("%s/main.go", location), Content: content, Mode: 0644})

	// root.go
	fileName = "root.go"
	tmpl, err = GetTemplate(ROOTGO_TEMPLATE_NAME)
	if err != nil {
		err = errors.Wrapf(err, fmt.Sprintf("%s: %s", ERR_TEMPLATE_NOT_FOUND, ROOTGO_TEMPLATE_NAME))
		return files, err
	}

	content, err = fillTemplate(fileName, tmpl, info)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}
	files = append(files, ToolFile{Name: fmt.Sprintf("%s/cmd/root.go", location), Content: content, Mode: 0644})

	// empty.go
	fileName = "emptygofile"
	tmpl, err = GetTemplate(EMPTYGO_TEMPLATE_NAME)
	if err != nil {
		err = errors.Wrapf(err, fmt.Sprintf("%s: %s", ERR_TEMPLATE_NOT_FOUND, EMPTYGO_TEMPLATE_NAME))
		return files, err
	}

	content, err = fillTemplate(fileName, tmpl, info)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}
	files = append(files, ToolFile{Name: fmt.Sprintf("%s/pkg/%s/%s.go", location, toolName, toolName), Content: content, Mode: 0644})

	// README.md
	fileName = "README.md"
	tmpl, err = GetTemplate(README_TEMPLATE_NAME)
	if err != nil {
		err = errors.Wrapf(err, fmt.Sprintf("%s: %s", ERR_TEMPLATE_NOT_FOUND, README_TEMPLATE_NAME))
		return files, err
	}

	content, err = fillTemplate(fileName, tmpl, info)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}
	files = append(files, ToolFile{Name: fmt.Sprintf("%s/%s", location, fileName), Content: content, Mode: 0644})

	return files, err
}

// abstract the template filling functions so we dry
func fillTemplate(templateName string, templateText string, data interface{}) (output string, err error) {
	tmpl, err := template.New(templateName).Parse(templateText)
	if err != nil {
		err = errors.Wrapf(err, "failed to parse %s template", templateName)
		return output, err
	}

	buf := new(bytes.Buffer)

	err = tmpl.Execute(buf, data)
	if err != nil {
		return output, err
	}

	output = buf.String()

	return output, err
}

// PromptForToolName prompts the user for a name for the new tool.
func PromptForToolName() (name string, err error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter a name for your new tool: ")

	input, err := reader.ReadString('\n')
	if err != nil {
		err = errors.Wrapf(err, "failed to read input name")
		return name, err
	}

	name = strings.TrimRight(input, "\n")

	return name, err
}

// PromptForToolPackage prompts the user for the package name for the new tool.
func PromptForToolPackage() (packageName string, err error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter a go package for your new tool: ")

	input, err := reader.ReadString('\n')
	if err != nil {
		err = errors.Wrapf(err, "failed to read input name")
		return packageName, err
	}

	packageName = strings.TrimRight(input, "\n")

	return packageName, err
}

// PromptForToolDescription prompts the user for a description for the new tool.
func PromptForToolDescription() (toolDescription string, err error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter a short, one line description for your new tool: ")

	input, err := reader.ReadString('\n')
	if err != nil {
		err = errors.Wrapf(err, "failed to read input name")
		return toolDescription, err
	}

	toolDescription = strings.TrimRight(input, "\n")

	return toolDescription, err
}

// PromptForToolAuthor prompts the user for a name for the new tool.
func PromptForToolAuthor() (author ToolAuthor, err error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter your name: ")

	input, err := reader.ReadString('\n')
	if err != nil {
		err = errors.Wrapf(err, "failed to read input name")
		return author, err
	}

	author.Name = strings.TrimRight(input, "\n")

	fmt.Printf("Enter  your email address: ")

	input, err = reader.ReadString('\n')
	if err != nil {
		err = errors.Wrapf(err, "failed to read input email address")
		return author, err
	}

	author.Email = strings.TrimRight(input, "\n")

	return author, err
}

// PromptForToolRepo prompts the user for a repository for the new tool.
func PromptForToolRepo() (toolDescription string, err error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter the dbt tool repository url for your new tool (where compiled tools will be published): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		err = errors.Wrapf(err, "failed to read input url")
		return toolDescription, err
	}

	toolDescription = strings.TrimRight(input, "\n")

	return toolDescription, err
}
