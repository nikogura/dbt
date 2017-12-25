package dbt

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"runtime"
	"syscall"
)

const dbtDir = ".dbt"
const trustDir = dbtDir + "/trust"
const toolDir = dbtDir + "/tools"
const configDir = dbtDir + "/conf"
const configFilePath = configDir + "/dbt.json"
const truststorePath = trustDir + "/truststore"
const dbtBinaryPath = "/usr/local/bin/dbt"

// DBT the dbt object itself
type DBT struct {
	Config  Config
	Verbose bool
}

// Config  configuration of the dbt object
type Config struct {
	Dbt   DbtConfig   `json:"dbt"`
	Tools ToolsConfig `json:"tools"`
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

	if verbose {
		log.Printf("Creating DBT directory in %s.dbt", homedir)
	}

	filePath := fmt.Sprintf("%s/%s", homedir, configFilePath)

	if verbose {
		log.Printf("Loading config from %s", filePath)
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

	if verbose {
		log.Printf("Creating DBT directory in %s.dbt", homedir)
	}

	dbtPath := fmt.Sprintf("%s/%s", homedir, dbtDir)

	if _, err := os.Stat(dbtPath); os.IsNotExist(err) {
		err = os.Mkdir(dbtPath, 0755)
		if err != nil {
			err = errors.Wrapf(err, "failed to create directory %s", dbtPath)
			return err
		}
	}

	trustPath := fmt.Sprintf("%s/%s", homedir, trustDir)

	if _, err := os.Stat(trustPath); os.IsNotExist(err) {
		err = os.Mkdir(trustPath, 0755)
		if err != nil {
			err = errors.Wrapf(err, "failed to create directory %s", trustPath)
			return err
		}
	}

	toolPath := fmt.Sprintf("%s/%s", homedir, toolDir)
	err = os.Mkdir(toolPath, 0755)
	if err != nil {
		err = errors.Wrapf(err, "failed to create directory %s", toolPath)
		return err
	}

	configPath := fmt.Sprintf("%s/%s", homedir, configDir)
	err = os.Mkdir(configPath, 0755)
	if err != nil {
		err = errors.Wrapf(err, "failed to create directory %s", configPath)
		return err
	}

	return err
}

// GetHomeDir get's the current user's homedir
func GetHomeDir() (homedir string, err error) {
	userObj, err := user.Current()
	if err != nil {
		err = errors.Wrapf(err, "failed to get current user")
		return homedir, err
	}

	homedir = userObj.HomeDir

	if homedir == "" {
		err = fmt.Errorf("no homedir for user %q", userObj.Username)
		return homedir, err
	}

	return homedir, err
}

// FetchTrustStore writes the downloaded trusted signing public keys to disk.
func (dbt *DBT) FetchTrustStore(homedir string, verbose bool) (err error) {

	uri := dbt.Config.Dbt.TrustStore

	resp, err := http.Get(uri)
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
			filePath := fmt.Sprintf("%s/%s", homedir, truststorePath)
			err = ioutil.WriteFile(filePath, []byte(keytext), 0644)
			if err != nil {
				err = errors.Wrapf(err, "failed to write trust file")
				return err
			}
		}
	}

	return err
}

// IsCurrent returns whether the currently running version is the latest version, and possibly an error if the version check failes
func (dbt *DBT) IsCurrent(binaryPath string) (ok bool, err error) {
	if binaryPath == "" {
		binaryPath = dbtBinaryPath
	}

	fmt.Fprint(os.Stderr, "Verifying that dbt is up to date....\n\n")
	fmt.Fprint(os.Stderr, "Checking available versions...\n\n")

	latest, err := dbt.FindLatestVersion(dbt.Config.Dbt.Repo, "")
	if err != nil {
		err = errors.Wrap(err, "failed to fetch dbt versions")
		return ok, err
	}

	fmt.Fprint(os.Stderr, fmt.Sprintf("Latest Version is: %s\n\n", latest))

	fmt.Fprint(os.Stderr, "Checking to see that I'm that version...\n\n")

	latestDbtVersionUrl := fmt.Sprintf("%s/%s/%s/%s/dbt", dbt.Config.Dbt.Repo, latest, runtime.GOOS, runtime.GOARCH)

	ok, err = VerifyFileVersion(latestDbtVersionUrl, binaryPath)
	if err != nil {
		err = errors.Wrap(err, "failed to check latest version")
		return ok, err
	}

	if ok {
		fmt.Fprintf(os.Stderr, "dbt is up to date.  (version %s)\n\n", latest)
		return ok, err
	}

	fmt.Fprint(os.Stderr, "nope.  Let's fix that.\n\nDownloading the latest.\n\n")

	return ok, err
}

