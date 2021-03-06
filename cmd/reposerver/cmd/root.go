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
)

var address string
var port int
var serverRoot string
var configFile string

var rootCmd = &cobra.Command{
	Use:   "reposerver",
	Short: "dbt repository server",
	Long: `
DBT Repository Server

Pure golang server implementation of the dbt trusted repostitory.

`,
	Example: "dbt -- server",
	Version: "0.1.0",
	Run:     Run,
}

func init() {
	rootCmd.Flags().StringVarP(&address, "address", "a", "127.0.0.1", "Address on which to run server.")
	rootCmd.Flags().IntVarP(&port, "port", "p", 9999, "Port on which to run server.")
	rootCmd.Flags().StringVarP(&serverRoot, "root", "r", "", "Server Root (Local path from which to serve components.")
	rootCmd.Flags().StringVarP(&configFile, "file", "f", "", "Config file for reposerver.")
}

// Execute  execute the command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Run run the reposerver
func Run(cmd *cobra.Command, args []string) {
	var repo *dbt.DBTRepoServer

	if configFile != "" {
		r, err := dbt.NewRepoServer(configFile)
		if err != nil {
			log.Fatalf("Failed to create reposerver from file: %s", err)
		}

		repo = r

	} else {
		repo = &dbt.DBTRepoServer{
			Address:    address,
			Port:       port,
			ServerRoot: serverRoot,
		}
	}

	if repo == nil {
		log.Fatalf("Failed to initialize reposerver object.  Cannot continue.")
	}

	err := repo.RunRepoServer()
	if err != nil {
		log.Fatalf("Error running server: %s", err)
	}
}
