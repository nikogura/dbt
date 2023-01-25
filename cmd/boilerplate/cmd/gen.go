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
	"fmt"
	"github.com/nikogura/dbt/pkg/boilerplate"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"log"
	"os"
)

var projectType string
var destDir string

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
		if projectType == "" {
			if len(args) > 0 {
				projectType = args[0]
			}
		}

		if destDir == "" {
			cwd, err := os.Getwd()
			if err != nil {
				log.Fatalf("failed getting current working directory")
			}

			destDir = cwd
		}

		if !boilerplate.IsValidProjectType(projectType) {
			log.Fatalf("invalid project type: %q. Valid project types are: %s", projectType, boilerplate.ValidProjectTypes())
		}

		fmt.Printf("Creating new project of type %q\n", projectType)

		data, err := boilerplate.PromptsForProject(projectType)
		if err != nil {
			log.Fatalf("prompts processing error: %v", err)
		}

		datamap, err := data.AsMap()
		if err != nil {
			log.Fatalf("failed converting project data to map")
		}

		wr, err := boilerplate.NewTmplWriter(afero.NewOsFs(), projectType, datamap)
		if err != nil {
			log.Fatalf("failed to create template writer: %v", err)
		}

		if err = wr.BuildProject(destDir); err != nil {
			log.Fatalf("failed to create templated project: %v", err)
		}

		fmt.Printf("New project created in ./%s\n", datamap["ProjectName"])

	},
}

func init() {
	RootCmd.AddCommand(genCmd)
	genCmd.Flags().StringVarP(&projectType, "type", "t", "cobra", "Project Type")
	genCmd.Flags().StringVarP(&destDir, "dest-dir", "d", "", "Destination Directory (Defaults to CWD")
}
