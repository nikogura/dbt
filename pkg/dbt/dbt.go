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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

// DbtDir is the standard dbt directory.  Usually ~/.dbt
const DbtDir = ".dbt"

// TrustDir is the directory under the dbt dir where the trust store is downloaded to
const TrustDir = DbtDir + "/trust"

// ToolDir is the directory where tools get downloaded to
const ToolDir = DbtDir + "/tools"

// ConfigDir is the directory where Dbt expects to find configuration info
const ConfigDir = DbtDir + "/conf"

// ConfigFilePath is the actual dbt config file path
const ConfigFilePath = ConfigDir + "/dbt.json"

// TruststorePath is the actual file path to the downloaded trust store
const TruststorePath = TrustDir + "/truststore"

// DBT the dbt object itself
type DBT struct {
	Config  Config
	Verbose bool
	Logger  *log.Logger
}

// Config  configuration of the dbt object
type Config struct {
	Dbt          DbtConfig   `json:"dbt"`
	Tools        ToolsConfig `json:"tools"`
	Username     string      `json:"username,omitempty"`
	Password     string      `json:"password,omitempty"`
	UsernameFunc string      `json:"usernamefunc,omitempty"`
	PasswordFunc string      `json:"passwordfunc,omitempty"`
}

// DbtConfig internal config of dbt
type DbtConfig struct {
	Repo       string `json:"repository"`
	TrustStore string `json:"truststore"`
}

// ToolsConfig is the config information for the tools to be downloaded and run
type ToolsConfig struct {
	Repo string `json:"repository"`
}

// NewDbt  creates a new dbt object
func NewDbt() (dbt *DBT, err error) {
	config, err := LoadDbtConfig("", false)
	if err != nil {
		err = errors.Wrapf(err, "failed to load config file")
	}

	dbt = &DBT{
		Config:  config,
		Verbose: false,
		Logger:  log.New(os.Stderr, "", 0),
	}

	return dbt, err
}

// LoadDbtConfig loads the dbt config from the expected location on the filesystem
func LoadDbtConfig(homedir string, verbose bool) (config Config, err error) {
	if homedir == "" {
		homedir, err = GetHomeDir()
		if err != nil {
			err = errors.Wrapf(err, "failed to get homedir")
			return config, err
		}
	}

	logger := log.New(os.Stderr, "", 0)

	if verbose {
		logger.Printf("Creating DBT directory in %s/.dbt", homedir)
	}

	filePath := fmt.Sprintf("%s/%s", homedir, ConfigFilePath)

	if verbose {
		logger.Printf("Loading config from %s", filePath)
	}

	mdBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(mdBytes, &config)
	if err != nil {
		return config, err
	}

	return config, err
}

// GenerateDbtDir generates the necessary dbt dirs in the user's homedir if they don't already exist.  If they do exist, it does nothing.
func GenerateDbtDir(homedir string, verbose bool) (err error) {
	if homedir == "" {
		homedir, err = GetHomeDir()
		if err != nil {
			err = errors.Wrapf(err, "failed to get homedir")
			return err
		}
	}

	logger := log.New(os.Stderr, "", 0)

	if verbose {
		logger.Printf("Creating DBT directory in %s.dbt", homedir)
	}

	dbtPath := fmt.Sprintf("%s/%s", homedir, DbtDir)

	if _, err := os.Stat(dbtPath); os.IsNotExist(err) {
		err = os.Mkdir(dbtPath, 0755)
		if err != nil {
			err = errors.Wrapf(err, "failed to create directory %s", dbtPath)
			return err
		}
	}

	trustPath := fmt.Sprintf("%s/%s", homedir, TrustDir)

	if _, err := os.Stat(trustPath); os.IsNotExist(err) {
		err = os.Mkdir(trustPath, 0755)
		if err != nil {
			err = errors.Wrapf(err, "failed to create directory %s", trustPath)
			return err
		}
	}

	toolPath := fmt.Sprintf("%s/%s", homedir, ToolDir)
	err = os.Mkdir(toolPath, 0755)
	if err != nil {
		err = errors.Wrapf(err, "failed to create directory %s", toolPath)
		return err
	}

	configPath := fmt.Sprintf("%s/%s", homedir, ConfigDir)
	err = os.Mkdir(configPath, 0755)
	if err != nil {
		err = errors.Wrapf(err, "failed to create directory %s", configPath)
		return err
	}

	return err
}

// GetHomeDir get's the current user's homedir
func GetHomeDir() (dir string, err error) {
	dir, err = homedir.Dir()
	return dir, err
}

