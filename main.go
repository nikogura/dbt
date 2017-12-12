package main

import (
	"fmt"
	"github.com/nikogura/dbt/dbt"
	"log"
	"os"
	"regexp"
	"syscall"
)

// DBT the file path of the installed dbt binary
const DBT = "/usr/local/bin/dbt"

// there are only two options for dbt itself, 'version' and 'offline'
var version string
var offline bool

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		helpMessage()
		os.Exit(1)
	}

	// start normal processing

	re := regexp.MustCompile("-")

	if !re.MatchString(args[0]) { // if the first arg is a word
		if args[0] == "help" { // if it's help, give the help
			helpMessage()
			os.Exit(0)
		}

	} else { // args[1] is either -v, -o, -ov

		err := dbt.GenerateDbtDir(false)
		if err != nil {
			log.Printf("Failed to generate necessary config directories: %s", err)
			os.Exit(1)
		}

		dbtObj, err := dbt.NewDbt()
		if err != nil {
			log.Printf("Error creating DBT object: %s", err)
			os.Exit(1)
		}

		possibles := []string{"-o", "-ov"}

		if dbt.StringInSlice(args[0], possibles) { // set quiet
			offline = true
		}

		// if we're not explicitly offline, try to upgrade in place
		if !offline {
			// first fetch the current truststore
			err = dbtObj.FetchTrustStore()
			if err != nil {
				log.Printf("Failed to fetch current truststore")
				os.Exit(1)
			}

			ok, err := dbtObj.IsCurrent()
			if err != nil {
				log.Printf("Failed to confirm whether we're up to date.")
			}

			if !ok {
				err = dbtObj.UpgradeInPlace()
				if err != nil {
					err = fmt.Errorf("upgrade in place failed: %s", err)
					log.Printf("Error: %s", err)
					os.Exit(1)
				}

				// Single white female ourself
				syscall.Exec(DBT, os.Args, os.Environ())
			}
		}

		if len(args) > 1 {
			possibles = []string{"-v", "-ov"}

			if dbt.StringInSlice(args[0], possibles) {
				if len(args) > 2 {
					version = args[1]

					dbtObj.RunTool(version, args[2:])

				} else {
					fmt.Println("-v flag requires a version.")
					os.Exit(1)
				}
			} else {
				if args[1] == "-v" {
					fmt.Println("-v flag requires a version.")
					os.Exit(1)

				} else {
					// deliberately leaving all error processing to the tool
					dbtObj.RunTool(version, args[1:])
				}
			}

		} else {
			helpMessage()
			os.Exit(1)
		}
	}
}

func helpMessage() {

	log.Printf(`DBT Distributed Binary Toolkit

Usage:

dbt [-o -v <version>] <tool> [tool args]
	-v version 			Specify version of tool to run.  (Defaults to latest)
	-o offline 			Offline mode.  Does not attempt to upgrade or find tools, just uses what's already on disk, and errors if it's not available.

`)
}
