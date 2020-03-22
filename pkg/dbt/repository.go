// Copyright © 2019 Nik Ogura <nik.ogura@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbt

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/net/html"
	"gopkg.in/cheggaaa/pb.v1"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AWS_ID_ENV_VAR Default env var for AWS access key
const AWS_ID_ENV_VAR = "AWS_ACCESS_KEY_ID"

// AWS_SECRET_ENV_VAR Default env var for AWS secret key
const AWS_SECRET_ENV_VAR = "AWS_SECRET_ACCESS_KEY"

// AWS_REGION_ENV_VAR Default env var for AWS region
const AWS_REGION_ENV_VAR = "AWS_DEFAULT_REGION"

// NOPROGRESS turns off the progress bar on file fetches.  Primarily used for testing to avoid cluttering up the output and confusing the test harness.
var NOPROGRESS = false

// ToolExists Returns true if a tool of the name input exists in the repository given.
func (dbt *DBT) ToolExists(toolName string) (found bool, err error) {
	var uri string
	var repoUrl string

	if toolName == "" {
		repoUrl = dbt.Config.Dbt.Repo
		uri = fmt.Sprintf("%s/", repoUrl)
	} else {
		repoUrl = dbt.Config.Tools.Repo
		uri = fmt.Sprintf("%s/%s/", repoUrl, toolName)
	}

	isS3, s3Meta := S3Url(uri)

	if isS3 {
		return dbt.S3ToolExists(toolName, s3Meta)
	}

	client := &http.Client{}

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request for url: %s", uri)
		return found, err
	}

	username := dbt.Config.Username
	password := dbt.Config.Password

	// Username func takes precedence over hardcoded username
	if dbt.Config.UsernameFunc != "" {
		username, err = GetFunc(dbt.Config.UsernameFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get username from shell function %q", dbt.Config.UsernameFunc)
			return found, err
		}
	}

	// PasswordFunc takes precedence over hardcoded password
	if dbt.Config.PasswordFunc != "" {
		password, err = GetFunc(dbt.Config.PasswordFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get password from shell function %q", dbt.Config.PasswordFunc)
			return found, err
		}
	}

	if username != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}

	resp, err := client.Do(req)

	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Failed to find tool in repo %q: %s", repoUrl, err))
		return false, err
	}

	if resp != nil {
		if resp.StatusCode != 200 {
			return false, err
		}
	} else {
		return false, err
	}

	return true, err
}

// ToolVersionExists returns true if the specified version of a tool is in the repo
func (dbt *DBT) ToolVersionExists(tool string, version string) (ok bool, err error) {
	var uri string

	repoUrl := dbt.Config.Tools.Repo

	if tool == "" {
		uri = fmt.Sprintf("%s/%s/", repoUrl, version)

	} else {
		uri = fmt.Sprintf("%s/%s/%s/", repoUrl, tool, version)
	}

	isS3, s3Meta := S3Url(uri)

	if isS3 {
		return dbt.S3ToolVersionExists(s3Meta)
	}

	client := &http.Client{}

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request for url: %s", uri)
		return ok, err
	}

	username := dbt.Config.Username
	password := dbt.Config.Password

	// Username func takes precedence over hardcoded username
	if dbt.Config.UsernameFunc != "" {
		username, err = GetFunc(dbt.Config.UsernameFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get username from shell function %q", dbt.Config.UsernameFunc)
			return ok, err
		}
	}

	// PasswordFunc takes precedence over hardcoded password
	if dbt.Config.PasswordFunc != "" {
		password, err = GetFunc(dbt.Config.PasswordFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get password from shell function %q", dbt.Config.PasswordFunc)
			return ok, err
		}
	}

	if username != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}

	resp, err := client.Do(req)
	if err != nil {
		err = errors.Wrapf(err, "Error looking for tool %q version %q in repo %q", tool, version, uri)
		return ok, err
	}

	if resp.StatusCode != 200 {
		return ok, err
	}

	ok = true

	return ok, err
}

