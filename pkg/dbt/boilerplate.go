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
	files = append(files, ToolFile{Name: fmt.Sprintf("%s/.gitignore", location), Content: GitignoreContents(), Mode: 0644})

	// pre-commit hook
	files = append(files, ToolFile{Name: fmt.Sprintf("%s/pre-commit-hook.sh", location), Content: PreCommitHookContents(), Mode: 0755})

	// Description Template
	files = append(files, ToolFile{Name: fmt.Sprintf("%s/templates/description.tmpl", location), Content: DescriptionTemplateContents(), Mode: 0644})

	// metadata.json
	fileName := "metadata.json"
	content, err := fillTemplate(fileName, MetadataContents(), info)
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
	content, err = fillTemplate(fileName, LicenseContents(), info)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}
	files = append(files, ToolFile{Name: fmt.Sprintf("%s/LICENSE", location), Content: content, Mode: 0644})

	// go.mod
	fileName = "go.mod"
	content, err = fillTemplate(fileName, GoModuleContents(), info)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}

	files = append(files, ToolFile{Name: fmt.Sprintf("%s/go.mod", location), Content: content, Mode: 0644})

	// main.go
	fileName = "main.go"
	content, err = fillTemplate(fileName, MainGoContents(), info)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}

	files = append(files, ToolFile{Name: fmt.Sprintf("%s/main.go", location), Content: content, Mode: 0644})

	// root.go
	fileName = "root.go"
	content, err = fillTemplate(fileName, RootGoContents(), info)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}
	files = append(files, ToolFile{Name: fmt.Sprintf("%s/cmd/root.go", location), Content: content, Mode: 0644})

	// empty.go
	fileName = "emptygofile"
	content, err = fillTemplate(fileName, EmptyGoFileContents(), info)
	if err != nil {
		err = errors.Wrapf(err, "failed to generate content for %s", fileName)
		return files, err
	}
	files = append(files, ToolFile{Name: fmt.Sprintf("%s/pkg/%s/%s.go", location, toolName, toolName), Content: content, Mode: 0644})

	// README.md
	fileName = "README.md"
	content, err = fillTemplate(fileName, ReadMeContents(), info)
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
