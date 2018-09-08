package dbt

import (
	"bufio"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/net/html"
	"gopkg.in/cheggaaa/pb.v1"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ToolExists Returns true if a tool of the name input exists in the repository given.
func ToolExists(repoUrl string, toolName string) (found bool, err error) {
	var uri string

	if toolName == "" {
		uri = fmt.Sprintf("%s/", repoUrl)
	} else {
		uri = fmt.Sprintf("%s/%s", repoUrl, toolName)

	}

	resp, err := http.Get(uri)

	if err != nil {
		errors.Wrap(err, fmt.Sprintf("Failed to find tool in repo %q: %s", repoUrl, err))
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
func ToolVersionExists(repoUrl string, tool string, version string) bool {
	var uri string

	if tool == "" {
		uri = fmt.Sprintf("%s/%s/", repoUrl, version)

	} else {
		uri = fmt.Sprintf("%s/%s/%s/", repoUrl, tool, version)
	}
	resp, err := http.Get(uri)

	if err != nil {
		fmt.Println(fmt.Sprintf("Error looking for tool %q version %q in repo %q: %s", tool, version, uri, err))
	}

	if resp.StatusCode != 200 {
		return false
	}

	return true

}

// FetchToolVersions Given a repo and the name of the job, returns the available versions, and possibly an error if things didn't go well.
func FetchToolVersions(repoUrl string, tool string) (versions []string, err error) {
	var uri string

	if tool == "" {
		uri = fmt.Sprintf("%s/", repoUrl)

	} else {
		uri = fmt.Sprintf("%s/%s/", repoUrl, tool)
	}

	resp, err := http.Get(uri)

	if err != nil {
		fmt.Println(fmt.Sprintf("Error looking for versions of tool %q in repo %q: %s", tool, uri, err))
		return versions, err
	}

	if resp != nil {
		versions = ParseVersionResponse(resp)

		defer resp.Body.Close()

	}

	return versions, err
}

// ParseVersionResponse does an http get of an url and returns a list of semantic version links found at that place
func ParseVersionResponse(resp *http.Response) (versions []string) {
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
func FetchFile(fileUrl string, destPath string) (err error) {

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}

	err = os.Chmod(destPath, 0755)
	if err != nil {
		return err
	}

	defer out.Close()

	headResp, err := http.Head(fileUrl)

	if err != nil {
		panic(err)
	}

	defer headResp.Body.Close()

	sizeHeader := headResp.Header.Get("Content-Length")
	if sizeHeader == "" {
		sizeHeader = "0"
	}

	size, err := strconv.Atoi(sizeHeader)

	if err != nil {
		panic(err)
	}

	// create and start progress bar
	bar := pb.New(size).SetUnits(pb.U_BYTES)
	bar.Start()

	resp, err := http.Get(fileUrl)

	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Error fetching file from %q", fileUrl))
		return err
	}

	if resp != nil {
		defer resp.Body.Close()
		// create proxy reader
		reader := bar.NewProxyReader(resp.Body)

		// and copy from pb reader
		io.Copy(out, reader)

		_, err = io.Copy(out, resp.Body)

		if err != nil {
			return err
		}
	}

	return err
}

// VerifyFileChecksum Verifies the sha256 checksum of a given file against an expected value
func VerifyFileChecksum(filePath string, expected string) (success bool, err error) {
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

// VerifyFileVersion verifies the version by matching it's Sha1 checksum against what the repo says it should be
func VerifyFileVersion(repoUrl string, filePath string) (success bool, err error) {
	uri := fmt.Sprintf("%s.sha1", repoUrl)

	resp, err := http.Get(uri)
	if err != nil {
		fmt.Println(fmt.Sprintf("Error fetching checksum from %q: %s", uri, err))
	}

	if resp != nil {
		defer resp.Body.Close()

		checksumBytes, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			success = false
			return success, err
		}

		expected := string(checksumBytes)
		actual, err := FileSha1(filePath)

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
func VerifyFileSignature(homedir string, filePath string) (success bool, err error) {
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

// FindLatestVersion finds the latest version of the tool given in the repo given.  If the tool name is "", it is expecting to parse versions in the root of the repo.  I.e. there's only one tool in the repo.
func FindLatestVersion(repoUrl string, toolName string) (latest string, err error) {
	toolInRepo, err := ToolExists(repoUrl, toolName)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("error checking repo %s for tool %s", repoUrl, toolName))
		return latest, err
	}

	if toolInRepo {
		versions, err := FetchToolVersions(repoUrl, toolName)
		if err != nil {
			err = errors.Wrap(err, fmt.Sprintf("error getting versions for tool %s from repo %s", toolName, repoUrl))
			return latest, err
		}

		latest = LatestVersion(versions)
		return latest, err
	}

	fmt.Printf("tool not in repo\n")

	return latest, err
}