// FetchToolVersions Given the name of a tool, returns the available versions, and possibly an error if things didn't go well.  If tool name is "", fetches versions of dbt itself.
func (dbt *DBT) FetchToolVersions(toolName string) (versions []string, err error) {
	var uri string
	var repoUrl string

	if toolName == "" {
		repoUrl = dbt.Config.Dbt.Repo
		uri = fmt.Sprintf("%s/", repoUrl)
	} else {
		repoUrl = dbt.Config.Tools.Repo
		uri = fmt.Sprintf("%s/%s/", repoUrl, toolName)
	}

	isS3, s3Meta := S3Url(uri)

	if isS3 {
		return dbt.S3FetchToolVersions(s3Meta)
	}

	client := &http.Client{}

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request for url: %s", uri)
		return versions, err
	}

	username := dbt.Config.Username
	password := dbt.Config.Password

	// Username func takes precedence over hardcoded username
	if dbt.Config.UsernameFunc != "" {
		username, err = GetFunc(dbt.Config.UsernameFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get username from shell function %q", dbt.Config.UsernameFunc)
			return versions, err
		}
	}

	// PasswordFunc takes precedence over hardcoded password
	if dbt.Config.PasswordFunc != "" {
		password, err = GetFunc(dbt.Config.PasswordFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get password from shell function %q", dbt.Config.PasswordFunc)
			return versions, err
		}
	}

	if username != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}

	resp, err := client.Do(req)
	if err != nil {
		err = errors.Wrapf(err, "Error looking for versions of tool %q in repo %q", toolName, uri)
		return versions, err
	}

	if resp != nil {
		versions = dbt.ParseVersionResponse(resp)

		defer resp.Body.Close()

	}

	return versions, err
}

// ParseVersionResponse does an http get of an url and returns a list of semantic version links found at that place
func (dbt *DBT) ParseVersionResponse(resp *http.Response) (versions []string) {
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
							// trim the trailing slash so we get actual semantic versions
							version := strings.TrimRight(a.Val, "/")

							// there could be other files, we only want things that look like semantic versions
							semverMatch := regexp.MustCompile(`^\d+\.\d+\.\d+$`)

							if semverMatch.MatchString(version) {
								versions = append(versions, version)

							}
						}
					}
				}
			}
		}
	}
}

// FetchFile Fetches a file and places it on the filesystem.
// Does not validate the signature.  That's a different step.
func (dbt *DBT) FetchFile(fileUrl string, destPath string) (err error) {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}

	err = os.Chmod(destPath, 0755)
	if err != nil {
		return err
	}

	defer out.Close()

	// Check to see if this is an S3 URL
	isS3, s3Meta := S3Url(fileUrl)

	if isS3 {
		return dbt.S3FetchFile(fileUrl, s3Meta, out)
	}

	client := &http.Client{
		Timeout: 300 * time.Second,
	}

	req, err := http.NewRequest("HEAD", fileUrl, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request for url: %s", fileUrl)
		return err
	}

	username := dbt.Config.Username
	password := dbt.Config.Password

	// Username func takes precedence over hardcoded username
	if dbt.Config.UsernameFunc != "" {
		username, err = GetFunc(dbt.Config.UsernameFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get username from shell function %q", dbt.Config.UsernameFunc)
			return err
		}
	}

	// PasswordFunc takes precedence over hardcoded password
	if dbt.Config.PasswordFunc != "" {
		password, err = GetFunc(dbt.Config.PasswordFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get password from shell function %q", dbt.Config.PasswordFunc)
			return err
		}
	}

	if username != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}

	var bar *pb.ProgressBar

	if !NOPROGRESS {
		headResp, err := client.Do(req)
		defer headResp.Body.Close()

		if err != nil {
			err = errors.Wrapf(err, "error making request to %s", fileUrl)
			return err
		}

		if headResp.StatusCode > 399 {
			err = errors.New(fmt.Sprintf("unable to request headers for %s: %d %s", fileUrl, headResp.StatusCode, headResp.Status))
			return err
		}

		sizeHeader := headResp.Header.Get("Content-Length")
		if sizeHeader == "" {
			sizeHeader = "0"
		}

		size, err := strconv.Atoi(sizeHeader)
		if err != nil {
			err = errors.Wrap(err, "unable to convert Content-Length header to integer")
			return err
		}

		// create and start progress bar
		bar = pb.New(size).SetUnits(pb.U_BYTES)
		bar.Output = os.Stderr
		bar.Start()

	}

	req, err = http.NewRequest("GET", fileUrl, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request for url: %s", fileUrl)
		return err
	}

	if username != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}

	resp, err := client.Do(req)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Error fetching file from %q", fileUrl))
		return err
	}

	if resp.StatusCode > 399 {
		err = errors.New(fmt.Sprintf("unable to make request to %s: %d %s", fileUrl, resp.StatusCode, resp.Status))
		return err
	}

	if resp != nil {
		defer resp.Body.Close()

		if bar != nil {
			// create proxy reader
			reader := bar.NewProxyReader(resp.Body)

			// and copy from pb reader
			_, _ = io.Copy(out, reader)
		}

		_, err = io.Copy(out, resp.Body)

		if err != nil {
			return err
		}
	}

	return err
}

