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
	assert.Equal(t, "dbt", rootCmd.Use, "Command use should be 'dbt'")
	assert.Equal(t, "Dynamic Binary Toolkit", rootCmd.Short, "Short description should match")
	assert.Contains(t, rootCmd.Long, "self-updating signed binaries", "Long description should mention self-updating")
	assert.NotEmpty(t, rootCmd.Version, "Version should be set")
}

func TestRootCmdFlags(t *testing.T) {
	// Test that flags are properly defined
	flags := rootCmd.Flags()

	// toolversion flag
	tvFlag := flags.Lookup("toolversion")
	assert.NotNil(t, tvFlag, "toolversion flag should exist")
	assert.Equal(t, "v", tvFlag.Shorthand, "toolversion shorthand should be 'v'")
	assert.Empty(t, tvFlag.DefValue, "toolversion default should be empty")

	// offline flag
	offlineFlag := flags.Lookup("offline")
	assert.NotNil(t, offlineFlag, "offline flag should exist")
	assert.Equal(t, "o", offlineFlag.Shorthand, "offline shorthand should be 'o'")
	assert.Equal(t, "false", offlineFlag.DefValue, "offline default should be false")

	// verbose flag
	verboseFlag := flags.Lookup("verbose")
	assert.NotNil(t, verboseFlag, "verbose flag should exist")
	assert.Equal(t, "V", verboseFlag.Shorthand, "verbose shorthand should be 'V'")
	assert.Equal(t, "false", verboseFlag.DefValue, "verbose default should be false")

	// server flag
	srvFlag := flags.Lookup("server")
	assert.NotNil(t, srvFlag, "server flag should exist")
	assert.Equal(t, "s", srvFlag.Shorthand, "server shorthand should be 's'")
	assert.Empty(t, srvFlag.DefValue, "server default should be empty")
}

func TestRootCmdHelp(t *testing.T) {
	// Test that help output contains expected content
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	require.NoError(t, err, "Help command should not error")

	output := buf.String()
	assert.Contains(t, output, "Dynamic Binary Toolkit", "Help should contain description")
	assert.Contains(t, output, "--toolversion", "Help should mention toolversion flag")
	assert.Contains(t, output, "--offline", "Help should mention offline flag")
	assert.Contains(t, output, "--verbose", "Help should mention verbose flag")
	assert.Contains(t, output, "--server", "Help should mention server flag")
}

func TestRootCmdVersion(t *testing.T) {
	// Test that version is properly set on the command
	// Note: The --version flag output goes to stdout, not the command's Out writer
	// Version is "dev" by default, injected at build time via ldflags for releases
	assert.NotEmpty(t, rootCmd.Version, "Version should be set")
}

func TestRootCmdExample(t *testing.T) {
	// Test that example is set
	assert.Equal(t, "dbt -s prod -- catalog list", rootCmd.Example, "Example should be set correctly")
}

func TestRootCmdLongDescriptionMentionsServer(t *testing.T) {
	// Test that long description mentions server selection
	assert.Contains(t, rootCmd.Long, "DBT_SERVER", "Long description should mention DBT_SERVER env var")
	assert.Contains(t, rootCmd.Long, "-s/--server", "Long description should mention server flag")
}
