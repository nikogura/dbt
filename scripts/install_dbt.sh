#!/bin/bash
# install_dbt.sh - Install dbt (Dynamic Binary Toolkit)
#
# Fully userspace installation - no sudo required.
# Automatically configures shell PATH.
#
# Quick install (pipe to bash):
#   curl -fsSL https://dbt.example.com/install_dbt.sh | bash -s -- --url https://dbt.example.com
#
# Or download and run:
#   ./install_dbt.sh --url https://dbt.example.com
#
# Installation modes:
#   Fresh install (default) - Downloads dbt, creates config, updates shell PATH
#   Add server (--add)      - Adds a new server to existing multi-server config
#   Clobber (--replace)     - Replaces everything (binary + config)

set -e

# Colors (disabled if not a terminal)
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    NC='\033[0m'
else
    RED='' GREEN='' YELLOW='' BLUE='' NC=''
fi

info() { echo -e "${GREEN}✓${NC} $1"; }
warn() { echo -e "${YELLOW}!${NC} $1"; }
error() { echo -e "${RED}✗${NC} $1"; exit 1; }
step() { echo -e "${BLUE}→${NC} $1"; }

usage() {
    cat <<EOF
Usage: $0 --url <server_url> [options]

Install dbt to ~/.local/bin (no sudo required).

Required:
    --url URL               Base URL of the dbt repository server

Options:
    --name NAME             Server alias (default: derived from URL)
    --add                   Add server to existing config
    --default               Make this the default server (with --add)
    --replace               Replace existing installation entirely
    --oidc-issuer URL       OIDC issuer for authentication
    --oidc-audience AUD     OIDC audience (default: dbt-server)
    --install-dir DIR       Install directory (default: ~/.local/bin)
    --no-modify-profile     Don't modify shell profile
    -y, --yes               Non-interactive mode
    -h, --help              Show this help

Quick install:
    curl -fsSL https://dbt.example.com/install_dbt.sh | bash -s -- --url https://dbt.example.com

Examples:
    $0 --url https://dbt.example.com
    $0 --url https://dbt.example.com --oidc-issuer https://dex.example.com
    $0 --url https://dbt.staging.example.com --name staging --add
EOF
}

# Parse arguments
SERVER_URL=""
SERVER_NAME=""
REPLACE=false
ADD_SERVER=false
MAKE_DEFAULT=false
OIDC_ISSUER=""
OIDC_AUDIENCE="dbt-server"
INSTALL_DIR="${HOME}/.local/bin"
MODIFY_PROFILE=true
NON_INTERACTIVE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --url) SERVER_URL="$2"; shift 2 ;;
        --name) SERVER_NAME="$2"; shift 2 ;;
        --replace) REPLACE=true; shift ;;
        --add) ADD_SERVER=true; shift ;;
        --default) MAKE_DEFAULT=true; shift ;;
        --oidc-issuer) OIDC_ISSUER="$2"; shift 2 ;;
        --oidc-audience) OIDC_AUDIENCE="$2"; shift 2 ;;
        --install-dir) INSTALL_DIR="$2"; shift 2 ;;
        --no-modify-profile) MODIFY_PROFILE=false; shift ;;
        -y|--yes) NON_INTERACTIVE=true; shift ;;
        -h|--help) usage; exit 0 ;;
        *) error "Unknown option: $1" ;;
    esac
done

[[ -z "$SERVER_URL" ]] && error "Server URL required. Use: $0 --url <url>"

# Normalize URL
SERVER_URL="${SERVER_URL%/}"

# Derive server name from URL if not provided
if [[ -z "$SERVER_NAME" ]]; then
    HOSTNAME=$(echo "$SERVER_URL" | sed -E 's|^https?://||' | cut -d'/' -f1 | cut -d':' -f1)
    FIRST=$(echo "$HOSTNAME" | cut -d'.' -f1)
    if [[ "$FIRST" == "dbt" ]]; then
        SECOND=$(echo "$HOSTNAME" | cut -d'.' -f2)
        [[ -n "$SECOND" && "$SECOND" != "example" && "$SECOND" != "com" ]] && SERVER_NAME="$SECOND" || SERVER_NAME="default"
    else
        SERVER_NAME="$FIRST"
    fi
