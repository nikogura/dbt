package catalog

import (
	"fmt"
	"github.com/nikogura/dbt/pkg/dbt"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

// List shows you what tools are available in your trusted repo.  Repo is figured out from the config in ~/.dbt/conf/dbt.json
func List(showVersions bool, homedir string) (err error) {
	fmt.Printf("Fetching information from the repository...\n")

	if homedir == "" {
		homedir, err = dbt.GetHomeDir()
		if err != nil {
			err = errors.Wrap(err, "failed to derive user home dir")
			return err
		}
	}

	config, err := dbt.LoadDbtConfig(homedir, false)
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

	dbtObj := &dbt.DBT{
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
func FetchDescription(config dbt.Config, tool string, version string) (description string, err error) {
	uri := fmt.Sprintf("%s/%s/%s/description.txt", config.Tools.Repo, tool, version)

	resp, err := http.Get(uri)

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
func FetchTools(config dbt.Config) (tools []Tool, err error) {
	uri := config.Tools.Repo
	resp, err := http.Get(uri)

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
