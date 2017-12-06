package main

import (
	"log"
)

func main() {

	helpMessage()

}

func helpMessage() {

	log.Printf(`DBT Distributed Binary Toolkit

Usage:

dbt [-v] <tool> [options]
	-v version 			Specify version of tool to run.  (Defaults to latest)
	-o offline 			Offline mode.  Does not attempt to upgrade or find tools, just uses what's already on disk, and errors if it's not available.

`)
}
