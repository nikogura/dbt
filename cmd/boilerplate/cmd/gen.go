// Copyright © 2018 Nik Ogura <nik.ogura@gmail.com>
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
	"github.com/nikogura/dbt/pkg/dbt"
	"github.com/spf13/cobra"
	"log"
)

// genCmd represents the create command
var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Creates a new DBT tool.",
	Long: `
Creates a new DBT tool.

When run, it will ask you for a tool name, package and description.  It will also ask you for the name and email address of the author.  This information is used to generate all the boilerplate files and such.

Then it will generate a basic, working tool for you that will compile and publish, but won't do much more than that.  The rest is up to you.

`,
	Run: func(cmd *cobra.Command, args []string) {
		commandName, err := dbt.PromptForToolName()
		if err != nil {
			log.Fatalf("Error getting tool name: %s", err)
		}

		packageName, err := dbt.PromptForToolPackage()
		if err != nil {
			log.Fatalf("Error getting tool package: %s", err)
		}

		packageDescription, err := dbt.PromptForToolDescription()
		if err != nil {
			log.Fatalf("Error getting tool description: %s", err)
		}

		author, err := dbt.PromptForToolAuthor()
		if err != nil {
			log.Fatalf("Eror getting tool author: %s", err)
		}

		repository, err := dbt.PromptForToolRepo()
		if err != nil {
			log.Fatalf("Eror getting tool repository: %s", err)
		}

		err = dbt.WriteConfigFiles(commandName, packageName, packageDescription, author, repository)
		if err != nil {
			log.Fatalf("Error generating boilerplate files: %s", err)
		}
	},
}

func init() {
	RootCmd.AddCommand(genCmd)
}