// VerifyFileChecksum Verifies the sha256 checksum of a given file against an expected value
func (dbt *DBT) VerifyFileChecksum(filePath string, expected string) (success bool, err error) {
	checksum, err := FileSha256(filePath)
	if err != nil {
		success = false
		return success, err
	}

	if checksum == expected {
		success = true
		return success, err
	}

	success = false
	return success, err
}

// VerifyFileVersion verifies the version by matching it's Sha256 checksum against what the repo says it should be
func (dbt *DBT) VerifyFileVersion(fileUrl string, filePath string) (success bool, err error) {
	uri := fmt.Sprintf("%s.sha256", fileUrl)

	// Check to see if this is an S3 URL
	isS3, s3Meta := S3Url(uri)

	if isS3 {
		return dbt.S3VerifyFileVersion(filePath, s3Meta)
	}

	client := &http.Client{}

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request for url: %s", uri)
		return success, err
	}

	username := dbt.Config.Username
	password := dbt.Config.Password

	// Username func takes precedence over hardcoded username
	if dbt.Config.UsernameFunc != "" {
		username, err = GetFunc(dbt.Config.UsernameFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get username from shell function %q", dbt.Config.UsernameFunc)
			return success, err
		}
	}

	// PasswordFunc takes precedence over hardcoded password
	if dbt.Config.PasswordFunc != "" {
		password, err = GetFunc(dbt.Config.PasswordFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get password from shell function %q", dbt.Config.PasswordFunc)
			return success, err
		}
	}

	if username != "" && password != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}

	resp, err := client.Do(req)
	if err != nil {
		err = errors.Wrapf(err, "Error fetching checksum from %q", uri)
		return success, err
	}

	if resp != nil {
		defer resp.Body.Close()

		checksumBytes, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			success = false
			return success, err
		}

		expected := string(checksumBytes)
		actual, err := FileSha256(filePath)

		if err != nil {
			success = false
			return success, err
		}

		if actual == expected {
			success = true
			return success, err
		}

		success = false
		return success, err
	}

	return success, err
}

// VerifyFileSignature verifies the signature on the given file
func (dbt *DBT) VerifyFileSignature(homedir string, filePath string) (success bool, err error) {
	if homedir == "" {
		homedir, err = GetHomeDir()
		if err != nil {
			err = errors.Wrapf(err, "failed to get homedir")
			return success, err
		}
	}

	sigFile := fmt.Sprintf("%s.asc", filePath)

	truststoreFileName := fmt.Sprintf("%s/%s", homedir, TruststorePath)

	truststore, err := os.Open(truststoreFileName)
	if err != nil {
		err = errors.Wrap(err, "failed to open truststore file")
		return false, err
	}

	defer truststore.Close()

	// openpgp.CheckArmoredDetatchedSignature doesn't actually check multiple certs, so we have to split the truststore file
	// and check each cert individually

	endToken := "-----END PGP PUBLIC KEY BLOCK-----"

	certs := make([]string, 0)

	scanner := bufio.NewScanner(truststore)

	cert := ""

	for scanner.Scan() {
		line := scanner.Text()
		cert += fmt.Sprintf("%s\n", line)

		if line == endToken {
			certs = append(certs, cert)
			cert = ""
		}
	}

	for _, cert := range certs {
		entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(cert))
		if err != nil {
			err = errors.Wrap(err, "failed to read cert from truststore")
			return false, err
		}

		signature, err := os.Open(sigFile)
		if err != nil {
			err = errors.Wrap(err, "failed to open signature file")
			return false, err
		}

		defer signature.Close()

		target, err := os.Open(filePath)
		if err != nil {
			err = errors.Wrap(err, "failed to open target file")
			return false, err
		}

		defer target.Close()

		entity, _ := openpgp.CheckArmoredDetachedSignature(entities, target, signature)
		if entity != nil {
			return true, nil
		}
	}

	err = fmt.Errorf("signing entity not in truststore")
	return false, err
}

// FindLatestVersion finds the latest version of the tool available in the tool repo.  If the tool name is "", it is expecting to parse versions of dbt itself.
func (dbt *DBT) FindLatestVersion(toolName string) (latest string, err error) {
	toolInRepo, err := dbt.ToolExists(toolName)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("error checking repo for tool %s", toolName))
		return latest, err
	}

	if toolInRepo {
		versions, err := dbt.FetchToolVersions(toolName)
		if err != nil {
			err = errors.Wrap(err, fmt.Sprintf("error getting versions for tool %s", toolName))
			return latest, err
		}

		latest = LatestVersion(versions)
		return latest, err
	}

	err = errors.New("tool not in repo")

	return latest, err
}

