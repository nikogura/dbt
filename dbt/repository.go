package dbt

import (
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/net/html"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// ToolExists Returns true if a tool of the name input exists in the repository given.
func ToolExists(repoUrl string, toolName string) (found bool, err error) {
	uri := fmt.Sprintf("%s/%s", repoUrl, toolName)
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
	uri := fmt.Sprintf("%s/%s/%s", repoUrl, tool, version)
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
	uri := fmt.Sprintf("%s/%s/", repoUrl, tool)

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
							// any links beyond the 'back' link will be versions
							// trim the trailing slash so we get actual semantic versions
							// TODO extend ParseVersionResponse to return only semantic version strings
							versions = append(versions, strings.TrimRight(a.Val, "/"))
						}
					}
				}
			}
		}
	}

	return versions
}

// FetchFile Fetches a file and places it on the filesystem.
// Does not validate the signature.  That's a different step.
func FetchFile(repoUrl string, destPath string) (err error) {

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}

	err = os.Chmod(destPath, 0755)
	if err != nil {
		return err
	}

	defer out.Close()

	resp, err := http.Get(repoUrl)

	if err != nil {
		fmt.Println(fmt.Sprintf("Error fetching binary from %q: %s", repoUrl, err))
	}

	if resp != nil {
		defer resp.Body.Close()

		_, err = io.Copy(out, resp.Body)

		if err != nil {
			return err
		}
	}

	return err
}

// Verifies the sha256 checksum of a given file against an expected value
func VerifyFileChecksum(filePath string, expected string) (success bool, err error) {
	checksum, err := FileSha256(filePath)
	if err != nil {
		success = false
		return success, err
	}

	if checksum == expected {
		success = true
		return success, err
	} else {
		success = false
		return success, err
	}
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
		} else {
			success = false
			return success, err
		}
	}

	return success, err
}

func VerifyFileSignature(filePath string) (success bool, err error) {
	signature, err := os.Open(filePath)

	if err != nil {
		return false, err
	}

	homedir, err := GetHomeDir()
	if err != nil {
		err = errors.Wrapf(err, "failed to get user home dir")
		return false, err
	}

	truststoreFileName := fmt.Sprintf("%s/%s", homedir, truststorePath)

	keyRingReader, err := os.Open(truststoreFileName)

	if err != nil {
		return false, err
	}

	target, err := os.Open(filePath)

	if err != nil {
		return false, err
	}

	keyring, err := openpgp.ReadArmoredKeyRing(keyRingReader)
	if err != nil {
		return false, err
	}

	entity, err := openpgp.CheckArmoredDetachedSignature(keyring, target, signature)

	if err != nil {
		return false, err
	}

	if entity != nil {
		return true, err
	}

	return false, err
}