fi

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) error "Unsupported architecture: $ARCH" ;;
esac

# Paths
CONFIG_DIR="${HOME}/.dbt/conf"
CONFIG_FILE="${CONFIG_DIR}/dbt.json"
DBT_BINARY="${INSTALL_DIR}/dbt"

echo ""
echo "dbt installer"
echo "============="
echo "Server: $SERVER_URL"
echo "Name:   $SERVER_NAME"
echo "OS:     $OS/$ARCH"
echo ""

# Check existing installation
EXISTING_DBT=false
EXISTING_CONFIG=false
[[ -x "$DBT_BINARY" ]] || command -v dbt &>/dev/null && EXISTING_DBT=true
[[ -f "$CONFIG_FILE" ]] && EXISTING_CONFIG=true

# Determine mode
MODE="fresh"
if [[ "$REPLACE" == "true" ]]; then
    MODE="replace"
elif [[ "$ADD_SERVER" == "true" ]]; then
    MODE="add"
elif [[ "$EXISTING_CONFIG" == "true" ]]; then
    # Check if this server already exists
    if command -v jq &>/dev/null && jq -e ".servers.\"$SERVER_NAME\"" "$CONFIG_FILE" &>/dev/null; then
        if [[ "$NON_INTERACTIVE" == "true" ]]; then
            error "Server '$SERVER_NAME' already configured. Use --replace or --add with different --name"
        fi
        warn "Server '$SERVER_NAME' already exists in config"
        echo "  1) Update server '$SERVER_NAME' config"
        echo "  2) Cancel"
        read -p "Choose [1/2]: " -n 1 -r; echo
        [[ "$REPLY" == "1" ]] && MODE="add" || { info "Cancelled"; exit 0; }
    else
        MODE="add"
        info "Adding server '$SERVER_NAME' to existing config"
    fi
fi

# Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"

# Download dbt
step "Downloading dbt..."
DBT_URL="${SERVER_URL}/dbt/${OS}/${ARCH}/dbt"
CHECKSUM_URL="${DBT_URL}.sha256"

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

if ! curl -fsSL -o "$TEMP_DIR/dbt" "$DBT_URL" 2>/dev/null; then
    error "Failed to download from $DBT_URL"
fi

# Verify checksum
if curl -fsSL -o "$TEMP_DIR/dbt.sha256" "$CHECKSUM_URL" 2>/dev/null; then
    EXPECTED=$(cat "$TEMP_DIR/dbt.sha256")
    if command -v sha256sum &>/dev/null; then
        ACTUAL=$(sha256sum "$TEMP_DIR/dbt" | cut -d' ' -f1)
    else
        ACTUAL=$(shasum -a 256 "$TEMP_DIR/dbt" | cut -d' ' -f1)
    fi
    [[ "$EXPECTED" != "$ACTUAL" ]] && error "Checksum mismatch!"
    info "Checksum verified"
fi

# Install binary
chmod +x "$TEMP_DIR/dbt"

# macOS: remove quarantine
[[ "$OS" == "darwin" ]] && xattr -d com.apple.quarantine "$TEMP_DIR/dbt" 2>/dev/null || true

if [[ "$MODE" == "fresh" || "$MODE" == "replace" ]]; then
    cp "$TEMP_DIR/dbt" "$DBT_BINARY"
    info "Installed dbt to $DBT_BINARY"
elif [[ "$EXISTING_DBT" == "false" ]]; then
    cp "$TEMP_DIR/dbt" "$DBT_BINARY"
    info "Installed dbt to $DBT_BINARY"
fi

# Build server config
build_server_json() {
    local j='{'
    j+="\"repository\":\"${SERVER_URL}/dbt\""
    j+=",\"truststore\":\"${SERVER_URL}/dbt/truststore\""
    j+=",\"toolsRepository\":\"${SERVER_URL}/dbt-tools\""
    if [[ -n "$OIDC_ISSUER" ]]; then
        j+=",\"authType\":\"oidc\""
        j+=",\"issuerUrl\":\"${OIDC_ISSUER}\""
        j+=",\"oidcAudience\":\"${OIDC_AUDIENCE}\""
    fi
    j+='}'
    echo "$j"
}