// FetchTrustStore writes the downloaded trusted signing public keys to disk.
func (dbt *DBT) FetchTrustStore(homedir string, verbose bool) (err error) {
	uri := dbt.Config.Dbt.TrustStore

	if verbose {
		fmt.Printf("Fetching truststore from %q\n", uri)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request for url: %s", uri)
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

	resp, err := client.Do(req)
	if err != nil {
		err = errors.Wrapf(err, "failed to fetch truststore from %s", uri)
		return err
	}
	if resp != nil {
		defer resp.Body.Close()

		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			err = errors.Wrapf(err, "failed to read truststore contents")
			return err
		}

		keytext := string(bodyBytes)

		// don't write anything if we have an empty string
		if keytext != "" {
			filePath := fmt.Sprintf("%s/%s", homedir, TruststorePath)
			err = ioutil.WriteFile(filePath, []byte(keytext), 0644)
			if err != nil {
				err = errors.Wrapf(err, "failed to write trust file")
				return err
			}
		}
	}

	return err
}

// IsCurrent returns whether the currently running version is the latest version, and possibly an error if the version check fails
func (dbt *DBT) IsCurrent(binaryPath string) (ok bool, err error) {
	latest, err := dbt.FindLatestVersion("")
	if err != nil {
		err = errors.Wrap(err, "failed to fetch dbt versions")
		return ok, err
	}

	latestDbtVersionUrl := fmt.Sprintf("%s/%s/%s/%s/dbt", dbt.Config.Dbt.Repo, latest, runtime.GOOS, runtime.GOARCH)

	ok, err = dbt.VerifyFileVersion(latestDbtVersionUrl, binaryPath)
	if err != nil {
		err = errors.Wrap(err, "failed to check latest version")
		return ok, err
	}

	if !ok {
		_, _ = fmt.Fprint(os.Stderr, fmt.Sprintf("Newer version of dbt available: %s\n\n", latest))
	}

	return ok, err
}

// UpgradeInPlace upgraded dbt in place
func (dbt *DBT) UpgradeInPlace(binaryPath string) (err error) {
	tmpDir, err := ioutil.TempDir("", "dbt")
	if err != nil {
		err = errors.Wrap(err, "failed to create temp dir")
		return err
	}

	defer os.RemoveAll(tmpDir)

	newBinaryFile := fmt.Sprintf("%s/dbt", tmpDir)

	latest, err := dbt.FindLatestVersion("")
	if err != nil {
		err = errors.Wrap(err, "failed to find latest dbt version")
		return err
	}

	latestDbtVersionUrl := fmt.Sprintf("%s/%s/%s/%s/dbt", dbt.Config.Dbt.Repo, latest, runtime.GOOS, runtime.GOARCH)

	err = dbt.FetchFile(latestDbtVersionUrl, newBinaryFile)
	if err != nil {
		err = errors.Wrap(err, "failed to fetch new dbt binary")
		return err
	}

	ok, err := dbt.VerifyFileVersion(latestDbtVersionUrl, newBinaryFile)
	if err != nil {
		err = errors.Wrap(err, "failed to verify downloaded binary")
		return err
	}

	if ok {
		// This is slightly more painful than it might otherwise be in order to handle modern linux systems where /tmp is tmpfs (can't just rename cross partition).
		// So instead we read the file, write the file to a temp file, and then rename.
		newBinaryTempFile := fmt.Sprintf("%s.new", binaryPath)

		b, err := ioutil.ReadFile(newBinaryFile)
		if err != nil {
			err = errors.Wrapf(err, "failed to read new binary file %s", newBinaryFile)
			return err
		}

		err = ioutil.WriteFile(newBinaryTempFile, b, 0755)
		if err != nil {
			err = errors.Wrapf(err, "failed to write new binary temp file %s", newBinaryTempFile)
			return err
		}

		err = os.Rename(newBinaryTempFile, binaryPath)
		if err != nil {
			err = errors.Wrap(err, "failed to move new binary into place")
			return err
		}

		err = os.Chmod(binaryPath, 0755)
		if err != nil {
			err = errors.Wrap(err, "failed to chmod new dbt binary")
			return err
		}
	}

	return err
}

