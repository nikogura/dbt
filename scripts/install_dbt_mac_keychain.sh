#!/bin/bash
# install_dbt_mac_keychain.sh - Install dbt on macOS with Keychain support
#
# Fully userspace installation - no sudo required.
# Automatically configures shell PATH and optionally stores credentials in Keychain.
#
# Quick install:
#   curl -fsSL https://dbt.example.com/install_dbt_mac_keychain.sh | bash -s -- --url https://dbt.example.com
#
# With Keychain credential storage:
#   ./install_dbt_mac_keychain.sh --url https://dbt.example.com --use-keychain

set -e

# macOS only
[[ "$(uname -s)" != "Darwin" ]] && { echo "This script is for macOS. Use install_dbt.sh for Linux."; exit 1; }

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

Install dbt to ~/.local/bin on macOS (no sudo required).
Optionally stores credentials in macOS Keychain.

Required:
    --url URL               Base URL of the dbt repository server

Options:
    --name NAME             Server alias (default: derived from URL)
    --add                   Add server to existing config
    --default               Make this the default server (with --add)
    --replace               Replace existing installation entirely
    --oidc-issuer URL       OIDC issuer for authentication
    --oidc-audience AUD     OIDC audience (default: dbt-server)
    --use-keychain          Store credentials in macOS Keychain
    --install-dir DIR       Install directory (default: ~/.local/bin)
    --no-modify-profile     Don't modify shell profile
    -y, --yes               Non-interactive mode
    -h, --help              Show this help

Quick install:
    curl -fsSL https://dbt.example.com/install_dbt_mac_keychain.sh | bash -s -- --url https://dbt.example.com

Examples:
    $0 --url https://dbt.example.com
    $0 --url https://dbt.example.com --oidc-issuer https://dex.example.com
    $0 --url https://dbt.example.com --use-keychain
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
USE_KEYCHAIN=false
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
        --use-keychain) USE_KEYCHAIN=true; shift ;;
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

# Derive server name
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

# Architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    arm64) ARCH="arm64" ;;
    *) error "Unsupported architecture: $ARCH" ;;
esac

# Paths
CONFIG_DIR="${HOME}/.dbt/conf"
CONFIG_FILE="${CONFIG_DIR}/dbt.json"
DBT_BINARY="${INSTALL_DIR}/dbt"

echo ""
echo "dbt installer (macOS)"
echo "====================="
echo "Server: $SERVER_URL"
echo "Name:   $SERVER_NAME"
echo "Arch:   $ARCH"
[[ "$USE_KEYCHAIN" == "true" ]] && echo "Auth:   Keychain"
[[ -n "$OIDC_ISSUER" ]] && echo "Auth:   OIDC ($OIDC_ISSUER)"
echo ""

# Check existing
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
    if command -v jq &>/dev/null && jq -e ".servers.\"$SERVER_NAME\"" "$CONFIG_FILE" &>/dev/null; then
        if [[ "$NON_INTERACTIVE" == "true" ]]; then
            error "Server '$SERVER_NAME' already configured. Use --replace or --add with different --name"
        fi
        warn "Server '$SERVER_NAME' already exists"
        echo "  1) Update server config"
        echo "  2) Cancel"
        read -p "Choose [1/2]: " -n 1 -r; echo
        [[ "$REPLY" == "1" ]] && MODE="add" || { info "Cancelled"; exit 0; }
    else
        MODE="add"
        info "Adding '$SERVER_NAME' to existing config"
    fi
fi

# Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"

# Download
step "Downloading dbt..."
DBT_URL="${SERVER_URL}/dbt/darwin/${ARCH}/dbt"
CHECKSUM_URL="${DBT_URL}.sha256"

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

if ! curl -fsSL -o "$TEMP_DIR/dbt" "$DBT_URL" 2>/dev/null; then
    error "Failed to download from $DBT_URL"
fi

# Verify checksum
if curl -fsSL -o "$TEMP_DIR/dbt.sha256" "$CHECKSUM_URL" 2>/dev/null; then
    EXPECTED=$(cat "$TEMP_DIR/dbt.sha256")
    ACTUAL=$(shasum -a 256 "$TEMP_DIR/dbt" | cut -d' ' -f1)
    [[ "$EXPECTED" != "$ACTUAL" ]] && error "Checksum mismatch!"
    info "Checksum verified"
fi

# Install
chmod +x "$TEMP_DIR/dbt"
xattr -d com.apple.quarantine "$TEMP_DIR/dbt" 2>/dev/null || true

if [[ "$MODE" == "fresh" || "$MODE" == "replace" ]]; then
    cp "$TEMP_DIR/dbt" "$DBT_BINARY"
    info "Installed to $DBT_BINARY"