// DefaultSession creates a default AWS session from local config path.  Hooks directly into credentials if present, or Credentials Provider if configured.
func DefaultSession() (awssession *session.Session, err error) {
	if os.Getenv(AWS_ID_ENV_VAR) == "" && os.Getenv(AWS_SECRET_ENV_VAR) == "" {
		_ = os.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	}

	awssession, err = session.NewSession()
	if err != nil {
		log.Fatalf("Failed to create aws session")
	}

	// For some reason this doesn't get picked up automatically, but we'll set it if it's present in the environment.
	if os.Getenv(AWS_REGION_ENV_VAR) != "" {
		awssession.Config.Region = aws.String(os.Getenv(AWS_REGION_ENV_VAR))
	}

	return awssession, err
}

// DirsForURL given a URL, return a list of path elements suitable for creating directories/ folders
func DirsForURL(uri string) (dirs []string, err error) {
	dirs = make([]string, 0)

	u, err := url.Parse(uri)
	if err != nil {
		err = errors.Wrapf(err, "failed to parse %s", uri)
		return dirs, err
	}

	dir := path.Dir(u.Path)
	dir = strings.TrimPrefix(dir, "/")
	parts := strings.Split(dir, "/")

	for len(parts) > 0 {
		dirs = append(dirs, strings.Join(parts, "/"))
		parts = parts[:len(parts)-1]
	}

	// Reverse the order, as this will be much easier to use the data to do path creation
	for i := len(dirs)/2 - 1; i >= 0; i-- {
		opp := len(dirs) - 1 - i
		dirs[i], dirs[opp] = dirs[opp], dirs[i]
	}

	return dirs, err
}

// S3Meta a struct for holding metadata for S3 Objects.  There's probably already a struct that holds this, but this is all I need.
type S3Meta struct {
	Bucket string
	Region string
	Key    string
	Url    string
}

// S3Url returns true, and a metadata struct if the url given appears to be in s3
func S3Url(url string) (ok bool, meta S3Meta) {
	// Check to see if it's an s3 URL.
	s3Url := regexp.MustCompile(`https?://(.*)\.s3\.(.*)\.amazonaws.com/?(.*)?`)

	matches := s3Url.FindAllStringSubmatch(url, -1)

	if len(matches) == 0 {
		return ok, meta
	}

	match := matches[0]

	if len(match) == 3 {
		meta = S3Meta{
			Bucket: match[1],
			Region: match[2],
			Url:    url,
		}

		ok = true
		return ok, meta

	} else if len(match) == 4 {
		meta = S3Meta{
			Bucket: match[1],
			Region: match[2],
			Key:    match[3],
			Url:    url,
		}

		ok = true
		return ok, meta
	}

	return ok, meta
}

// S3FetchFile fetches a file out of S3 instead of using a normal HTTP GET
func (dbt *DBT) S3FetchFile(fileUrl string, meta S3Meta, outFile *os.File) (err error) {
	headOptions := &s3.HeadObjectInput{
		Bucket: aws.String(meta.Bucket),
		Key:    aws.String(meta.Key),
	}

	headSvc := s3.New(dbt.S3Session)

	fileMeta, err := headSvc.HeadObject(headOptions)
	if err != nil {
		err = errors.Wrapf(err, "failed to get metadata for %s", fileUrl)
		return err
	}

	downloader := s3manager.NewDownloader(dbt.S3Session)
	downloadOptions := &s3.GetObjectInput{
		Bucket: aws.String(meta.Bucket),
		Key:    aws.String(meta.Key),
	}

	if !NOPROGRESS {
		// create and start progress bar
		bar := pb.New(int(*fileMeta.ContentLength)).SetUnits(pb.U_BYTES)
		bar.Output = os.Stderr
		bar.Start()

		buf := &aws.WriteAtBuffer{}

		_, err = downloader.Download(buf, downloadOptions)
		if err != nil {
			err = errors.Wrapf(err, "unable to download file from %s", fileUrl)
			return err
		}

		// create proxy reader
		reader := bar.NewProxyReader(bytes.NewBuffer(buf.Bytes()))

		// and copy from pb reader
		_, _ = io.Copy(outFile, reader)

		_, err = io.Copy(outFile, bytes.NewReader(buf.Bytes()))

		return err
	}

	_, err = downloader.Download(outFile, downloadOptions)
	if err != nil {
		err = errors.Wrapf(err, "download failed")
		return err
	}

	return err

}

