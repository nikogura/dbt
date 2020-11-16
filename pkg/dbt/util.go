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
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

// StringInSlice returns true if the given string is in the given slice
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// SemverParse breaks apart a semantic version strings and returns a slice of int's holding the parts
func SemverParse(version string) (parts []int, err error) {
	stringParts := strings.Split(version, ".")

	for _, part := range stringParts {
		number, err := strconv.Atoi(part)
		if err != nil {
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

// VersionAIsNewerThanB returns true if Semantic Version string v1 is newer (higher numbers) than Semantic Version string v2
func VersionAIsNewerThanB(a string, b string) (result bool) {
	aParts, err := SemverParse(a)
	if err != nil {
		return false
	}

	bParts, err := SemverParse(b)
	if err != nil {
		return true
	}

	major := Spaceship(aParts[0], bParts[0])

	if major == 0 {
		minor := Spaceship(aParts[1], bParts[1])

		if minor == 0 {
			patch := Spaceship(aParts[2], bParts[2])

			if patch == 0 {
				return false
			}
			if patch > 0 {
				return true
			}
			return false

		}
		if minor > 0 {
			return true
		}
		return false

	}
	if major > 0 {
		return true
	}
	return false
}

// Spaceship A very simple implementation of a useful operator that go seems not to have.
// returns 1 if a > b, -1 if a < b, and 0 if a == b
func Spaceship(a int, b int) int {
	if a < b {
		return -1

	}
	if a > b {
		return 1
	}
	return 0
}

// FileSha256 returns the hex encoded Sha256 checksum for the given file
func FileSha256(fileName string) (checksum string, err error) {
	hasher := sha256.New()
	checksumBytes, err := ioutil.ReadFile(fileName)

	if err != nil {
		return checksum, err
	}

	_, err = hasher.Write(checksumBytes)

	if err != nil {
		return checksum, err
	}

	checksum = hex.EncodeToString(hasher.Sum(nil))

	return checksum, err
}

// FileSha1 returns the hex encoded Sha1 checksum for the given file
func FileSha1(fileName string) (checksum string, err error) {
	hasher := sha1.New()
	checksumBytes, err := ioutil.ReadFile(fileName)

	if err != nil {
		return checksum, err
	}

	_, err = hasher.Write(checksumBytes)

	if err != nil {
		return checksum, err
	}

	checksum = hex.EncodeToString(hasher.Sum(nil))

	return checksum, err
}

// FileCopy copies a single file from src to dst
func FileCopy(src, dst string) error {
	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer dstfd.Close()

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	return os.Chmod(dst, srcinfo.Mode())
}

// DirCopy copies a whole directory recursively
func DirCopy(src string, dst string) error {
	var err error
	var fds []os.FileInfo
	var srcinfo os.FileInfo

	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcinfo.Mode()); err != nil {
		return err
	}

	if fds, err = ioutil.ReadDir(src); err != nil {
		return err
	}
	for _, fd := range fds {
		srcfp := path.Join(src, fd.Name())
		dstfp := path.Join(dst, fd.Name())

		if fd.IsDir() {
			if err = DirCopy(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		} else {
			if err = FileCopy(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		}
	}
	return nil
}

// GetFunc runs a shell command that is a getter function.  This could certainly be dangerous, so be careful how you use it.
func GetFunc(shellCommand string) (result string, err error) {
	cmd := exec.Command("sh", "-c", shellCommand)

	stdout, err := cmd.StdoutPipe()

	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	cmd.Env = os.Environ()

	err = cmd.Start()
	if err != nil {
		err = errors.Wrapf(err, "failed to run %q", shellCommand)
		return result, err
	}

	stdoutBytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		err = errors.Wrapf(err, "error reading stdout from func")
		return result, err
	}

	err = cmd.Wait()
	if err != nil {
		err = errors.Wrapf(err, "error waiting for %q to exit", shellCommand)
		return result, err
	}

	result = strings.TrimSuffix(string(stdoutBytes), "\n")

	return result, err
}

// AuthHeaders Convenience function to add auth headers - basic or token for non-s3 requests.
func (dbt *DBT) AuthHeaders(r *http.Request) (err error) {
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

	return err
}
