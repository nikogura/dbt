package dbt

import (
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
)

const dbtDir = ".dbt"
const trustDir = ".dbt/trust"
const trustFile = "trusted.keys"

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

// WriteTrustedKeys writes the downloaded trusted signing public keys to disk.  If offline, it does nothing.
func WriteTrustedKeys(keytext string) (err error) {
	if keytext != "" {
		fileName := fmt.Sprintf("%s/%s", trustDir, trustFile)
		err = ioutil.WriteFile(fileName, []byte(keytext), 0644)
		if err != nil {
			err = errors.Wrapf(err, "failed to write trust file")
			return err
		}
	}

	return err
}
