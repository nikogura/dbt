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
)

const dbtDir = ".dbt"
const trustDir = dbtDir + "/trust"
const toolDir = dbtDir + "/tools"
const configDir = dbtDir + "/conf"
const configFilePath = configDir + "/dbt.json"
const truststorePath = trustDir + "/truststore"

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
	config, err := LoadDbtConfig(false)
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
func LoadDbtConfig(verbose bool) (config Config, err error) {
	userObj, err := user.Current()
	if err != nil {
		err = errors.Wrapf(err, "failed to get current user")
		return config, err
	}

	homeDir := userObj.HomeDir

	if homeDir == "" {
		err = fmt.Errorf("no homedir for user %q", userObj.Username)
		return config, err
	}

	return loadDbtConfig(homeDir, verbose)
}

func loadDbtConfig(parentDir string, verbose bool) (config Config, err error) {

	filePath := fmt.Sprintf("%s/%s", parentDir, configFilePath)

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
func GenerateDbtDir(verbose bool) (err error) {
	userObj, err := user.Current()
	if err != nil {
		err = errors.Wrapf(err, "failed to get current user")
		return err
	}

	homeDir := userObj.HomeDir

	if homeDir == "" {
		err = fmt.Errorf("no homedir for user %q", userObj.Username)
		return err
	}

	return generateDbtDir(homeDir, verbose)

}

// generateDbtDir generates the dbt dir tree under the given parent dir.  This function is split out from GenerateDbtDir merely to be able to test the dir tree creation code.
func generateDbtDir(parentDir string, verbose bool) (err error) {
	if verbose {
		log.Printf("Creating DBT directory in %s.dbt", parentDir)
	}

	dbtPath := fmt.Sprintf("%s/%s", parentDir, dbtDir)

	if _, err := os.Stat(dbtPath); os.IsNotExist(err) {
		err = os.Mkdir(dbtPath, 0755)
		if err != nil {
			err = errors.Wrapf(err, "failed to create directory %s", dbtPath)
			return err
		}
	}

	trustPath := fmt.Sprintf("%s/%s", parentDir, trustDir)

	if _, err := os.Stat(trustPath); os.IsNotExist(err) {
		err = os.Mkdir(trustPath, 0755)
		if err != nil {
			err = errors.Wrapf(err, "failed to create directory %s", trustPath)
			return err
		}
	}

	toolPath := fmt.Sprintf("%s/%s", parentDir, toolDir)
	err = os.Mkdir(toolPath, 0755)
	if err != nil {
		err = errors.Wrapf(err, "failed to create directory %s", toolPath)
		return err
	}

	configPath := fmt.Sprintf("%s/%s", parentDir, configDir)
	err = os.Mkdir(configPath, 0755)
	if err != nil {
		err = errors.Wrapf(err, "failed to create directory %s", configPath)
		return err
	}

	return err
}

// FetchTrustStore writes the downloaded trusted signing public keys to disk.
func (dbt *DBT) FetchTrustStore() (err error) {
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
			err = ioutil.WriteFile(truststorePath, []byte(keytext), 0644)
			if err != nil {
				err = errors.Wrapf(err, "failed to write trust file")
				return err
			}
		}
	}

	return err
}

// IsCurrent returns whether the currently running version is the latest version, and possibly an error if the version check failes
func (dbt *DBT) IsCurrent() (ok bool, err error) {
	log.Printf("Attempting to find latest version")
	return ok, err
}

// UpgradeInPlace upgraded dbt in place
func (dbt *DBT) UpgradeInPlace() (err error) {
	log.Printf("Attempting upgrade in place")
	return err
}

// RunTool runs the dbt tool indicated by the args
func (dbt *DBT) RunTool(version string, args []string) (err error) {

	log.Printf("Running: %s", args)

	return err
}
