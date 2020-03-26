package dbt

import (
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"
)

// FetchCatalog shows you what tools are available in your trusted repo.  Repo is figured out from the config in ~/.dbt/conf/dbt.json
func (dbt *DBT) FetchCatalog(showVersions bool) (err error) {
	fmt.Printf("Fetching information from the repository...\n")

	tools, err := dbt.FetchToolNames()
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

	for _, tool := range tools {
		version, err := dbt.FindLatestVersion(tool.Name)
		if err != nil {
			err = errors.Wrapf(err, "failed to get latest version of %s from %s", tool.Name, dbt.Config.Tools.Repo)
			return err
		}

		description, err := dbt.FetchToolDescription(tool.Name, version)
		if err != nil {
			err = errors.Wrapf(err, "Failed to get description of %s from %s", tool.Name, dbt.Config.Tools.Repo)
			return err
		}

		tool.FormattedName = fmt.Sprintf(formatstring, tool.Name, pad)

		fmt.Printf("\t%s\t\t%s\t\t\t%s\n", tool.FormattedName, version, description)

		if showVersions {
			versions, err := dbt.FetchToolVersions(tool.Name)
			if err != nil {
				err = errors.Wrapf(err, "failed to get versions of %s from %s", tool.Name, dbt.Config.Tools.Repo)
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

// FetchToolDescription fetches the tool description from the repository.
func (dbt *DBT) FetchToolDescription(tool string, version string) (description string, err error) {
	uri := fmt.Sprintf("%s/%s/%s/description.txt", dbt.Config.Tools.Repo, tool, version)

	isS3, s3Meta := S3Url(uri)

	if isS3 {
		return dbt.S3FetchDescription(s3Meta)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	dbt.VerboseOutput("Fetching tool description from  from %s", uri)

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request for url: %s", uri)
		return description, err
	}

	username := dbt.Config.Username
	password := dbt.Config.Password

	// Username func takes precedence over hardcoded username
	if dbt.Config.UsernameFunc != "" {
		username, err = GetFunc(dbt.Config.UsernameFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get username from shell function %q", dbt.Config.UsernameFunc)
			return description, err
		}
	}

	// PasswordFunc takes precedence over hardcoded password
	if dbt.Config.PasswordFunc != "" {
		password, err = GetFunc(dbt.Config.PasswordFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get password from shell function %q", dbt.Config.PasswordFunc)
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

// FetchToolNames returns a list of tool names found in the trusted repo
func (dbt *DBT) FetchToolNames() (tools []Tool, err error) {
	rawUrl := dbt.Config.Tools.Repo
	// strip off a trailing slash if there is one
	munged := strings.TrimSuffix(rawUrl, "/")
	// Then add one cos we definitely need one for http gets
	uri := fmt.Sprintf("%s/", munged)

	isS3, s3Meta := S3Url(uri)

	if isS3 {
		return dbt.S3FetchToolNames(s3Meta)
	}

	dbt.VerboseOutput("Fetching tool names from %s", uri)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request for url: %s", uri)
		return tools, err
	}

	username := dbt.Config.Username
	password := dbt.Config.Password

	// Username func takes precedence over hardcoded username
	if dbt.Config.UsernameFunc != "" {
		username, err = GetFunc(dbt.Config.UsernameFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get username from shell function %q", dbt.Config.UsernameFunc)
			return tools, err
		}
	}

	// PasswordFunc takes precedence over hardcoded password
	if dbt.Config.PasswordFunc != "" {
		password, err = GetFunc(dbt.Config.PasswordFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get password from shell function %q", dbt.Config.PasswordFunc)
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

// S3FetchDescription fetches the tool description from S3
func (dbt *DBT) S3FetchDescription(meta S3Meta) (description string, err error) {
	dbt.VerboseOutput("Fetching tool description from  from %s", meta.Url)
	downloader := s3manager.NewDownloader(dbt.S3Session)
	downloadOptions := &s3.GetObjectInput{
		Bucket: aws.String(meta.Bucket),
		Key:    aws.String(meta.Key),
	}

	buf := &aws.WriteAtBuffer{}

	_, err = downloader.Download(buf, downloadOptions)
	if err != nil {
		err = errors.Wrapf(err, "unable to download description from %s", meta.Url)
		return description, err
	}

	description = string(buf.Bytes())

	return description, err
}

// S3FetchTools fetches the list of available tools from S3
func (dbt *DBT) S3FetchToolNames(meta S3Meta) (tools []Tool, err error) {
	tools = make([]Tool, 0)
	uniqueTools := make(map[string]int)
	svc := s3.New(dbt.S3Session)

	dbt.VerboseOutput("Fetching tool names from %s", meta.Url)

	options := &s3.ListObjectsInput{
		Bucket:    aws.String(meta.Bucket),
		Prefix:    aws.String(meta.Key),
		Delimiter: aws.String("/"),
	}

	resp, err := svc.ListObjects(options)
	if err != nil {
		err = errors.Wrapf(err, "failed to list objects at %s", meta.Key)
		return tools, err
	}

	for _, k := range resp.Contents {
		dbt.VerboseOutput("  %s", *k.Key)
		uniqueTools[*k.Key] = 1
	}

	sorted := make([]string, 0)

	for k := range uniqueTools {
		sorted = append(sorted, k)
	}

	sort.Strings(sorted)

	for _, name := range sorted {
		tools = append(tools, Tool{Name: name})
	}

	return tools, err
}
