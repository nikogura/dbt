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

package cmd

import (
	"fmt"
	"github.com/nikogura/dbt/pkg/dbt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
	"syscall"
)

var toolVersion string
var offline bool
var verbose bool

var rootCmd = &cobra.Command{
	Use:   "dbt",
	Short: "Dynamic Binary Toolkit",
	Long: `
Dynamic Binary Toolkit

A framework for running self-updating signed binaries from a trusted repository.

Run 'dbt -- catalog list' to see a list of what tools are available in your repository.

`,
	Example: "dbt -- catalog list",
	Version: "3.5.0",
	Run:     Run,
}

func init() {
	rootCmd.Flags().StringVarP(&toolVersion, "toolversion", "v", "", "Version of tool to run.")
	rootCmd.Flags().BoolVarP(&offline, "offline", "o", false, "Offline mode.")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "V", false, "Verbose output")
}

// Execute - execute the command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Run run dbt itself.
func Run(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		err := cmd.Help()
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	dbtObj, err := dbt.NewDbt("")
	if err != nil {
		log.Fatalf("Error creating DBT object: %s", err)
	}

	dbtObj.SetVerbose(verbose)

	homedir, err := dbt.GetHomeDir()
	if err != nil {
		log.Fatalf("Failed to discover user homedir: %s\n", err)
	}

	dbtBinary, err := exec.LookPath("dbt")
	if err != nil {
		log.Fatalf("Couldn't find `dbt` in $PATH: %s", err)
	}

	// if we're not explicitly offline, try to upgrade in place
	if !offline {
		// first fetch the current truststore
		err = dbtObj.FetchTrustStore(homedir)
		if err != nil {
			log.Fatalf("Failed to fetch remote truststore: %s.\n\nIf you want to try in 'offline' mode, retry your command again with: dbt -o ...", err)
		}

		ok, err := dbtObj.IsCurrent(dbtBinary)
		if err != nil {
			log.Printf("Failed to confirm whether we're up to date: %s", err)
		}

		if !ok {
			log.Printf("Downloading and verifying new version of dbt.")
			err = dbtObj.UpgradeInPlace(dbtBinary)
			if err != nil {
				err = fmt.Errorf("upgrade in place failed: %s", err)
				log.Fatalf("Error: %s", err)
			}

			// Single white female ourself
			_ = syscall.Exec(dbtBinary, os.Args, os.Environ())
		}
	}

	err = dbtObj.RunTool(toolVersion, args, homedir, offline)
	if err != nil {
		log.Fatal(err)
	}
}
