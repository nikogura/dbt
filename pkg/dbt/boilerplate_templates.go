package dbt

// DEFAULT_GITIGNORE_TEMPLATE the contents of the .gitignore file
const DEFAULT_GITIGNORE_TEMPLATE = `# Created by .ignore support plugin (hsz.mobi)
### Go template
# Binaries for programs and plugins
*.exe
*.dll
*.so
*.dylib

# Test binary, build with 'go test -c'
*.test

# Output of the go coverage tool, specifically when used with LiteIDE
*.out

# Project-local glide cache, RE: https://github.com/Masterminds/glide/issues/736
.glide/

# IntelliJ
/out/

.idea/
*.iml
`

// DEFAULT_METADATA_TEMPLATE contents of an initial metadata.json file
const DEFAULT_METADATA_TEMPLATE = `{
  "name": "{{.ToolName}}",
  "version": "0.1.0",
  "package": "{{.PackageName}}",
  "description": "{{.PackageDescription}}",
  "repository": "{{.Repository}}",
  "building": {
    "targets": [
      {
        "name": "darwin/amd64"
      },
      {
        "name": "linux/amd64"
      }
    ],
    "extras": [
      {
        "template": "templates/description.tmpl",
        "filename": "description.txt",
        "executable": false
      }
    ]
  },
  "signing": {
    "program": "gpg",
    "email": "you@yourmail.com"

  },
  "publishing": {
    "targets": [
      {
        "src": "description.txt",
        "dst": "__REPOSITORY__/__TOOLNAME__/__VERSION__/description.txt",
        "sig": true,
        "checksums": false
      },
      {
        "src": "{{.ToolName}}_darwin_amd64",
        "dst": "__REPOSITORY__/__TOOLNAME__/__VERSION__/darwin/amd64/__TOOLNAME__",
        "sig": true,
        "checksums": false
      },
      {
        "src": "{{.ToolName}}_linux_amd64",
        "dst": "__REPOSITORY__/__TOOLNAME__/__VERSION__/linux/amd64/__TOOLNAME__",
        "sig": true,
        "checksums": false
      }
    ],
    "usernamefunc": "echo -n $PUBLISH_USERNAME",
    "passwordfunc": "echo -n $PUBLISH_PASSWORD"
  }
}`

// DEFAULT_PREHOOK_TEMPLATE a git pre-commit hook
const DEFAULT_PREHOOK_TEMPLATE = `#!/usr/bin/env bash
/usr/local/go/bin/gofmt -w ./`

// DEFAULT_GOMODULE_TEMPLATE requirements for a basic tool
const DEFAULT_GOMODULE_TEMPLATE = `module {{.PackageName}}

require (
	github.com/davecgh/go-spew v0.0.0-20171005155431-ecdeabc65495
	github.com/fsnotify/fsnotify v0.0.0-20170329110642-4da3e2cfbabc
	github.com/hashicorp/hcl v0.0.0-20171017181929-23c074d0eceb
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/magiconair/properties v0.0.0-20171031211101-49d762b9817b
	github.com/mitchellh/go-homedir v0.0.0-20180523094522-3864e76763d9
	github.com/mitchellh/mapstructure v0.0.0-20171017171808-06020f85339e
	github.com/pelletier/go-toml v0.0.0-20171222114548-0131db6d737c
	github.com/pmezard/go-difflib v1.0.0
	github.com/spf13/afero v0.0.0-20171228125011-57afd63c6860
	github.com/spf13/cast v1.1.0
	github.com/spf13/cobra v0.0.0-20171207074935-ccaecb155a21
	github.com/spf13/jwalterweatherman v0.0.0-20170901151539-12bd96e66386
	github.com/spf13/pflag v0.0.0-20171106142849-4c012f6dcd95
	github.com/spf13/viper v0.0.0-20171227194143-aafc9e6bc7b7
	github.com/stretchr/testify v0.0.0-20171018052257-2aa2c176b9da
	golang.org/x/sys v0.0.0-20171222143536-83801418e1b5
	golang.org/x/text v0.0.0-20171227012246-e19ae1496984
	gopkg.in/yaml.v2 v2.0.0-20171116090243-287cf08546ab
)
`

// DEFAULT_MAIN_GO_TEMPLATE cobra main.go file
const DEFAULT_MAIN_GO_TEMPLATE = `
// Copyright © {{.CopyrightYear}} {{.Author.Name}} <{{.Author.Email}}>
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

package main

import "{{.PackageName}}/cmd"

func main() {
	cmd.Execute()
}`

