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

//nolint:gochecknoglobals // Cobra requires global variables for flags
var address string

//nolint:gochecknoglobals // Cobra requires global variables for flags
var port int

//nolint:gochecknoglobals // Cobra requires global variables for flags
var serverRoot string

//nolint:gochecknoglobals // Cobra requires global variables for flags
var configFile string

//nolint:gochecknoglobals // Cobra boilerplate
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

//nolint:gochecknoinits // Cobra boilerplate
func init() {
	rootCmd.Flags().StringVarP(&address, "address", "a", "127.0.0.1", "Address on which to run server.")
	rootCmd.Flags().IntVarP(&port, "port", "p", 9999, "Port on which to run server.")
	rootCmd.Flags().StringVarP(&serverRoot, "root", "r", "", "Server Root (Local path from which to serve components.")
	rootCmd.Flags().StringVarP(&configFile, "file", "f", "", "Config file for reposerver.")
}

// Execute executes the root command.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Run runs the reposerver.
func Run(cmd *cobra.Command, args []string) {
	repo := buildRepoServer(cmd)

	if repo == nil {
		log.Fatalf("Failed to initialize reposerver object.  Cannot continue.")
	}

	err := repo.RunRepoServer()
	if err != nil {
		log.Fatalf("Error running server: %s", err)
	}
}

// buildRepoServer creates a DBTRepoServer from config file, env vars, or CLI flags.
// Priority: config file (-f) wins outright; otherwise env vars provide the base
// and CLI flags override address/port/root on top.
func buildRepoServer(cmd *cobra.Command) (repo *dbt.DBTRepoServer) {
	if configFile != "" {
		r, err := dbt.NewRepoServer(configFile)
		if err != nil {
			log.Fatalf("Failed to create reposerver from file: %s", err)
		}

		repo = r

		return repo
	}

	// Build from env vars, then apply any CLI flag overrides.
	// This allows Docker CMD defaults (e.g. -a 0.0.0.0 -p 9999 -r /var/dbt)
	// to coexist with REPOSERVER_* env vars for auth configuration.
	r, err := dbt.NewRepoServerFromEnv()
	if err != nil {
		log.Fatalf("Failed to create reposerver from environment: %s", err)
	}

	repo = r

	applyCLIOverrides(cmd, repo)

	return repo
}

// applyCLIOverrides applies CLI flag values to the server config when explicitly set.
func applyCLIOverrides(cmd *cobra.Command, repo *dbt.DBTRepoServer) {
	if cmd.Flags().Changed("address") {
		repo.Address = address
	}

	if cmd.Flags().Changed("port") {
		repo.Port = port
	}

	if cmd.Flags().Changed("root") {
		repo.ServerRoot = serverRoot
	}
}