// UpgradeInPlace upgraded dbt in place
func (dbt *DBT) UpgradeInPlace(binaryPath string) (err error) {
	if binaryPath == "" {
		binaryPath = dbtBinaryPath
	}
	fmt.Fprint(os.Stderr, "Attempting to upgrade in place.\n\n")

	tmpDir, err := ioutil.TempDir("", "dbt")
	if err != nil {
		err = errors.Wrap(err, "failed to create temp dir")
		return err
	}

	defer os.RemoveAll(tmpDir)

	newBinaryFile := fmt.Sprintf("%s/dbt", tmpDir)

	latest, err := dbt.FindLatestVersion(dbt.Config.Dbt.Repo, "")
	if err != nil {
		err = errors.Wrap(err, "failed to find latest dbt version")
		return err
	}

	fmt.Fprintf(os.Stderr, "Latest Version is : %s\n\n", latest)

	fmt.Fprint(os.Stderr, "Checking to see that I'm that version...\n\n")

	latestDbtVersionUrl := fmt.Sprintf("%s/%s/%s/%s/dbt", dbt.Config.Dbt.Repo, latest, runtime.GOOS, runtime.GOARCH)

	err = FetchFile(latestDbtVersionUrl, newBinaryFile)
	if err != nil {
		err = errors.Wrap(err, "failed to fetch new dbt binary")
		return err
	}

	fmt.Fprint(os.Stderr, "Binary downloaded.  Verifying it.\n\n")

	ok, err := VerifyFileVersion(latestDbtVersionUrl, newBinaryFile)
	if err != nil {
		err = errors.Wrap(err, "failed to verify downloaded binary")
	}

	if ok {
		fmt.Fprint(os.Stderr, "new version verifies.  Swapping it into place.\n\n")
		err = os.Rename(newBinaryFile, binaryPath)
		if err != nil {
			err = errors.Wrap(err, "failed to move new binary into place")
		}

		err = os.Chmod(binaryPath, 0755)
		if err != nil {
			err = errors.Wrap(err, "failed to chmod new dbt binary")
			return err
		}
	}

	return err
}

// FindLatestVersion finds the latest version of the tool given in the repo given.  If the tool name is "", it is expecting to parse versions in the root of the repo.  I.e. there's only one tool in the repo.
func (dbt *DBT) FindLatestVersion(repoUrl string, toolName string) (latest string, err error) {
	toolInRepo, err := ToolExists(repoUrl, toolName)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("error checking repo %s for tool %s", dbt.Config.Tools.Repo, toolName))
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

// RunTool runs the dbt tool indicated by the args
func (dbt *DBT) RunTool(version string, args []string, homedir string, offline bool) (err error) {
	toolName := args[0]
	localPath := fmt.Sprintf("%s/%s/%s", homedir, toolDir, toolName)

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
	latestVersion, err := dbt.FindLatestVersion(dbt.Config.Tools.Repo, toolName)
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

	// check to see if the latest version is what we have
	uptodate, err := VerifyFileVersion(toolUrl, localPath)
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

	// if no, download it and then run it
	err = FetchFile(toolUrl, localPath)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("failed to fetch binary for %s from %s", toolName, toolUrl))
		return err
	}

	// finally run it
	err = dbt.verifyAndRun(homedir, args)

	return err
}

func (dbt *DBT) verifyAndRun(homedir string, args []string) (err error) {
	toolName := args[0]
	localPath := fmt.Sprintf("%s/%s", toolDir, toolName)
	localChecksumPath := fmt.Sprintf("%s/%s.sha256", toolDir, toolName)

	checksumBytes, err := ioutil.ReadFile(localChecksumPath)
	if err != nil {
		err = errors.Wrap(err, "error reading local checksum file")
		return err
	}

	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		checksumOk, err := VerifyFileChecksum(localPath, string(checksumBytes))
		if err != nil {
			err = errors.Wrap(err, "error validating checksum")
			return err
		}

		if !checksumOk {
			err = fmt.Errorf("checksum of %s failed to verify", toolName)
			return err
		}

		signatureOk, err := VerifyFileSignature(homedir, localPath)
		if err != nil {
			err = errors.Wrap(err, "error validating signature")
			return err
		}

		if !signatureOk {
			err = fmt.Errorf("signature of %s failed to verify", toolName)
			return err
		}

		err = dbt.runExec(args)
		if err != nil {
			err = errors.Wrap(err, "failed to run already downloaded tool")
			return err
		}
	}

	return err
}

func (dbt *DBT) runExec(args []string) (err error) {
	toolName := args[0]
	localPath := fmt.Sprintf("%s/%s", toolDir, toolName)

	env := os.Environ()

	err = syscall.Exec(localPath, args, env)
	if err != nil {
		err = errors.Wrap(err, "error running exec")
		return err
	}

	return err
}
