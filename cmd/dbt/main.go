package main

import (
	"fmt"
	"github.com/nikogura/dbt/pkg/dbt"
	"log"
	"os"
	"syscall"
)

// DBT the file path of the installed dbt binary
const DBT = "/usr/local/bin/dbt"

// VERSION the version of dbt.  Must match version in metadata.json
const VERSION = "2.1.9"

// there are only two options for dbt itself, 'version' and 'offline'
var version string
var offline bool

func main() {
	args := os.Args[1:]

	// exit early if there are no args, or if the first arg is 'help'
	exitEarlyIf(args)

	dbtObj, err := dbt.NewDbt()
	if err != nil {
		log.Fatalf("Error creating DBT object: %s", err)
	}

	possibles := []string{"-o", "-ov"}

	if dbt.StringInSlice(args[0], possibles) { // set quiet
		offline = true
	}

	homedir, err := dbt.GetHomeDir()
	if err != nil {
		log.Fatalf("Failed to discover user homedir: %s\n", err)
	}

	// if we're not explicitly offline, try to upgrade in place
	if !offline {
		// first fetch the current truststore
		err = dbtObj.FetchTrustStore(homedir, false)
		if err != nil {
			log.Fatalf("Failed to fetch current truststore: %s", err)
		}

		ok, err := dbtObj.IsCurrent("")
		if err != nil {
			log.Printf("Failed to confirm whether we're up to date: %s", err)
		}

		if !ok {
			log.Printf("Downloading and verifying new version of dbt.")
			err = dbtObj.UpgradeInPlace("")
			if err != nil {
				err = fmt.Errorf("upgrade in place failed: %s", err)
				log.Fatalf("Error: %s", err)
			}

			// Single white female ourself
			syscall.Exec(DBT, os.Args, os.Environ())
		}
	}

	if len(args) > 0 {
		possibles = []string{"-v", "-ov"}

		if dbt.StringInSlice(args[0], possibles) {
			if len(args) > 2 {
				version = args[1]

				err = dbtObj.RunTool(version, args[2:], homedir, offline)
				if err != nil {
					log.Fatalf("Error running tool: %s", err)
				}

			} else {
				log.Fatalf("-v flag requires a version.")
			}
		} else {
			err = dbtObj.RunTool(version, args, homedir, offline)
			if err != nil {
				log.Fatalf("Error running tool: %s", err)
			}
		}

	} else {
		helpMessage()
		log.Fatal(1)
	}
}

func exitEarlyIf(args []string) {
	if len(args) == 0 { // no args, print help and exit
		helpMessage()
		os.Exit(0)
	}

	if args[0] == "help" { // if we asked for help, give the help
		helpMessage()
		os.Exit(0)
	}

}

func helpMessage() {

	log.Printf(`DBT Dynamic Binary Toolkit version: %s

Usage:

dbt [-o -v <version>] <tool> [tool args]
	-v version 			Specify version of tool to run.  (Defaults to latest)
	-o offline 			Offline mode.  Does not attempt to upgrade or find tools, just uses what's already on disk, and errors if it's not available.

Run 'dbt catalog list' to see a list of what tools are available in your repository.

`, VERSION)
}