elif [[ "$EXISTING_DBT" == "false" ]]; then
    cp "$TEMP_DIR/dbt" "$DBT_BINARY"
    info "Installed to $DBT_BINARY"
fi

# Keychain setup
if [[ "$USE_KEYCHAIN" == "true" ]]; then
    step "Setting up Keychain..."

    KEYCHAIN_ACCOUNT="dbt-${SERVER_NAME}"
    KEYCHAIN_SERVICE_USER="${KEYCHAIN_ACCOUNT}-username"
    KEYCHAIN_SERVICE_PASS="${KEYCHAIN_ACCOUNT}-password"

    # Check existing
    if security find-generic-password -a "$KEYCHAIN_ACCOUNT" -s "$KEYCHAIN_SERVICE_USER" &>/dev/null; then
        if [[ "$NON_INTERACTIVE" == "true" ]]; then
            info "Using existing Keychain credentials"
        else
            warn "Credentials for '$SERVER_NAME' exist in Keychain"
            read -p "Update? [y/N]: " -n 1 -r; echo
            if [[ "$REPLY" =~ ^[Yy]$ ]]; then
                read -p "Username: " KC_USER
                read -s -p "Password: " KC_PASS; echo
                security add-generic-password -a "$KEYCHAIN_ACCOUNT" -s "$KEYCHAIN_SERVICE_USER" -w "$KC_USER" -U
                security add-generic-password -a "$KEYCHAIN_ACCOUNT" -s "$KEYCHAIN_SERVICE_PASS" -w "$KC_PASS" -U
                info "Updated Keychain credentials"
            fi
        fi
    else
        if [[ "$NON_INTERACTIVE" == "true" ]]; then
            warn "Keychain credentials not found - skipping (run interactively to set)"
            USE_KEYCHAIN=false
        else
            read -p "Username: " KC_USER
            read -s -p "Password: " KC_PASS; echo
            security add-generic-password -a "$KEYCHAIN_ACCOUNT" -s "$KEYCHAIN_SERVICE_USER" -w "$KC_USER"
            security add-generic-password -a "$KEYCHAIN_ACCOUNT" -s "$KEYCHAIN_SERVICE_PASS" -w "$KC_PASS"
            info "Stored credentials in Keychain"
        fi
    fi
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
    elif [[ "$USE_KEYCHAIN" == "true" ]]; then
        j+=",\"usernamefunc\":\"security find-generic-password -a dbt-${SERVER_NAME} -s dbt-${SERVER_NAME}-username -w\""
        j+=",\"passwordfunc\":\"security find-generic-password -a dbt-${SERVER_NAME} -s dbt-${SERVER_NAME}-password -w\""
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
    info "Created config"

elif [[ "$MODE" == "add" ]]; then
    if [[ ! -f "$CONFIG_FILE" ]]; then
        cat > "$CONFIG_FILE" <<EOF
{
    "servers": {
        "$SERVER_NAME": $SERVER_JSON
    },
    "defaultServer": "$SERVER_NAME"
}
EOF
    elif ! command -v jq &>/dev/null; then
        # Try homebrew
        if command -v brew &>/dev/null; then
            step "Installing jq..."
            brew install jq >/dev/null 2>&1
        else
            error "jq required. Install via: brew install jq"
        fi
    fi

    if command -v jq &>/dev/null; then
        if jq -e '.servers' "$CONFIG_FILE" &>/dev/null; then
            if [[ "$MAKE_DEFAULT" == "true" ]]; then
                jq --arg n "$SERVER_NAME" --argjson s "$SERVER_JSON" \
                   '.servers[$n]=$s | .defaultServer=$n' "$CONFIG_FILE" > "$TEMP_DIR/config.json"
            else
                jq --arg n "$SERVER_NAME" --argjson s "$SERVER_JSON" \
                   '.servers[$n]=$s' "$CONFIG_FILE" > "$TEMP_DIR/config.json"
            fi
            mv "$TEMP_DIR/config.json" "$CONFIG_FILE"
        else
            # Convert legacy
            LEGACY_REPO=$(jq -r '.dbt.repository // empty' "$CONFIG_FILE")
            LEGACY_TRUST=$(jq -r '.dbt.truststore // empty' "$CONFIG_FILE")
            LEGACY_TOOLS=$(jq -r '.tools.repository // empty' "$CONFIG_FILE")
            LEGACY_AUTH=$(jq -r '.authType // empty' "$CONFIG_FILE")
            LEGACY_ISSUER=$(jq -r '.issuerUrl // empty' "$CONFIG_FILE")
            LEGACY_AUD=$(jq -r '.oidcAudience // empty' "$CONFIG_FILE")
            LEGACY_UFUNC=$(jq -r '.usernamefunc // empty' "$CONFIG_FILE")
            LEGACY_PFUNC=$(jq -r '.passwordfunc // empty' "$CONFIG_FILE")

            LEGACY_JSON='{"repository":"'$LEGACY_REPO'"'
            [[ -n "$LEGACY_TRUST" ]] && LEGACY_JSON+=',"truststore":"'$LEGACY_TRUST'"'
            [[ -n "$LEGACY_TOOLS" ]] && LEGACY_JSON+=',"toolsRepository":"'$LEGACY_TOOLS'"'
            [[ -n "$LEGACY_AUTH" ]] && LEGACY_JSON+=',"authType":"'$LEGACY_AUTH'"'
            [[ -n "$LEGACY_ISSUER" ]] && LEGACY_JSON+=',"issuerUrl":"'$LEGACY_ISSUER'"'
            [[ -n "$LEGACY_AUD" ]] && LEGACY_JSON+=',"oidcAudience":"'$LEGACY_AUD'"'
            [[ -n "$LEGACY_UFUNC" ]] && LEGACY_JSON+=',"usernamefunc":"'$LEGACY_UFUNC'"'
            [[ -n "$LEGACY_PFUNC" ]] && LEGACY_JSON+=',"passwordfunc":"'$LEGACY_PFUNC'"'
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
        fi
    fi
    info "Updated config"
