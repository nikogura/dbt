package boilerplate

// GitignoreContents the contents of the .gitignore file
func GitignoreContents() string {
	return `# Created by .ignore support plugin (hsz.mobi)
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

vendor/**
!vendor/vendor.json`
}

// MetadataContents contents of an initial metadata.json file
func MetadataContents() string {
	return `{
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
}

// PreCommitHookContents a git pre-commit hook
func PreCommitHookContents() string {
	return `#!/usr/bin/env bash
/usr/local/go/bin/gofmt -w ./`
}

// VendorJsonContents  a vendor.json sufficient for a newly created tool to build
func VendorJsonContents() string {
	return `{
	"comment": "",
	"ignore": "test",
	"package": [
		{
			"checksumSHA1": "mrz/kicZiUaHxkyfvC/DyQcr8Do=",
			"path": "github.com/davecgh/go-spew/spew",
			"revision": "ecdeabc65495df2dec95d7c4a4c3e021903035e5",
			"revisionTime": "2017-10-02T20:02:53Z"
		},
		{
			"checksumSHA1": "x2Km0Qy3WgJJnV19Zv25VwTJcBM=",
			"path": "github.com/fsnotify/fsnotify",
			"revision": "4da3e2cfbabc9f751898f250b49f2439785783a1",
			"revisionTime": "2017-03-29T04:21:07Z"
		},
		{
			"checksumSHA1": "HtpYAWHvd9mq+mHkpo7z8PGzMik=",
			"path": "github.com/hashicorp/hcl",
			"revision": "23c074d0eceb2b8a5bfdbb271ab780cde70f05a8",
			"revisionTime": "2017-10-17T18:19:29Z"
		},
		{
			"checksumSHA1": "XQmjDva9JCGGkIecOgwtBEMCJhU=",
			"path": "github.com/hashicorp/hcl/hcl/ast",
			"revision": "23c074d0eceb2b8a5bfdbb271ab780cde70f05a8",
			"revisionTime": "2017-10-17T18:19:29Z"
		},
		{
			"checksumSHA1": "/15SVLnCDzxICSatuYbfctrcpSM=",
			"path": "github.com/hashicorp/hcl/hcl/parser",
			"revision": "23c074d0eceb2b8a5bfdbb271ab780cde70f05a8",
			"revisionTime": "2017-10-17T18:19:29Z"
		},
		{
			"checksumSHA1": "WR1BjzDKgv6uE+3ShcDTYz0Gl6A=",
			"path": "github.com/hashicorp/hcl/hcl/printer",
			"revision": "23c074d0eceb2b8a5bfdbb271ab780cde70f05a8",
			"revisionTime": "2017-10-17T18:19:29Z"
		},
		{
			"checksumSHA1": "PYDzRc61T0pbwWuLNHgBRp/gJII=",
			"path": "github.com/hashicorp/hcl/hcl/scanner",
			"revision": "23c074d0eceb2b8a5bfdbb271ab780cde70f05a8",
			"revisionTime": "2017-10-17T18:19:29Z"
		},
		{
			"checksumSHA1": "oS3SCN9Wd6D8/LG0Yx1fu84a7gI=",
			"path": "github.com/hashicorp/hcl/hcl/strconv",
			"revision": "23c074d0eceb2b8a5bfdbb271ab780cde70f05a8",
			"revisionTime": "2017-10-17T18:19:29Z"
		},
		{
			"checksumSHA1": "c6yprzj06ASwCo18TtbbNNBHljA=",
			"path": "github.com/hashicorp/hcl/hcl/token",
			"revision": "23c074d0eceb2b8a5bfdbb271ab780cde70f05a8",
			"revisionTime": "2017-10-17T18:19:29Z"
		},
		{
			"checksumSHA1": "PwlfXt7mFS8UYzWxOK5DOq0yxS0=",
			"path": "github.com/hashicorp/hcl/json/parser",
			"revision": "23c074d0eceb2b8a5bfdbb271ab780cde70f05a8",
			"revisionTime": "2017-10-17T18:19:29Z"
		},
		{
			"checksumSHA1": "afrZ8VmAwfTdDAYVgNSXbxa4GsA=",
			"path": "github.com/hashicorp/hcl/json/scanner",
			"revision": "23c074d0eceb2b8a5bfdbb271ab780cde70f05a8",
			"revisionTime": "2017-10-17T18:19:29Z"
		},
		{
			"checksumSHA1": "fNlXQCQEnb+B3k5UDL/r15xtSJY=",
			"path": "github.com/hashicorp/hcl/json/token",
			"revision": "23c074d0eceb2b8a5bfdbb271ab780cde70f05a8",
			"revisionTime": "2017-10-17T18:19:29Z"
		},
		{
			"checksumSHA1": "40vJyUB4ezQSn/NSadsKEOrudMc=",
			"path": "github.com/inconshreveable/mousetrap",
			"revision": "76626ae9c91c4f2a10f34cad8ce83ea42c93bb75",
			"revisionTime": "2014-10-17T20:07:13Z"
		},
		{
			"checksumSHA1": "8ae1DyNE/yY9NvY3PmvtQdLBJnc=",
			"path": "github.com/magiconair/properties",
			"revision": "49d762b9817ba1c2e9d0c69183c2b4a8b8f1d934",
			"revisionTime": "2017-10-31T21:05:36Z"
		},
		{
			"checksumSHA1": "V/quM7+em2ByJbWBLOsEwnY3j/Q=",
			"path": "github.com/mitchellh/go-homedir",
			"revision": "b8bc1bf767474819792c23f32d8286a45736f1c6",
			"revisionTime": "2016-12-03T19:45:07Z"
		},
		{
			"checksumSHA1": "gILp4IL+xwXLH6tJtRLrnZ56F24=",
			"path": "github.com/mitchellh/mapstructure",
			"revision": "06020f85339e21b2478f756a78e295255ffa4d6a",
			"revisionTime": "2017-10-17T17:18:08Z"
		},
		{
			"checksumSHA1": "H5wlR62j1Ru5rKRDM9eCb6iUKLA=",
			"path": "github.com/pelletier/go-toml",
			"revision": "0131db6d737cfbbfb678f8b7d92e55e27ce46224",
			"revisionTime": "2017-12-22T11:45:48Z"
		},
		{
			"checksumSHA1": "LuFv4/jlrmFNnDb/5SCSEPAM9vU=",
			"path": "github.com/pmezard/go-difflib/difflib",
			"revision": "792786c7400a136282c1664665ae0a8db921c6c2",
			"revisionTime": "2016-01-10T10:55:54Z"
		},
		{
			"checksumSHA1": "dW6L6oTOv4XfIahhwNzxb2Qu9to=",
			"path": "github.com/spf13/afero",
			"revision": "57afd63c68602b63ed976de00dd066ccb3c319db",
			"revisionTime": "2017-12-28T12:50:11Z"
		},
		{
			"checksumSHA1": "X6RueW0rO55PbOQ0sMWSQOxVl4I=",
			"path": "github.com/spf13/afero/mem",
			"revision": "57afd63c68602b63ed976de00dd066ccb3c319db",
			"revisionTime": "2017-12-28T12:50:11Z"
		},
		{
			"checksumSHA1": "Sq0QP4JywTr7UM4hTK1cjCi7jec=",
			"path": "github.com/spf13/cast",
			"revision": "acbeb36b902d72a7a4c18e8f3241075e7ab763e4",
			"revisionTime": "2017-04-13T08:50:28Z"
		},
		{
			"checksumSHA1": "aG5wPXVGAEu90TjPFNZFRtox2Zo=",
			"path": "github.com/spf13/cobra",
			"revision": "ccaecb155a2177302cb56cae929251a256d0f646",
			"revisionTime": "2017-12-07T07:49:35Z"
		},
		{
			"checksumSHA1": "suLj1G8Vd//a/a3sUEKz/ROalz0=",
			"path": "github.com/spf13/jwalterweatherman",
			"revision": "12bd96e66386c1960ab0f74ced1362f66f552f7b",
			"revisionTime": "2017-09-01T15:06:07Z"
		},
		{
			"checksumSHA1": "fKq6NiaqP3DFxnCRF5mmpJWTSUA=",
			"path": "github.com/spf13/pflag",
			"revision": "4c012f6dcd9546820e378d0bdda4d8fc772cdfea",
			"revisionTime": "2017-11-06T14:28:49Z"
		},
		{
			"checksumSHA1": "GWX9W5F1QBqLZsS1bYsG3jXjb3g=",
			"path": "github.com/spf13/viper",
			"revision": "aafc9e6bc7b7bb53ddaa75a5ef49a17d6e654be5",
			"revisionTime": "2017-11-29T09:51:06Z"
		},
		{
			"checksumSHA1": "mGbTYZ8dHVTiPTTJu3ktp+84pPI=",
			"path": "github.com/stretchr/testify/assert",
			"revision": "2aa2c176b9dab406a6970f6a55f513e8a8c8b18f",
			"revisionTime": "2017-08-14T20:04:35Z"
		},
		{
			"checksumSHA1": "ZwEZK9AUUeSqDnWUO4fDc9rwTIA=",
			"path": "golang.org/x/sys/unix",
			"revision": "83801418e1b59fb1880e363299581ee543af32ca",
			"revisionTime": "2017-12-22T10:59:23Z"
		},
		{
			"checksumSHA1": "ziMb9+ANGRJSSIuxYdRbA+cDRBQ=",
			"path": "golang.org/x/text/transform",
			"revision": "e19ae1496984b1c655b8044a65c0300a3c878dd3",
			"revisionTime": "2017-12-24T20:31:28Z"
		},
		{
			"checksumSHA1": "BCNYmf4Ek93G4lk5x3ucNi/lTwA=",
			"path": "golang.org/x/text/unicode/norm",
			"revision": "e19ae1496984b1c655b8044a65c0300a3c878dd3",
			"revisionTime": "2017-12-24T20:31:28Z"
		},
		{
			"checksumSHA1": "fRgp9UZPllOlkPssv7frzQx4z9A=",
			"path": "gopkg.in/yaml.v2",
			"revision": "287cf08546ab5e7e37d55a84f7ed3fd1db036de5",
			"revisionTime": "2017-11-16T09:02:43Z"
		},
		{
			"checksumSHA1": "SvPS4zqZYImMcBu+dLgp6+9eeUo=",
			"path": "github.com/mitchellh/go-homedir",
			"revision": "3864e76763d94a6df2f9960b16a20a33da9f9a66",
			"revisionTime": "2018-05-23T09:45:22Z"
		}
	],
	"rootPath": "{{.PackageName}}"
}`
}

// MainGoContents cobra main.go file
func MainGoContents() string {
	return `
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
}

// RootGoContents root.go cobra file
func RootGoContents() string {
	return `// Copyright © {{.CopyrightYear}} {{.Author.Name}} <{{.Author.Email}}>
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
}

// LicenseContents Apache license
func LicenseContents() string {
	return `Apache License
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
}

// EmptyGoFileContents an empty go file in the tool package
func EmptyGoFileContents() string {
	return `package {{.ToolName}}

import (
	"fmt"
)

func Run() {
	fmt.Println("It works")
}`
}

// DescriptionTemplateContents description template
func DescriptionTemplateContents() string {
	return "{{.Description}}"
}

// ReadMeContents a boilerplate readme stub
func ReadMeContents() string {
	return "# {{.ToolName}}\n\n{{.PackageDescription}}\n"
}
