package dbt

import (
	"encoding/base64"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ListCatalog shows you what tools are available in your trusted repo.  Repo is figured out from the config in ~/.dbt/conf/dbt.json
func ListCatalog(showVersions bool, homedir string) (err error) {
	fmt.Printf("Fetching information from the repository...\n")

	if homedir == "" {
		homedir, err = GetHomeDir()
		if err != nil {
			err = errors.Wrap(err, "failed to derive user home dir")
			return err
		}
	}

	config, err := LoadDbtConfig(homedir, false)
	if err != nil {
		err = errors.Wrapf(err, "failed to load config from %s/.dbt/conf/dbt.json", homedir)
		return err
	}

	tools, err := FetchTools(config)
	if err != nil {
		err = errors.Wrap(err, "failed to fetch tools from repo")
	}

	// figure out the longest name and set up the fixed with spacing based on it
	largest := 0
	spacing := 4
	pad := strings.Repeat(" ", spacing)

	for _, thing := range tools {
		if len(thing.Name) > largest {
			largest = len(thing.Name)
		}
	}

	fieldLength := largest + spacing
	// %% is literal %

	// Have to construct the format string first
	formatstring := fmt.Sprintf("%%%ds%%s", fieldLength)

	fmt.Printf("Commands:\n\n")
	fmt.Printf("\tCommand Name\t\tLatest Version\t\tDescription\n\n")
	fmt.Printf("\n\n")

	dbtObj := &DBT{
		Config:  config,
		Verbose: false,
		Logger:  log.New(os.Stderr, "", 0),
	}

	for _, tool := range tools {
		version, err := dbtObj.FindLatestVersion(tool.Name)
		if err != nil {
			err = errors.Wrapf(err, "failed to get latest version of %s from %s", tool.Name, config.Tools.Repo)
			return err
		}

		description, err := FetchDescription(config, tool.Name, version)
		if err != nil {
			err = errors.Wrapf(err, "Failed to get description of %s from %s", tool.Name, config.Tools.Repo)
			return err
		}

		tool.FormattedName = fmt.Sprintf(formatstring, tool.Name, pad)

		fmt.Printf("\t%s\t\t%s\t\t\t%s\n", tool.FormattedName, version, description)

		if showVersions {
			versions, err := dbtObj.FetchToolVersions(tool.Name)
			if err != nil {
				err = errors.Wrapf(err, "failed to get versions of %s from %s", tool.Name, config.Tools.Repo)
				return err
			}

			for _, v := range versions {
				if v != version {
					fmt.Printf("\t\t\t\t\t%s\n", v)
				}
			}
		}
	}

	fmt.Printf("\n\n")
	fmt.Printf("Further information on any tool can be shown by running 'dbt <command> help'.\n")
	fmt.Printf("\n\n")

	return err
}

// FetchDescription fetches the tool description from the repository.
func FetchDescription(config Config, tool string, version string) (description string, err error) {
	uri := fmt.Sprintf("%s/%s/%s/description.txt", config.Tools.Repo, tool, version)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request for url: %s", uri)
		return description, err
	}

	username := config.Username
	password := config.Password

	// Username func takes precedence over hardcoded username
	if config.UsernameFunc != "" {
		username, err = GetFunc(config.UsernameFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get username from shell function %q", config.UsernameFunc)
			return description, err
		}
	}

	// PasswordFunc takes precedence over hardcoded password
	if config.PasswordFunc != "" {
		password, err = GetFunc(config.PasswordFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get password from shell function %q", config.PasswordFunc)
			return description, err
		}
	}

	if username != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}

	resp, err := client.Do(req)

	if err != nil {
		err = errors.Wrapf(err, "Error looking for command description in repo %q", uri)
		return description, err
	}

	if resp != nil {
		defer resp.Body.Close()
		responseBytes, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			err = errors.Wrap(err, "Error reading description")
			return description, err
		}

		description = string(responseBytes)
	}

	return description, err
}

// FetchTools returns a list of tool names found in the trusted repo
func FetchTools(config Config) (tools []Tool, err error) {
	// strip off a trailing slash if there is one
	rawUrl := config.Tools.Repo
	munged := strings.TrimRight(rawUrl, "/")
	// Then add one cos we definitely need one
	uri := fmt.Sprintf("%s/", munged)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request for url: %s", uri)
		return tools, err
	}

	username := config.Username
	password := config.Password

	// Username func takes precedence over hardcoded username
	if config.UsernameFunc != "" {
		username, err = GetFunc(config.UsernameFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get username from shell function %q", config.UsernameFunc)
			return tools, err
		}
	}

	// PasswordFunc takes precedence over hardcoded password
	if config.PasswordFunc != "" {
		password, err = GetFunc(config.PasswordFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get password from shell function %q", config.PasswordFunc)
			return tools, err
		}
	}

	if username != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}

	resp, err := client.Do(req)

	tools = make([]Tool, 0)

	if err != nil {
		err = errors.Wrapf(err, "Error looking for commands in repo %q", uri)
		return tools, err
	}

	if resp != nil {
		defer resp.Body.Close()

		parser := html.NewTokenizer(resp.Body)

		for {
			tt := parser.Next()

			switch {
			case tt == html.ErrorToken:
				return
			case tt == html.StartTagToken:
				t := parser.Token()
				isAnchor := t.Data == "a"
				if isAnchor {
					for _, a := range t.Attr {
						if a.Key == "href" {
							if a.Val != "../" {
								// any links beyond the 'back' link will be versions
								// trim the trailing slash so we get actual semantic versions
								name := strings.TrimRight(a.Val, "/")
								tools = append(tools, Tool{Name: name})
							}
						}
					}
				}
			}
		}
	}

	return tools, err
}

// Tool is a struct representing pertinent info on a dbt tool
type Tool struct {
	Name          string
	FormattedName string
	Version       string
	Description   string
}
