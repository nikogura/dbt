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
	"cmp"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/nikogura/jwt-ssh-agent-go/pkg/agentjwt"
	"github.com/pkg/errors"
)

// SemverParse breaks apart a semantic version string and returns a slice of ints holding the parts.
func SemverParse(version string) (parts []int, err error) {
	stringParts := strings.Split(version, ".")

	for _, part := range stringParts {
		number, atoiErr := strconv.Atoi(part)
		if atoiErr != nil {
			err = atoiErr
			return parts, err
		}

		parts = append(parts, number)
	}

	return parts, err
}

// LatestVersion returns the latest version teased out of a list of semantic version strings.
func LatestVersion(versions []string) (latest string) {
	for _, version := range versions {
		if latest == "" {
			latest = version
		} else {

			if VersionAIsNewerThanB(latest, version) {
			} else {
				latest = version
			}
		}
	}

	return latest
}

// VersionAIsNewerThanB returns true if Semantic Version string a is newer (higher numbers) than Semantic Version string b.
func VersionAIsNewerThanB(a string, b string) (result bool) {
	aParts, aErr := SemverParse(a)
	if aErr != nil {
		result = false
		return result
	}

	bParts, bErr := SemverParse(b)
	if bErr != nil {
		result = true
		return result
	}

	major := cmp.Compare(aParts[0], bParts[0])

	//nolint:nestif // semantic version comparison requires nested major/minor/patch checks
	if major == 0 {
		minor := cmp.Compare(aParts[1], bParts[1])

		if minor == 0 {
			patch := cmp.Compare(aParts[2], bParts[2])

			if patch == 0 {
				result = false
				return result
			}
			if patch > 0 {
				result = true
				return result
			}
			result = false
			return result

		}
		if minor > 0 {
			result = true
			return result
		}
		result = false
		return result

	}
	if major > 0 {
		result = true
		return result
	}
	result = false
	return result
}

// FileSha256 returns the hex encoded Sha256 checksum for the given file.
func FileSha256(fileName string) (checksum string, err error) {
	hasher := sha256.New()
	checksumBytes, readErr := os.ReadFile(fileName)

	if readErr != nil {
		err = readErr
		return checksum, err
	}

	_, err = hasher.Write(checksumBytes)

	if err != nil {
		return checksum, err
	}

	checksum = hex.EncodeToString(hasher.Sum(nil))

	return checksum, err
}

// FileSha1 returns the hex encoded Sha1 checksum for the given file.
func FileSha1(fileName string) (checksum string, err error) {
	hasher := sha1.New()
	checksumBytes, readErr := os.ReadFile(fileName)

	if readErr != nil {
		err = readErr
		return checksum, err
	}

	_, err = hasher.Write(checksumBytes)

	if err != nil {
		return checksum, err
	}

	checksum = hex.EncodeToString(hasher.Sum(nil))

	return checksum, err
}

// FileCopy copies a single file from src to dst.
func FileCopy(src, dst string) (err error) {
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	srcfd, err = os.Open(src)
	if err != nil {
		return err
	}
	defer srcfd.Close()

	dstfd, err = os.Create(dst)
	if err != nil {
		return err
	}
	defer dstfd.Close()

	_, err = io.Copy(dstfd, srcfd)
	if err != nil {
		return err
	}
	srcinfo, err = os.Stat(src)
	if err != nil {
		return err
	}
	err = os.Chmod(dst, srcinfo.Mode())
	return err
}

// DirCopy copies a whole directory recursively.
func DirCopy(src string, dst string) (err error) {
	var fds []os.DirEntry
	var srcinfo os.FileInfo

	srcinfo, err = os.Stat(src)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dst, srcinfo.Mode())
	if err != nil {
		return err
	}

	fds, err = os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, fd := range fds {
		srcfp := path.Join(src, fd.Name())
		dstfp := path.Join(dst, fd.Name())

		if fd.IsDir() {
			copyErr := DirCopy(srcfp, dstfp)
			if copyErr != nil {
				fmt.Println(copyErr)
			}
		} else {
			copyErr := FileCopy(srcfp, dstfp)
			if copyErr != nil {
				fmt.Println(copyErr)
			}
		}
	}
	return err
}

// GetFunc runs a shell command that is a getter function.  This could certainly be dangerous, so be careful how you use it.
func GetFunc(shellCommand string) (result string, err error) {
	cmd := exec.CommandContext(context.Background(), "sh", "-c", shellCommand)

	stdout, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		err = errors.Wrapf(pipeErr, "failed to get stdout pipe for %q", shellCommand)
		return result, err
	}

	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	cmd.Env = os.Environ()

	startErr := cmd.Start()
	if startErr != nil {
		err = errors.Wrapf(startErr, "failed to run %q", shellCommand)
		return result, err
	}

	stdoutBytes, readErr := io.ReadAll(stdout)
	if readErr != nil {
		err = errors.Wrapf(readErr, "error reading stdout from func")
		return result, err
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		err = errors.Wrapf(waitErr, "error waiting for %q to exit", shellCommand)
		return result, err
	}

	result = strings.TrimSuffix(string(stdoutBytes), "\n")

	return result, err
}

