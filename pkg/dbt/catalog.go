package dbt

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
)

// defaultRequestTimeout is the default timeout for HTTP requests.
const defaultRequestTimeout = 10 * time.Second

// FetchCatalog shows you what tools are available in your trusted repo.  Repo is figured out from the config in ~/.dbt/conf/dbt.json.
//
//nolint:gocognit // catalog display logic requires multiple levels of iteration
func (dbt *DBT) FetchCatalog(showVersions bool) (err error) {
	fmt.Printf("Fetching information from the repository...\n")

	tools, toolsErr := dbt.FetchToolNames()
	if toolsErr != nil {
		err = errors.Wrap(toolsErr, "failed to fetch tools from repo")
		return err
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

	for _, tool := range tools {
		version, versionErr := dbt.FindLatestVersion(tool.Name)
		if versionErr != nil {
			err = errors.Wrapf(versionErr, "failed to get latest version of %s from %s", tool.Name, dbt.Config.Tools.Repo)
			return err
		}

		description, descErr := dbt.FetchToolDescription(tool.Name, version)
		if descErr != nil {
			err = errors.Wrapf(descErr, "Failed to get description of %s from %s", tool.Name, dbt.Config.Tools.Repo)
			return err
		}

		tool.FormattedName = fmt.Sprintf(formatstring, tool.Name, pad)

		fmt.Printf("\t%s\t\t%s\t\t\t%s\n", tool.FormattedName, version, description)

		if showVersions {
			versions, versionsErr := dbt.FetchToolVersions(tool.Name)
			if versionsErr != nil {
				err = errors.Wrapf(versionsErr, "failed to get versions of %s from %s", tool.Name, dbt.Config.Tools.Repo)
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
		description, err = dbt.S3FetchDescription(s3Meta)
		return description, err
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	dbt.VerboseOutput("Fetching tool description from  from %s", uri)

	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if reqErr != nil {
		err = errors.Wrapf(reqErr, "failed to create request for url: %s", uri)
		return description, err
	}

	err = dbt.AuthHeaders(req)
	if err != nil {
		err = errors.Wrapf(err, "failed adding auth headers")
		return description, err
	}

	resp, doErr := client.Do(req)

	if doErr != nil {
		err = errors.Wrapf(doErr, "Error looking for command description in repo %q", uri)
		return description, err
	}

	if resp != nil {
		defer resp.Body.Close()
		responseBytes, readErr := io.ReadAll(resp.Body)

		if readErr != nil {
			err = errors.Wrap(readErr, "Error reading description")
			return description, err
		}

		description = strings.TrimSpace(string(responseBytes))
	}

	return description, err
}

// FetchToolNames returns a list of tool names found in the trusted repo.
func (dbt *DBT) FetchToolNames() (tools []Tool, err error) {
	rawURL := dbt.Config.Tools.Repo
	// strip off a trailing slash if there is one
	munged := strings.TrimSuffix(rawURL, "/")
	// Then add one cos we definitely need one for http gets
	uri := fmt.Sprintf("%s/", munged)

	isS3, s3Meta := S3Url(uri)

	if isS3 {
		tools, err = dbt.S3FetchToolNames(s3Meta)
		return tools, err
	}

	dbt.VerboseOutput("Fetching tool names from %s", uri)

	client := &http.Client{
		Timeout: defaultRequestTimeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if reqErr != nil {
		err = errors.Wrapf(reqErr, "failed to create request for url: %s", uri)
		return tools, err
	}

	err = dbt.AuthHeaders(req)
	if err != nil {
		err = errors.Wrapf(err, "failed adding auth headers")
		return tools, err
	}

	resp, doErr := client.Do(req)

	tools = make([]Tool, 0)

	if doErr != nil {
		err = errors.Wrapf(doErr, "Error looking for commands in repo %q", uri)
		return tools, err
	}

	if resp != nil {
		defer resp.Body.Close()

		tools, err = parseToolNamesFromHTML(resp.Body)
	}

	return tools, err
}

// parseToolNamesFromHTML extracts tool names from an HTML directory listing.
//
//nolint:gocognit,unparam // HTML parsing requires nested loops; err kept for interface consistency
func parseToolNamesFromHTML(body io.Reader) (tools []Tool, err error) {
	tools = make([]Tool, 0)
	parser := html.NewTokenizer(body)

	for {
		tt := parser.Next()

		//nolint:exhaustive // only handling relevant token types
		switch tt {
		case html.ErrorToken:
			return tools, err
		case html.StartTagToken:
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
		default:
			// Skip other token types (TextToken, EndTagToken, etc.)
		}
	}
}

// Tool is a struct representing pertinent info on a dbt tool.
type Tool struct {
	Name          string
	FormattedName string
	Version       string
	Description   string
}

// S3FetchDescription fetches the tool description from S3.
func (dbt *DBT) S3FetchDescription(meta S3Meta) (description string, err error) {
	dbt.VerboseOutput("Fetching tool description from  from %s", meta.URL)
	downloader := s3manager.NewDownloader(dbt.S3Session)
	downloadOptions := &s3.GetObjectInput{
		Bucket: aws.String(meta.Bucket),
		Key:    aws.String(meta.Key),
	}

	buf := &aws.WriteAtBuffer{}

	_, err = downloader.Download(buf, downloadOptions)
	if err != nil {
		err = errors.Wrapf(err, "unable to download description from %s", meta.URL)
		return description, err
	}

	description = strings.TrimSpace(string(buf.Bytes()))

	return description, err
}

// S3FetchToolNames fetches the list of available tools from S3.
func (dbt *DBT) S3FetchToolNames(meta S3Meta) (tools []Tool, err error) {
	tools = make([]Tool, 0)
	svc := s3.New(dbt.S3Session)

	dbt.VerboseOutput("Fetching tool names from %s", meta.URL)

	options := &s3.ListObjectsInput{
		Bucket:    aws.String(meta.Bucket),
		Prefix:    aws.String(meta.Key),
		Delimiter: aws.String("/"),
	}

	resp, listErr := svc.ListObjects(options)
	if listErr != nil {
		err = errors.Wrapf(listErr, "failed to list objects at %s", meta.Key)
		dbt.VerboseOutput("Error: %s", err)
		return tools, err
	}

	for _, p := range resp.CommonPrefixes {
		name := *p.Prefix
		name = strings.TrimSuffix(name, "/")
		tools = append(tools, Tool{Name: name})
	}

	return tools, err
}