// RunTool runs the dbt tool indicated by the args
func (dbt *DBT) RunTool(version string, args []string, homedir string, offline bool) (err error) {
	toolName := args[0]

	if toolName == "--" {
		toolName = args[1]
		args = args[1:]
	}

	localPath := fmt.Sprintf("%s/%s/%s", homedir, ToolDir, toolName)

	// if offline, if tool is present and verifies, run it
	if offline {
		err = dbt.verifyAndRun(homedir, args)
		if err != nil {
			err = errors.Wrap(err, "offline run failed")
			return err
		}

		return err
	}

	// we're not offline, so find the latest
	latestVersion, err := dbt.FindLatestVersion(toolName)
	if err != nil {
		err = errors.Wrap(err, "failed to find latest version")
		return err
	}

	// if it's not in the repo, it might still be on the filesystem
	if latestVersion == "" {
		// if it is indeed on the filesystem
		if _, err := os.Stat(localPath); !os.IsNotExist(err) {
			// attempt to run it in offline mode
			err = dbt.verifyAndRun(homedir, args)
			if err != nil {
				err = errors.Wrap(err, "offline run failed")
				return err
			}

			// and return if it's successful
			return err
		}

		// It's not in the repo, and not on the filesystem, there's not a damn thing we can do.  Fail.
		err = fmt.Errorf("Tool %s is not in repo, and has not been previously downloaded.  Cannot run.\n", toolName)
		return err
	}

	// if version is unset, version is latest version
	if version == "" {
		version = latestVersion
	}

	// url should be http(s)://tool-repo/toolName/version/os/arch/tool
	toolUrl := fmt.Sprintf("%s/%s/%s/%s/%s/%s", dbt.Config.Tools.Repo, toolName, version, runtime.GOOS, runtime.GOARCH, toolName)

	if _, err := os.Stat(localPath); !os.IsNotExist(err) {

		// check to see if the latest version is what we have
		uptodate, err := dbt.VerifyFileVersion(toolUrl, localPath)
		if err != nil {
			err = errors.Wrap(err, "failed to verify file version")
			return err
		}

		// if yes, run it
		if uptodate {
			err = dbt.verifyAndRun(homedir, args)
			if err != nil {
				err = errors.Wrap(err, "run failed")
				return err
			}

			return err
		}
	}

	// download the binary
	dbt.Logger.Printf("Downloading binary tool %q version %s.", toolName, version)
	err = dbt.FetchFile(toolUrl, localPath)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("failed to fetch binary for %s from %s", toolName, toolUrl))
		return err
	}

	// download the checksum
	toolChecksumUrl := fmt.Sprintf("%s.sha256", toolUrl)
	toolChecksumFile := fmt.Sprintf("%s.sha256", localPath)

	err = dbt.FetchFile(toolChecksumUrl, toolChecksumFile)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("failed to fetch checksum for %s from %s", toolName, toolChecksumUrl))
		return err
	}

	// download the signature
	toolSignatureUrl := fmt.Sprintf("%s.asc", toolUrl)
	toolSignatureFile := fmt.Sprintf("%s.asc", localPath)

	err = dbt.FetchFile(toolSignatureUrl, toolSignatureFile)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("failed to fetch signature for %s from %s", toolName, toolSignatureUrl))
		return err
	}

	// finally run it
	err = dbt.verifyAndRun(homedir, args)

	return err
}

func (dbt *DBT) verifyAndRun(homedir string, args []string) (err error) {
	toolName := args[0]
	if toolName == "--" {
		toolName = args[1]
		args = args[1:]
	}

	localPath := fmt.Sprintf("%s/%s/%s", homedir, ToolDir, toolName)
	localChecksumPath := fmt.Sprintf("%s/%s/%s.sha256", homedir, ToolDir, toolName)

	checksumBytes, err := ioutil.ReadFile(localChecksumPath)
	if err != nil {
		err = errors.Wrap(err, "error reading local checksum file")
		return err
	}

	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		checksumOk, err := dbt.VerifyFileChecksum(localPath, string(checksumBytes))
		if err != nil {
			err = errors.Wrap(err, "error validating checksum")
			return err
		}

		if !checksumOk {
			err = fmt.Errorf("checksum of %s failed to verify", toolName)
			return err
		}

		signatureOk, err := dbt.VerifyFileSignature(homedir, localPath)
		if err != nil {
			err = errors.Wrap(err, "error validating signature")
			return err
		}

		if !signatureOk {
			err = fmt.Errorf("signature of %s failed to verify", toolName)
			return err
		}

		err = dbt.runExec(homedir, args)
		if err != nil {
			err = errors.Wrap(err, "failed to run already downloaded tool")
			return err
		}
	}

	return err
}

var testExec bool

func (dbt *DBT) runExec(homedir string, args []string) (err error) {
	toolName := args[0]
	localPath := fmt.Sprintf("%s/%s/%s", homedir, ToolDir, toolName)

	env := os.Environ()

	if testExec {
		env = append(env, "GO_WANT_HELPER_PROCESS=1")
		cs := []string{"-test.run=TestHelperProcess", "--", localPath}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		bytes, err := cmd.Output()
		if err != nil {
			err = errors.Wrap(err, "error running exec")
			return err
		}

		fmt.Printf("\nTest Command Output: %q\n", string(bytes))

	} else {
		err = syscall.Exec(localPath, args, env)
		if err != nil {
			err = errors.Wrap(err, "error running exec")
			return err
		}

	}

	return err
}