fi

# Update shell profile
update_profile() {
    local profile="$1"
    local marker="# dbt PATH"

    [[ ! -f "$profile" ]] && return 1
    grep -q "$marker" "$profile" 2>/dev/null && return 2  # Already set up by us
    grep -qE "PATH=.*\\.local/bin" "$profile" 2>/dev/null && return 2  # Already has it

    cat >> "$profile" <<EOF

$marker
export PATH="\$HOME/.local/bin:\$PATH"
EOF
    return 0
}

if [[ "$MODIFY_PROFILE" == "true" ]]; then
    step "Configuring shell..."

    PROFILE_UPDATED=false
    PATH_LINE='export PATH="$HOME/.local/bin:$PATH"'
    MARKER="# dbt PATH"

    # Detect current shell
    CURRENT_SHELL=$(basename "${SHELL:-/bin/zsh}")

    # macOS defaults to zsh since Catalina
    TARGET_PROFILE="$HOME/.zshrc"

    # Check if PATH already configured
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
        if [[ ! -f "$TARGET_PROFILE" ]]; then
            touch "$TARGET_PROFILE"
        fi
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
            if [[ ! -f "$TARGET_PROFILE" ]]; then
                touch "$TARGET_PROFILE"
            fi
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
            echo "To add dbt to your PATH manually, add this line to ~/.zshrc:"
            echo ""
            echo "    $PATH_LINE"
        fi
    fi

    # Also update bash profile if user is using bash and it exists
    if [[ "$CURRENT_SHELL" == "bash" ]]; then
        BASH_PROFILE=""
        if [[ -f "$HOME/.bash_profile" ]]; then
            BASH_PROFILE="$HOME/.bash_profile"
        elif [[ -f "$HOME/.bashrc" ]]; then
            BASH_PROFILE="$HOME/.bashrc"
        fi

        if [[ -n "$BASH_PROFILE" ]]; then
            if ! grep -q "$MARKER" "$BASH_PROFILE" 2>/dev/null && \
               ! grep -qE "PATH=.*\\.local/bin" "$BASH_PROFILE" 2>/dev/null; then
                if [[ "$NON_INTERACTIVE" == "true" ]]; then
                    cat >> "$BASH_PROFILE" <<EOF

$MARKER
export PATH="\$HOME/.local/bin:\$PATH"
EOF
                    info "Also added to $BASH_PROFILE"
                fi
            fi
        fi
    fi
fi

# PATH check
PATH_OK=false
case ":$PATH:" in
    *":$INSTALL_DIR:"*) PATH_OK=true ;;
esac

# Done
echo ""
echo -e "${GREEN}Installation complete!${NC}"
echo ""

if [[ "$PATH_OK" == "false" ]]; then
    echo "To start using dbt, either:"
    echo ""
    echo "  1. Open a new terminal, or"
    echo "  2. Run: source ~/.zshrc"
    echo ""
fi

echo "Then run:"
echo ""
echo "  dbt -- catalog list"
echo ""

if command -v jq &>/dev/null && jq -e '.servers | length > 1' "$CONFIG_FILE" &>/dev/null 2>&1; then
    echo "Configured servers:"
    jq -r '.servers | keys[]' "$CONFIG_FILE" | sed 's/^/  - /'
    echo ""
    echo "Default: $(jq -r '.defaultServer' "$CONFIG_FILE")"
    echo ""
    echo "Use a specific server:"
    echo "  dbt -s $SERVER_NAME -- catalog list"
fi
