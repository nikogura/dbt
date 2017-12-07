package dbt

import "log"

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