# Update config
step "Configuring..."
SERVER_JSON=$(build_server_json)

if [[ "$MODE" == "fresh" || "$MODE" == "replace" ]]; then
    cat > "$CONFIG_FILE" <<EOF
{
    "servers": {
        "$SERVER_NAME": $SERVER_JSON
    },
    "defaultServer": "$SERVER_NAME"
}
EOF
    info "Created config: $CONFIG_FILE"

elif [[ "$MODE" == "add" ]]; then
    if [[ ! -f "$CONFIG_FILE" ]]; then
        # No config yet
        cat > "$CONFIG_FILE" <<EOF
{
    "servers": {
        "$SERVER_NAME": $SERVER_JSON
    },
    "defaultServer": "$SERVER_NAME"
}
EOF
    elif ! command -v jq &>/dev/null; then
        error "jq required to modify config. Install jq or use --replace"
    elif jq -e '.servers' "$CONFIG_FILE" &>/dev/null; then
        # Multi-server format
        if [[ "$MAKE_DEFAULT" == "true" ]]; then
            jq --arg n "$SERVER_NAME" --argjson s "$SERVER_JSON" \
               '.servers[$n]=$s | .defaultServer=$n' "$CONFIG_FILE" > "$TEMP_DIR/config.json"
        else
            jq --arg n "$SERVER_NAME" --argjson s "$SERVER_JSON" \
               '.servers[$n]=$s' "$CONFIG_FILE" > "$TEMP_DIR/config.json"
        fi
        mv "$TEMP_DIR/config.json" "$CONFIG_FILE"
    else
        # Legacy format - convert
        LEGACY_REPO=$(jq -r '.dbt.repository // empty' "$CONFIG_FILE")
        LEGACY_TRUST=$(jq -r '.dbt.truststore // empty' "$CONFIG_FILE")
        LEGACY_TOOLS=$(jq -r '.tools.repository // empty' "$CONFIG_FILE")
        LEGACY_AUTH=$(jq -r '.authType // empty' "$CONFIG_FILE")
        LEGACY_ISSUER=$(jq -r '.issuerUrl // empty' "$CONFIG_FILE")
        LEGACY_AUD=$(jq -r '.oidcAudience // empty' "$CONFIG_FILE")

        LEGACY_JSON='{"repository":"'$LEGACY_REPO'"'
        [[ -n "$LEGACY_TRUST" ]] && LEGACY_JSON+=',"truststore":"'$LEGACY_TRUST'"'
        [[ -n "$LEGACY_TOOLS" ]] && LEGACY_JSON+=',"toolsRepository":"'$LEGACY_TOOLS'"'
        [[ -n "$LEGACY_AUTH" ]] && LEGACY_JSON+=',"authType":"'$LEGACY_AUTH'"'
        [[ -n "$LEGACY_ISSUER" ]] && LEGACY_JSON+=',"issuerUrl":"'$LEGACY_ISSUER'"'
        [[ -n "$LEGACY_AUD" ]] && LEGACY_JSON+=',"oidcAudience":"'$LEGACY_AUD'"'
        LEGACY_JSON+='}'

        DEFAULT=$([[ "$MAKE_DEFAULT" == "true" ]] && echo "$SERVER_NAME" || echo "legacy")
        cat > "$CONFIG_FILE" <<EOF
{
    "servers": {
        "legacy": $LEGACY_JSON,
        "$SERVER_NAME": $SERVER_JSON
    },
    "defaultServer": "$DEFAULT"
}
EOF
        info "Converted legacy config, added '$SERVER_NAME'"
    fi
    info "Updated config"
fi

# Update shell profile for PATH
update_shell_profile() {
    local profile="$1"
    local marker="# dbt PATH"

    [[ ! -f "$profile" ]] && return 1

    # Check if already configured
    if grep -q "$marker" "$profile" 2>/dev/null; then
        return 2  # Already set up by us
    fi

    # Check if PATH already includes install dir
    if grep -qE "PATH=.*\\.local/bin" "$profile" 2>/dev/null; then
        return 2  # Already has it
    fi

    # Add to profile
    cat >> "$profile" <<EOF

$marker
export PATH="\$HOME/.local/bin:\$PATH"
EOF
    return 0
}