// GetFuncUsername runs a shell command that is a getter function for the username.  This could certainly be dangerous, so be careful how you use it.
func GetFuncUsername(shellCommand string, username string) (result []string, err error) {
	// add the username as the first arg of the shell command
	shellCommand = fmt.Sprintf("%s %s", shellCommand, username)

	cmd := exec.CommandContext(context.Background(), "sh", "-c", shellCommand)

	stdout, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		err = errors.Wrapf(pipeErr, "failed to get stdout pipe for %q", shellCommand)
		return result, err
	}

	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	cmd.Env = os.Environ()

	startErr := cmd.Start()
	if startErr != nil {
		err = errors.Wrapf(startErr, "failed to run %q", shellCommand)
		return result, err
	}

	stdoutBytes, readErr := io.ReadAll(stdout)
	if readErr != nil {
		err = errors.Wrapf(readErr, "error reading stdout from func")
		return result, err
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		err = errors.Wrapf(waitErr, "error waiting for %q to exit", shellCommand)
		return result, err
	}

	result = make([]string, 0)

	result = append(result, strings.TrimSuffix(string(stdoutBytes), "\n"))

	return result, err
}

// AuthHeaders is a convenience function to add auth headers - basic or token for non-s3 requests.
//
//nolint:gocognit // auth header logic requires multiple configuration checks
func (dbt *DBT) AuthHeaders(r *http.Request) (err error) {
	// OIDC Auth - if configured, use OIDC token in Authorization header
	if dbt.OIDCClient != nil {
		token, tokenErr := dbt.OIDCClient.GetToken(r.Context())
		if tokenErr != nil {
			err = errors.Wrap(tokenErr, "failed to get OIDC token")
			return err
		}
		r.Header.Set("Authorization", "Bearer "+token)
		return err
	}

	// Basic Auth
	// start with values hardcoded in the config file
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
		r.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}

	// Pubkey JWT Auth
	// Start with username and pubkey hardcoded in the config
	pubkey := dbt.Config.Pubkey

	// read pubkey from file
	if dbt.Config.PubkeyPath != "" {
		b, readErr := os.ReadFile(dbt.Config.PubkeyPath)
		if readErr != nil {
			err = errors.Wrapf(readErr, "failed to read public key from file %s", dbt.Config.PubkeyPath)
			return err
		}

		pubkey = string(b)

	}

	// PubkeyFunc takes precedence over files and hardcoding
	if dbt.Config.PubkeyFunc != "" {
		pubkey, err = GetFunc(dbt.Config.PubkeyFunc)
		if err != nil {
			err = errors.Wrapf(err, "failed to get public key from shell function %q", dbt.Config.PubkeyFunc)
			return err
		}
	}

	// Extract the domain from the repo server
	domain, domainErr := ExtractDomain(dbt.Config.Dbt.Repo)
	if domainErr != nil {
		err = errors.Wrapf(domainErr, "failed extracting domain from configured dbt repo url %s", dbt.Config.Dbt.Repo)
		return err
	}

	// Don't try to sign a token if we don't actually have a public key
	if pubkey != "" {
		// use username and pubkey to set Token header
		// Note: Updated for jwt-ssh-agent-go upgrade - subject should be username, audience should be domain
		token, signErr := agentjwt.SignedJwtToken(username, domain, pubkey)
		if signErr != nil {
			err = errors.Wrapf(signErr, "failed to sign JWT token")
			return err
		}

		if token != "" {
			r.Header.Add("Token", token)
		}
	}

	return err
}

func ExtractDomain(urlLikeString string) (domain string, err error) {
	urlLikeString = strings.TrimSpace(urlLikeString)

	if regexp.MustCompile(`^https?`).MatchString(urlLikeString) {
		read, _ := url.Parse(urlLikeString)
		urlLikeString = read.Host
	}

	if regexp.MustCompile(`^www\.`).MatchString(urlLikeString) {
		urlLikeString = regexp.MustCompile(`^www\.`).ReplaceAllString(urlLikeString, "")
	}

	domain = regexp.MustCompile(`([a-z0-9\-]+\.)*[a-z0-9\-]+`).FindString(urlLikeString)
	if domain == "" {
		err = errors.New(fmt.Sprintf("failed parsing domain from %s", urlLikeString))
		return domain, err
	}

	return domain, err
}
