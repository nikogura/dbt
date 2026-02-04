// Copyright © 2025 Nik Ogura <nik.ogura@gmail.com>
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
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmdStructure(t *testing.T) {
	// Test that the root command is properly configured
	assert.Equal(t, "catalog", RootCmd.Use, "Command use should be 'catalog'")
	assert.Equal(t, "Tool for showing available DBT tools.", RootCmd.Short, "Short description should match")
	assert.Contains(t, RootCmd.Long, "trusted repository", "Long description should mention trusted repository")
}

func TestRootCmdPersistentFlags(t *testing.T) {
	// Test that persistent flags are properly defined
	flags := RootCmd.PersistentFlags()

	// versions flag
	versionsFlag := flags.Lookup("versions")
	assert.NotNil(t, versionsFlag, "versions flag should exist")
	assert.Equal(t, "v", versionsFlag.Shorthand, "versions shorthand should be 'v'")
	assert.Equal(t, "false", versionsFlag.DefValue, "versions default should be false")

	// verbose flag
	verboseFlag := flags.Lookup("verbose")
	assert.NotNil(t, verboseFlag, "verbose flag should exist")
	assert.Equal(t, "V", verboseFlag.Shorthand, "verbose shorthand should be 'V'")
	assert.Equal(t, "false", verboseFlag.DefValue, "verbose default should be false")
}

func TestRootCmdHelp(t *testing.T) {
	// Test that help output contains expected content
	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetArgs([]string{"--help"})

	err := RootCmd.Execute()
	require.NoError(t, err, "Help command should not error")

	output := buf.String()
	assert.Contains(t, output, "Tool for showing available DBT tools", "Help should contain description")
	assert.Contains(t, output, "--versions", "Help should mention versions flag")
	assert.Contains(t, output, "--verbose", "Help should mention verbose flag")
}

func TestRootCmdHasSubcommands(t *testing.T) {
	// Test that the list subcommand is registered
	listFound := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "list" {
			listFound = true
			break
		}
	}
	assert.True(t, listFound, "list subcommand should be registered")
}
