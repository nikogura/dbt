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
	assert.Equal(t, "reposerver", rootCmd.Use, "Command use should be 'reposerver'")
	assert.Equal(t, "dbt repository server", rootCmd.Short, "Short description should match")
	assert.Contains(t, rootCmd.Long, "DBT Repository Server", "Long description should mention repository server")
	assert.Equal(t, "0.1.0", rootCmd.Version, "Version should be set")
}

func TestRootCmdFlags(t *testing.T) {
	// Test that flags are properly defined
	flags := rootCmd.Flags()

	// address flag
	addressFlag := flags.Lookup("address")
	assert.NotNil(t, addressFlag, "address flag should exist")
	assert.Equal(t, "a", addressFlag.Shorthand, "address shorthand should be 'a'")
	assert.Equal(t, "127.0.0.1", addressFlag.DefValue, "address default should be 127.0.0.1")

	// port flag
	portFlag := flags.Lookup("port")
	assert.NotNil(t, portFlag, "port flag should exist")
	assert.Equal(t, "p", portFlag.Shorthand, "port shorthand should be 'p'")
	assert.Equal(t, "9999", portFlag.DefValue, "port default should be 9999")

	// root flag (server root)
	rootFlag := flags.Lookup("root")
	assert.NotNil(t, rootFlag, "root flag should exist")
	assert.Equal(t, "r", rootFlag.Shorthand, "root shorthand should be 'r'")
	assert.Empty(t, rootFlag.DefValue, "root default should be empty")

	// file flag (config file)
	fileFlag := flags.Lookup("file")
	assert.NotNil(t, fileFlag, "file flag should exist")
	assert.Equal(t, "f", fileFlag.Shorthand, "file shorthand should be 'f'")
	assert.Empty(t, fileFlag.DefValue, "file default should be empty")
}

func TestRootCmdHelp(t *testing.T) {
	// Test that help output contains expected content
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	require.NoError(t, err, "Help command should not error")

	output := buf.String()
	assert.Contains(t, output, "DBT Repository Server", "Help should contain description")
	assert.Contains(t, output, "--address", "Help should mention address flag")
	assert.Contains(t, output, "--port", "Help should mention port flag")
	assert.Contains(t, output, "--root", "Help should mention root flag")
	assert.Contains(t, output, "--file", "Help should mention file flag")
}

func TestRootCmdVersion(t *testing.T) {
	// Test that version is properly set on the command
	// Note: The --version flag output goes to stdout, not the command's Out writer
	// So we verify the version is set correctly on the command itself
	assert.Equal(t, "0.1.0", rootCmd.Version, "Version should be 0.1.0")
}

func TestRootCmdExample(t *testing.T) {
	// Test that example is set
	assert.Equal(t, "dbt -- server", rootCmd.Example, "Example should be set correctly")
}

func TestRootCmdRunIsSet(t *testing.T) {
	// Test that the Run function is defined
	assert.NotNil(t, rootCmd.Run, "root command should have Run function defined")
}
