// Copyright © 2022 Nik Ogura <nik.ogura@gmail.com>
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
	"github.com/spf13/cobra"
)

// typesCmd represents the create command
var typesCmd = &cobra.Command{
	Use:   "types",
	Short: "Lists available boilerplate types.",
	Long: `
Lists available boilerplate types.
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Valid Project types:\n")
		for _, t := range boilerplate.ValidProjectTypes() {
			fmt.Printf("  %s\n", t)
		}
	},
}

func init() {
	RootCmd.AddCommand(typesCmd)
}
