#!/bin/bash
# install_dbt.sh - Install dbt (Dynamic Binary Toolkit)
#
# Usage: install_dbt.sh [server_name]
#
# Arguments:
#   server_name  Optional. Name of the server to use from multi-server config.
#                If not provided, uses the default server.
#
# Environment variables:
#   DBT_SERVER   Alternative way to specify server name (lower priority than argument)
#
# Examples:
#   ./install_dbt.sh           # Uses default server
#   ./install_dbt.sh prod      # Uses server named "prod"
#   DBT_SERVER=staging ./install_dbt.sh  # Uses "staging" via env var

set -e

SERVER_NAME="${1:-${DBT_SERVER:-}}"

if [ -n "$SERVER_NAME" ]; then
    echo "Installing dbt using server: $SERVER_NAME"
    export DBT_SERVER="$SERVER_NAME"
fi

# Determine OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Detected OS: $OS, Architecture: $ARCH"

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

# Add to PATH if not already there
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo "Adding $INSTALL_DIR to PATH"
        echo ""
        echo "Add the following to your shell profile (.bashrc, .zshrc, etc.):"
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