// S3ToolExists detects whether a tool exists in S3 by looking at the top level folder for the tool
func (dbt *DBT) S3ToolExists(toolName string, meta S3Meta) (found bool, err error) {
	svc := s3.New(dbt.S3Session)
	options := &s3.ListObjectsInput{
		Bucket:    aws.String(meta.Bucket),
		Prefix:    aws.String(meta.Key),
		Delimiter: aws.String("/"),
	}

	resp, err := svc.ListObjects(options)
	if err != nil {
		err = errors.Wrapf(err, "failed to list objects at %s", meta.Key)
		return found, err
	}

	if len(resp.Contents) > 0 {
		found = true
	}

	return found, err
}

// S3FetchTruststore fetches the truststore out of S3 writing it into the dbt dir on the local disk
func (dbt *DBT) S3FetchTruststore(homedir string, meta S3Meta, verbose bool) (err error) {
	downloader := s3manager.NewDownloader(dbt.S3Session)
	filePath := fmt.Sprintf("%s/%s", homedir, TruststorePath)
	file, err := os.Create(filePath)
	if err != nil {
		err = errors.Wrapf(err, "Failed opening truststore file %s", filePath)
		return err
	}
	_, err = downloader.Download(file, &s3.GetObjectInput{
		Bucket: aws.String(meta.Bucket),
		Key:    aws.String(meta.Key),
	})

	if err != nil {
		err = errors.Wrapf(err, "failed to download truststore from %s", meta.Url)
	}

	return err
}

// S3ToolVersionExists returns true if the tool version exists
func (dbt *DBT) S3ToolVersionExists(meta S3Meta) (ok bool, err error) {
	headOptions := &s3.HeadObjectInput{
		Bucket: aws.String(meta.Bucket),
		Key:    aws.String(meta.Key),
	}

	log.Printf("Looking for %q in %s", meta.Key, meta.Bucket)

	headSvc := s3.New(dbt.S3Session)

	// not found is an error, as opposed to a successful request that has a 404 code
	_, fetchErr := headSvc.HeadObject(headOptions)
	if fetchErr != nil {
		return ok, err
	}

	ok = true

	return ok, err
}

// S3VerifyFileVersion verifies the version of a file on the filesystem matches the sha256 hash stored in the s3 bucket for that file
func (dbt *DBT) S3VerifyFileVersion(filePath string, meta S3Meta) (success bool, err error) {
	// get checksum file from s3
	buff := &aws.WriteAtBuffer{}
	downloader := s3manager.NewDownloader(dbt.S3Session)
	_, err = downloader.Download(buff, &s3.GetObjectInput{
		Bucket: aws.String(meta.Bucket),
		Key:    aws.String(meta.Key),
	})

	if err != nil {
		err = errors.Wrapf(err, "failed to download checksum for %s", meta.Url)
		return success, err
	}

	// compare it to what's on the disk
	expected := string(buff.Bytes())
	actual, err := FileSha256(filePath)

	if err != nil {
		success = false
		return success, err
	}

	if actual == expected {
		success = true
		return success, err
	}

	return success, err
}

// S3FetchToolVersions fetches available versions for a tool from S3
func (dbt *DBT) S3FetchToolVersions(meta S3Meta) (versions []string, err error) {
	versions = make([]string, 0)
	uniqueVersions := make(map[string]int)
	svc := s3.New(dbt.S3Session)

	options := &s3.ListObjectsInput{
		Bucket: aws.String(meta.Bucket),
		Prefix: aws.String(meta.Key),
	}

	resp, err := svc.ListObjects(options)
	if err != nil {
		err = errors.Wrapf(err, "failed to list objects at %s", meta.Key)
		return versions, err
	}

	dir := regexp.MustCompile(`\d+\.\d+\.\d+/`)
	semver := regexp.MustCompile(`\d+\.\d+\.\d+`)

	for _, k := range resp.Contents {
		if dir.MatchString(*k.Key) {
			parts := strings.Split(*k.Key, "/")
			if len(parts) > 0 {
				if semver.MatchString(parts[0]) {
					uniqueVersions[parts[0]] = 1
				} else if len(parts) > 1 {
					if semver.MatchString(parts[1]) {
						uniqueVersions[parts[1]] = 1
					}
				}
			}
		}
	}

	for k := range uniqueVersions {
		versions = append(versions, k)
	}

	return versions, err
}
