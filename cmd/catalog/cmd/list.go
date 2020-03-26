// Copyright © 2017 Nik Ogura <nik.ogura@gmail.com>
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

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "ListCatalog available tools.",
	Long: `
ListCatalog available tools.
`,
	Run: func(cmd *cobra.Command, args []string) {
		dbtObj, err := dbt.NewDbt("")
		if err != nil {
			log.Fatalf("Error creating DBT object: %s", err)
		}

		dbtObj.SetVerbose(verbose)

		err = dbtObj.FetchCatalog(versions)
		if err != nil {
			fmt.Printf("Error running list: %s\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}
