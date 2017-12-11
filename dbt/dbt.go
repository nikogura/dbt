package dbt

import (
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"os"
	"os/user"
)

const dbtDir = ".dbt"
const trustDir = dbtDir + "/trust"
const trustFileName = "trusted.keys"
const trustFilePath = trustDir + "/" + trustFileName
const toolDir = dbtDir + "/tools"

// IsCurrent returns whether the currently running version is the latest version, and possibly an error if the version check failes
func IsCurrent() (ok bool, err error) {
	log.Printf("Attempting to find latest version")
	return ok, err
}

// UpgradeInPlace upgraded dbt in place
func UpgradeInPlace() (err error) {
	log.Printf("Attempting upgrade in place")
	return err
}

// RunTool runs the dbt tool indicated by the args
func RunTool(version string, args []string) (err error) {

	log.Printf("Running: %s", args)

	return err
}

// GenerateDbtDir generates the necessary dbt dirs in the user's homedir if they don't already exist.  If htey do exist, it does nothing.
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

	dbtPath := fmt.Sprintf("%s%s", parentDir, dbtDir)

	if _, err := os.Stat(dbtPath); os.IsNotExist(err) {
		err = os.Mkdir(dbtPath, 0755)
		if err != nil {
			err = errors.Wrapf(err, "failed to create directory %s", dbtPath)
			return err
		}
	}

	trustPath := fmt.Sprintf("%s%s", parentDir, trustDir)

	if _, err := os.Stat(trustPath); os.IsNotExist(err) {
		err = os.Mkdir(trustPath, 0755)
		if err != nil {
			err = errors.Wrapf(err, "failed to create directory %s", trustPath)
			return err
		}
	}

	toolPath := fmt.Sprintf("%s%s", parentDir, toolDir)
	err = os.Mkdir(toolPath, 0755)
	if err != nil {
		err = errors.Wrapf(err, "failed to create directory %s", toolPath)
		return err
	}

	return err
}

// WriteTrustedKeys writes the downloaded trusted signing public keys to disk.  If offline, it does nothing.
func WriteTrustedKeys(keytext string) (err error) {
	// TODO WriteTrustedKeys needs a path prefix, the homedir
	if keytext != "" {
		fileName := trustFilePath
		err = ioutil.WriteFile(fileName, []byte(keytext), 0644)
		if err != nil {
			err = errors.Wrapf(err, "failed to write trust file")
			return err
		}
	}

	return err
}