if [[ "$MODIFY_PROFILE" == "true" ]]; then
    step "Configuring shell PATH..."

    PROFILE_UPDATED=false
    PATH_LINE='export PATH="$HOME/.local/bin:$PATH"'

    # Detect current shell
    CURRENT_SHELL=$(basename "${SHELL:-/bin/bash}")

    # Determine target profile
    case "$CURRENT_SHELL" in
        zsh)
            TARGET_PROFILE="$HOME/.zshrc"
            ;;
        bash)
            if [[ -f "$HOME/.bashrc" ]]; then
                TARGET_PROFILE="$HOME/.bashrc"
            else
                TARGET_PROFILE="$HOME/.bash_profile"
            fi
            ;;
        *)
            # Default to bashrc for unknown shells
            TARGET_PROFILE="$HOME/.bashrc"
            ;;
    esac

    # On macOS, prefer zshrc (default shell since Catalina)
    if [[ "$OS" == "darwin" ]]; then
        TARGET_PROFILE="$HOME/.zshrc"
    fi

    # Check if PATH already configured
    MARKER="# dbt PATH"
    ALREADY_CONFIGURED=false

    if [[ -f "$TARGET_PROFILE" ]]; then
        if grep -q "$MARKER" "$TARGET_PROFILE" 2>/dev/null || \
           grep -qE "PATH=.*\\.local/bin" "$TARGET_PROFILE" 2>/dev/null; then
            ALREADY_CONFIGURED=true
        fi
    fi

    if [[ "$ALREADY_CONFIGURED" == "true" ]]; then
        info "PATH already configured in $TARGET_PROFILE"
        PROFILE_UPDATED=true
    elif [[ "$NON_INTERACTIVE" == "true" ]]; then
        # Non-interactive: auto-add
        cat >> "$TARGET_PROFILE" <<EOF

$MARKER
export PATH="\$HOME/.local/bin:\$PATH"
EOF
        info "Added to $TARGET_PROFILE:"
        echo "    $PATH_LINE"
        PROFILE_UPDATED=true
    else
        # Interactive: ask user
        echo ""
        echo "Detected shell: $CURRENT_SHELL"
        echo "To use dbt, ~/.local/bin must be in your PATH."
        echo ""
        echo "Add this line to $TARGET_PROFILE?"
        echo "    $PATH_LINE"
        echo ""
        read -p "Update $TARGET_PROFILE? [Y/n]: " -n 1 -r; echo
        if [[ ! "$REPLY" =~ ^[Nn]$ ]]; then
            cat >> "$TARGET_PROFILE" <<EOF

$MARKER
export PATH="\$HOME/.local/bin:\$PATH"
EOF
            info "Added to $TARGET_PROFILE:"
            echo "    $PATH_LINE"
            PROFILE_UPDATED=true
        else
            warn "Skipped shell profile update"
            echo ""
            echo "To add dbt to your PATH manually, add this line:"
            echo ""
            echo "    $PATH_LINE"
            echo ""
            echo "  For bash: add to ~/.bashrc or ~/.bash_profile"
            echo "  For zsh:  add to ~/.zshrc"
        fi
    fi
fi

# Check if PATH is current
PATH_OK=false
case ":$PATH:" in
    *":$INSTALL_DIR:"*) PATH_OK=true ;;
esac

# Done!
echo ""
echo -e "${GREEN}Installation complete!${NC}"
echo ""

if [[ "$PATH_OK" == "false" ]]; then
    echo "To start using dbt, either:"
    echo ""
    echo "  1. Open a new terminal, or"
    echo "  2. Run: export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
fi

echo "Then run:"
echo ""
echo "  dbt -- catalog list"
echo ""

# Show multi-server info if applicable
if command -v jq &>/dev/null && jq -e '.servers | length > 1' "$CONFIG_FILE" &>/dev/null 2>&1; then
    echo "Configured servers:"
    jq -r '.servers | keys[]' "$CONFIG_FILE" | sed 's/^/  - /'
    echo ""
    echo "Default: $(jq -r '.defaultServer' "$CONFIG_FILE")"
    echo ""
    echo "Use a specific server:"
    echo "  dbt -s $SERVER_NAME -- catalog list"
fi