// DEFAULT_ROOT_GO_TEMPLATE root.go cobra file
const DEFAULT_ROOT_GO_TEMPLATE = `// Copyright © {{.CopyrightYear}} {{.Author.Name}} <{{.Author.Email}}>
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
	"os"
	"{{.PackageName}}/pkg/{{.ToolName}}"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "{{.ToolName}}",
	Short: "{{.PackageDescription}}",
	// Note:  Change the following line to backticks for long multiline descriptions.
	Long: "{{.PackageDescription}}",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		{{.ToolName}}.Run()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.foo.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

	// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".foo" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".foo")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
	fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
`

// DEFAULT_LICENSE_TEMPLATE Apache license
const DEFAULT_LICENSE_TEMPLATE = `Apache License
	Version 2.0, January 2004
http://www.apache.org/licenses/

	TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

	1. Definitions.

		"License" shall mean the terms and conditions for use, reproduction,
		and distribution as defined by Sections 1 through 9 of this document.

		"Licensor" shall mean the copyright owner or entity authorized by
	the copyright owner that is granting the License.

		"Legal Entity" shall mean the union of the acting entity and all
	other entities that control, are controlled by, or are under common
	control with that entity. For the purposes of this definition,
		"control" means (i) the power, direct or indirect, to cause the
	direction or management of such entity, whether by contract or
	otherwise, or (ii) ownership of fifty percent (50%) or more of the
	outstanding shares, or (iii) beneficial ownership of such entity.

		"You" (or "Your") shall mean an individual or Legal Entity
	exercising permissions granted by this License.

		"Source" form shall mean the preferred form for making modifications,
		including but not limited to software source code, documentation
	source, and configuration files.

		"Object" form shall mean any form resulting from mechanical
	transformation or translation of a Source form, including but
	not limited to compiled object code, generated documentation,
		and conversions to other media types.

		"Work" shall mean the work of authorship, whether in Source or
	Object form, made available under the License, as indicated by a
	copyright notice that is included in or attached to the work
	(an example is provided in the Appendix below).

	"Derivative Works" shall mean any work, whether in Source or Object
	form, that is based on (or derived from) the Work and for which the
	editorial revisions, annotations, elaborations, or other modifications
	represent, as a whole, an original work of authorship. For the purposes
	of this License, Derivative Works shall not include works that remain
	separable from, or merely link (or bind by name) to the interfaces of,
		the Work and Derivative Works thereof.

		"Contribution" shall mean any work of authorship, including
	the original version of the Work and any modifications or additions
	to that Work or Derivative Works thereof, that is intentionally
	submitted to Licensor for inclusion in the Work by the copyright owner
	or by an individual or Legal Entity authorized to submit on behalf of
	the copyright owner. For the purposes of this definition, "submitted"
	means any form of electronic, verbal, or written communication sent
	to the Licensor or its representatives, including but not limited to
	communication on electronic mailing lists, source code control systems,
		and issue tracking systems that are managed by, or on behalf of, the
	Licensor for the purpose of discussing and improving the Work, but
	excluding communication that is conspicuously marked or otherwise
	designated in writing by the copyright owner as "Not a Contribution."

	"Contributor" shall mean Licensor and any individual or Legal Entity
	on behalf of whom a Contribution has been received by Licensor and
	subsequently incorporated within the Work.

		2. Grant of Copyright License. Subject to the terms and conditions of
	this License, each Contributor hereby grants to You a perpetual,
		worldwide, non-exclusive, no-charge, royalty-free, irrevocable
	copyright license to reproduce, prepare Derivative Works of,
		publicly display, publicly perform, sublicense, and distribute the
	Work and such Derivative Works in Source or Object form.

		3. Grant of Patent License. Subject to the terms and conditions of
	this License, each Contributor hereby grants to You a perpetual,
		worldwide, non-exclusive, no-charge, royalty-free, irrevocable
	(except as stated in this section) patent license to make, have made,
		use, offer to sell, sell, import, and otherwise transfer the Work,
		where such license applies only to those patent claims licensable
	by such Contributor that are necessarily infringed by their
	Contribution(s) alone or by combination of their Contribution(s)
	with the Work to which such Contribution(s) was submitted. If You
	institute patent litigation against any entity (including a
	cross-claim or counterclaim in a lawsuit) alleging that the Work
	or a Contribution incorporated within the Work constitutes direct
	or contributory patent infringement, then any patent licenses
	granted to You under this License for that Work shall terminate
	as of the date such litigation is filed.

		4. Redistribution. You may reproduce and distribute copies of the
	Work or Derivative Works thereof in any medium, with or without
	modifications, and in Source or Object form, provided that You
	meet the following conditions:

	(a) You must give any other recipients of the Work or
	Derivative Works a copy of this License; and

	(b) You must cause any modified files to carry prominent notices
	stating that You changed the files; and

	(c) You must retain, in the Source form of any Derivative Works
	that You distribute, all copyright, patent, trademark, and
	attribution notices from the Source form of the Work,
		excluding those notices that do not pertain to any part of
	the Derivative Works; and

	(d) If the Work includes a "NOTICE" text file as part of its
	distribution, then any Derivative Works that You distribute must
	include a readable copy of the attribution notices contained
	within such NOTICE file, excluding those notices that do not
	pertain to any part of the Derivative Works, in at least one
	of the following places: within a NOTICE text file distributed
	as part of the Derivative Works; within the Source form or
	documentation, if provided along with the Derivative Works; or,
		within a display generated by the Derivative Works, if and
	wherever such third-party notices normally appear. The contents
	of the NOTICE file are for informational purposes only and
	do not modify the License. You may add Your own attribution
	notices within Derivative Works that You distribute, alongside
	or as an addendum to the NOTICE text from the Work, provided
	that such additional attribution notices cannot be construed
	as modifying the License.

	You may add Your own copyright statement to Your modifications and
	may provide additional or different license terms and conditions
	for use, reproduction, or distribution of Your modifications, or
	for any such Derivative Works as a whole, provided Your use,
		reproduction, and distribution of the Work otherwise complies with
	the conditions stated in this License.

		5. Submission of Contributions. Unless You explicitly state otherwise,
		any Contribution intentionally submitted for inclusion in the Work
	by You to the Licensor shall be under the terms and conditions of
	this License, without any additional terms or conditions.
	Notwithstanding the above, nothing herein shall supersede or modify
	the terms of any separate license agreement you may have executed
	with Licensor regarding such Contributions.

		6. Trademarks. This License does not grant permission to use the trade
	names, trademarks, service marks, or product names of the Licensor,
		except as required for reasonable and customary use in describing the
	origin of the Work and reproducing the content of the NOTICE file.

		7. Disclaimer of Warranty. Unless required by applicable law or
	agreed to in writing, Licensor provides the Work (and each
	Contributor provides its Contributions) on an "AS IS" BASIS,
		WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
	implied, including, without limitation, any warranties or conditions
	of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
	PARTICULAR PURPOSE. You are solely responsible for determining the
	appropriateness of using or redistributing the Work and assume any
	risks associated with Your exercise of permissions under this License.

		8. Limitation of Liability. In no event and under no legal theory,
		whether in tort (including negligence), contract, or otherwise,
		unless required by applicable law (such as deliberate and grossly
	negligent acts) or agreed to in writing, shall any Contributor be
	liable to You for damages, including any direct, indirect, special,
		incidental, or consequential damages of any character arising as a
	result of this License or out of the use or inability to use the
	Work (including but not limited to damages for loss of goodwill,
		work stoppage, computer failure or malfunction, or any and all
	other commercial damages or losses), even if such Contributor
	has been advised of the possibility of such damages.

		9. Accepting Warranty or Additional Liability. While redistributing
	the Work or Derivative Works thereof, You may choose to offer,
		and charge a fee for, acceptance of support, warranty, indemnity,
		or other liability obligations and/or rights consistent with this
	License. However, in accepting such obligations, You may act only
	on Your own behalf and on Your sole responsibility, not on behalf
	of any other Contributor, and only if You agree to indemnify,
		defend, and hold each Contributor harmless for any liability
	incurred by, or claims asserted against, such Contributor by reason
	of your accepting any such warranty or additional liability.

	END OF TERMS AND CONDITIONS

APPENDIX: How to apply the Apache License to your work.

	To apply the Apache License to your work, attach the following
	boilerplate notice, with the fields enclosed by brackets "[]"
	replaced with your own identifying information. (Don't include
	the brackets!)  The text should be enclosed in the appropriate
	comment syntax for the file format. We also recommend that a
	file or class name and description of purpose be included on the
	same "printed page" as the copyright notice for easier
	identification within third-party archives.

	Copyright {{.CopyrightYear}} {{.Author.Name}}

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.`

// DEFAULT_EMPTY_GO_TEMPLATE an empty go file in the tool package
const DEFAULT_EMPTY_GO_TEMPLATE = `package {{.ToolName}}

import (
	"fmt"
)

func Run() {
	fmt.Println("It works")
}`

// DEFAULT_DESCRIPTION_TEMPLATE description template
const DEFAULT_DESCRIPTION_TEMPLATE = "{{.Description}}"

// DEFAULT_README_TEMPLATE a boilerplate readme stub
const DEFAULT_README_TEMPLATE = "# {{.ToolName}}\n\n{{.PackageDescription}}\n"
