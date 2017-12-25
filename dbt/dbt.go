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
func (dbt *DBT) IsCurrent() (ok bool, err error) {
	log.Printf("Attempting to find latest version")
	// TODO Implement IsCurrent()
	return ok, err
}

// UpgradeInPlace upgraded dbt in place
func (dbt *DBT) UpgradeInPlace() (err error) {
	log.Printf("Attempting upgrade in place")
	// TODO Implement UpgradeInPlace()
	return err
}

// RunTool runs the dbt tool indicated by the args
func (dbt *DBT) RunTool(version string, args []string) (err error) {

	log.Printf("Running: %s", args)
	// TODO Implement RunTool()

	return err
}
