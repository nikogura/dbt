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

func TestListCmdStructure(t *testing.T) {
	// Test that the list command is properly configured
	assert.Equal(t, "list", listCmd.Use, "Command use should be 'list'")
	assert.Equal(t, "ListCatalog available tools.", listCmd.Short, "Short description should match")
	assert.Contains(t, listCmd.Long, "ListCatalog available tools", "Long description should describe listing")
}

func TestListCmdHelp(t *testing.T) {
	// Test that help output contains expected content
	buf := new(bytes.Buffer)
	listCmd.SetOut(buf)

	// Use Help() directly to get the help output
	err := listCmd.Help()
	require.NoError(t, err, "Help command should not error")

	output := buf.String()
	assert.Contains(t, output, "ListCatalog available tools", "Help should contain description")
}

func TestListCmdInheritsFlags(t *testing.T) {
	// Test that the list command inherits persistent flags from root
	// This verifies the parent-child relationship is set up correctly

	// Check that parent is set (indicating it's registered as a subcommand)
	assert.NotNil(t, listCmd.Parent(), "list command should have a parent")
	assert.Equal(t, "catalog", listCmd.Parent().Use, "list command parent should be catalog")
}

func TestListCmdRunIsSet(t *testing.T) {
	// Test that the Run function is defined
	assert.NotNil(t, listCmd.Run, "list command should have Run function defined")
}
