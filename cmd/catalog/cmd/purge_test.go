package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPurgeCmdStructure(t *testing.T) {
	assert.Equal(t, "purge <toolname>", purgeCmd.Use, "Command use should be 'purge <toolname>'")
	assert.Equal(t, "Purge tool versions from the repository.", purgeCmd.Short, "Short description should match")
	assert.Contains(t, purgeCmd.Long, "Purge tool versions", "Long description should describe purging")
}

func TestPurgeCmdHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	purgeCmd.SetOut(buf)

	err := purgeCmd.Help()
	require.NoError(t, err, "Help command should not error")

	output := buf.String()
	assert.Contains(t, output, "Purge tool versions", "Help should contain description")
	assert.Contains(t, output, "--all", "Help should document --all flag")
	assert.Contains(t, output, "--older-than", "Help should document --older-than flag")
	assert.Contains(t, output, "--keep", "Help should document --keep flag")
	assert.Contains(t, output, "--dry-run", "Help should document --dry-run flag")
	assert.Contains(t, output, "--yes", "Help should document --yes flag")
}

func TestPurgeCmdInheritsFlags(t *testing.T) {
	assert.NotNil(t, purgeCmd.Parent(), "purge command should have a parent")
	assert.Equal(t, "catalog", purgeCmd.Parent().Use, "purge command parent should be catalog")
}

func TestPurgeCmdRunIsSet(t *testing.T) {
	assert.NotNil(t, purgeCmd.Run, "purge command should have Run function defined")
}

func TestPurgeCmdFlags(t *testing.T) {
	// Verify all flags are registered
	allFlag := purgeCmd.Flags().Lookup("all")
	assert.NotNil(t, allFlag, "--all flag should be registered")

	olderThanFlag := purgeCmd.Flags().Lookup("older-than")
	assert.NotNil(t, olderThanFlag, "--older-than flag should be registered")

	keepFlag := purgeCmd.Flags().Lookup("keep")
	assert.NotNil(t, keepFlag, "--keep flag should be registered")

	keepLatestFlag := purgeCmd.Flags().Lookup("keep-latest")
	assert.NotNil(t, keepLatestFlag, "--keep-latest flag should be registered")

	dryRunFlag := purgeCmd.Flags().Lookup("dry-run")
	assert.NotNil(t, dryRunFlag, "--dry-run flag should be registered")

	yesFlag := purgeCmd.Flags().Lookup("yes")
	assert.NotNil(t, yesFlag, "--yes flag should be registered")

	// Verify -y shorthand for --yes
	assert.Equal(t, "y", yesFlag.Shorthand, "--yes should have -y shorthand")
}
