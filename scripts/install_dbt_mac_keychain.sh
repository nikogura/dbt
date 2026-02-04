#!/bin/bash
# install_dbt_mac_keychain.sh - Install dbt (Dynamic Binary Toolkit) on macOS with Keychain support
#
# Usage: install_dbt_mac_keychain.sh [server_name]
#
# Arguments:
#   server_name  Optional. Name of the server to use from multi-server config.
#                If not provided, uses the default server.
#
# Environment variables:
#   DBT_SERVER   Alternative way to specify server name (lower priority than argument)
#
# This script is specifically for macOS and sets up dbt with Keychain integration
# for secure credential storage.
#
# Examples:
#   ./install_dbt_mac_keychain.sh           # Uses default server
#   ./install_dbt_mac_keychain.sh prod      # Uses server named "prod"
#   DBT_SERVER=staging ./install_dbt_mac_keychain.sh  # Uses "staging" via env var

set -e

# Verify we're on macOS
if [ "$(uname -s)" != "Darwin" ]; then
    echo "This script is for macOS only. Please use install_dbt.sh for other platforms."
    exit 1
fi

SERVER_NAME="${1:-${DBT_SERVER:-}}"

if [ -n "$SERVER_NAME" ]; then
    echo "Installing dbt using server: $SERVER_NAME"
    export DBT_SERVER="$SERVER_NAME"
fi

# Determine architecture
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Detected macOS on $ARCH architecture"

# Check for existing dbt installation
if command -v dbt &> /dev/null; then
    echo "dbt is already installed. Running upgrade..."
    if [ -n "$SERVER_NAME" ]; then
        dbt -s "$SERVER_NAME" -- catalog list
    else
        dbt -- catalog list
    fi
    exit 0
fi

# Determine install location
INSTALL_DIR="${HOME}/.local/bin"
if [ ! -d "$INSTALL_DIR" ]; then
    mkdir -p "$INSTALL_DIR"
fi

# Check for Homebrew and suggest adding to PATH
if command -v brew &> /dev/null; then
    BREW_PREFIX=$(brew --prefix)
    echo "Homebrew detected at $BREW_PREFIX"
fi

# Add to PATH if not already there
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo "Adding $INSTALL_DIR to PATH"
        echo ""
        echo "Add the following to your shell profile (.zshrc or .bash_profile):"
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
        ;;
esac

echo ""
echo "Installation complete!"
echo ""
echo "If you haven't already, add $INSTALL_DIR to your PATH:"
echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
echo ""
if [ -n "$SERVER_NAME" ]; then
    echo "To use dbt with server '$SERVER_NAME':"
    echo "  dbt -s $SERVER_NAME -- catalog list"
else
    echo "To get started:"
    echo "  dbt -- catalog list"
fi
