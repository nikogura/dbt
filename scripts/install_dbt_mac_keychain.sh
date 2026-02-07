#!/bin/bash
# install_dbt_mac_keychain.sh - Install dbt on macOS with Keychain support
#
# Fully userspace installation - no sudo required.
# Automatically configures shell PATH and optionally stores credentials in Keychain.
#
# Supports both HTTP and S3 backends.
#
# Quick install (HTTP):
#   curl -fsSL https://dbt.example.com/install_dbt_mac_keychain.sh | bash -s -- --url https://dbt.example.com
#
# Quick install (S3):
#   aws s3 cp s3://your-bucket/install_dbt_mac_keychain.sh - | bash -s -- --url s3://your-bucket
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
    --oidc-audience AUD     OIDC audience (default: server URL)
    --oidc-client-id ID     OIDC client ID (default: dbt-ssh for SSH, dbt for device flow)
    --oidc-client-secret S  OIDC client secret (optional)
    --connector-id ID       OIDC connector ID (use 'ssh' for SSH-OIDC token exchange)
    --s3-region REGION      AWS region for S3 URL (default: auto-detect or us-east-1)
    --tools-url URL         Tools repository URL (default: derived from --url)
    --use-keychain          Store credentials in macOS Keychain
    --install-dir DIR       Install directory (default: ~/.local/bin)
    --no-modify-profile     Don't modify shell profile
    -y, --yes               Non-interactive mode
    -h, --help              Show this help

Authentication modes:
    S3 URLs (s3://...)      Uses AWS credentials (no --oidc-* needed)
    HTTP with SSH-OIDC      Use --connector-id ssh (requires ssh-agent)
    HTTP with device flow   Default OIDC mode (opens browser)
    HTTP with Keychain      Use --use-keychain (Basic auth stored in Keychain)

Quick install (HTTP):
    curl -fsSL https://dbt.example.com/install_dbt_mac_keychain.sh | bash -s -- --url https://dbt.example.com

Quick install (S3):
    aws s3 cp s3://your-bucket/install_dbt_mac_keychain.sh - | bash -s -- --url s3://your-bucket

Quick install (SSH-OIDC):
    curl -fsSL https://dbt.example.com/install_dbt_mac_keychain.sh | bash -s -- \\
        --url https://dbt.example.com \\
        --oidc-issuer https://dex.example.com \\
        --connector-id ssh

Examples:
    $0 --url https://dbt.example.com
    $0 --url https://dbt.example.com --oidc-issuer https://dex.example.com
    $0 --url https://dbt.example.com --oidc-issuer https://dex.example.com --connector-id ssh
    $0 --url https://dbt.example.com --use-keychain
    $0 --url https://dbt.staging.example.com --name staging --add
    $0 --url s3://your-dbt-bucket                    # S3 backend (requires AWS CLI)
EOF
}

# Parse arguments
SERVER_URL=""
TOOLS_URL=""
SERVER_NAME=""
REPLACE=false
ADD_SERVER=false
MAKE_DEFAULT=false
OIDC_ISSUER=""
OIDC_AUDIENCE=""
OIDC_CLIENT_ID=""
OIDC_CLIENT_SECRET=""
CONNECTOR_ID=""
S3_REGION=""
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
        --oidc-client-id) OIDC_CLIENT_ID="$2"; shift 2 ;;
        --oidc-client-secret) OIDC_CLIENT_SECRET="$2"; shift 2 ;;
        --connector-id) CONNECTOR_ID="$2"; shift 2 ;;
        --s3-region) S3_REGION="$2"; shift 2 ;;
        --tools-url) TOOLS_URL="$2"; shift 2 ;;
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

# Detect if this is an S3 URL
IS_S3=false
S3_BUCKET=""
CONFIG_URL="$SERVER_URL"        # URL to use in dbt config (HTTP for runtime)
TOOLS_CONFIG_URL=""             # Tools URL for config

if [[ "$SERVER_URL" == s3://* ]]; then
    IS_S3=true
    command -v aws >/dev/null 2>&1 || error "AWS CLI required for S3 URLs"

    # Tools URL is required for S3 installations
    [[ -z "$TOOLS_URL" ]] && error "Tools URL required for S3. Use: --tools-url s3://your-tools-bucket"

    # Extract bucket name
    S3_BUCKET=$(echo "$SERVER_URL" | sed -E 's|^s3://||' | cut -d'/' -f1)

    # Auto-detect region if not provided
    if [[ -z "$S3_REGION" ]]; then
        S3_REGION=$(aws s3api get-bucket-location --bucket "$S3_BUCKET" --query 'LocationConstraint' --output text 2>/dev/null)
        # us-east-1 returns "None" or empty
        [[ -z "$S3_REGION" || "$S3_REGION" == "None" || "$S3_REGION" == "null" ]] && S3_REGION="us-east-1"
    fi

    # Convert S3 URL to HTTPS endpoint for dbt config
    CONFIG_URL="https://${S3_BUCKET}.s3.${S3_REGION}.amazonaws.com"

    # Convert tools URL to HTTPS endpoint
    if [[ "$TOOLS_URL" == s3://* ]]; then
        TOOLS_BUCKET=$(echo "$TOOLS_URL" | sed -E 's|^s3://||' | cut -d'/' -f1)
        TOOLS_CONFIG_URL="https://${TOOLS_BUCKET}.s3.${S3_REGION}.amazonaws.com"
    else
        TOOLS_CONFIG_URL="$TOOLS_URL"
    fi
else
    # HTTP server - tools URL optional, defaults to /dbt-tools path
    if [[ -z "$TOOLS_URL" ]]; then
        TOOLS_CONFIG_URL="${CONFIG_URL}/dbt-tools"
    else
        TOOLS_CONFIG_URL="$TOOLS_URL"
    fi
fi

# Set default audience if not provided (use server URL)
[[ -z "$OIDC_AUDIENCE" ]] && OIDC_AUDIENCE="$SERVER_URL"

# Set default client ID based on auth type
if [[ -z "$OIDC_CLIENT_ID" ]]; then
    if [[ "$CONNECTOR_ID" == "ssh" ]]; then
        OIDC_CLIENT_ID="dbt-ssh"
    else
        OIDC_CLIENT_ID="dbt"
    fi
fi

# OIDC token cache
CACHED_OIDC_TOKEN=""

# Create SSH-signed JWT using jwt-ssh-agent-go or Python fallback
create_ssh_signed_jwt() {
    local audience="$1"
    local username="${USER:-$(whoami)}"

    # Try jwt-ssh-agent-go (creates SSH-signed JWTs)
    if command -v jwt-ssh-agent-go >/dev/null 2>&1; then
        local jwt
        jwt=$(jwt-ssh-agent-go token --server "$audience" --username "$username" 2>/dev/null)
        if [[ -n "$jwt" && "$jwt" == *"."*"."* ]]; then
            echo "$jwt"
            return 0
        fi
    fi

    # Try Python with ssh-agent protocol (widely available fallback)
    if command -v python3 >/dev/null 2>&1; then
        local jwt
        jwt=$(python3 - "$audience" "$username" 2>/dev/null <<'PYTHON_EOF'
import sys
import os
import json
import base64
import time
import secrets
import socket
import struct

def b64url_encode(data):
    if isinstance(data, str):
        data = data.encode()
    return base64.urlsafe_b64encode(data).rstrip(b'=').decode()

def create_jwt_header_payload(audience, username):
    now = int(time.time())
    header = {"alg": "SSH", "typ": "JWT"}
    payload = {
        "iss": "kubectl-ssh-oidc",
        "sub": username,
        "aud": audience,
        "jti": secrets.token_hex(16),
        "exp": now + 300,
        "iat": now,
        "nbf": now
    }
    return b64url_encode(json.dumps(header)) + "." + b64url_encode(json.dumps(payload))

def sign_with_ssh_agent(data):
    """Sign data using SSH agent (RFC 4251 protocol)"""
    sock_path = os.environ.get('SSH_AUTH_SOCK')
    if not sock_path:
        return None

    try:
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.connect(sock_path)

        # Request identities (SSH2_AGENTC_REQUEST_IDENTITIES = 11)
        sock.sendall(struct.pack('>IB', 1, 11))

        # Read response length
        resp_len = struct.unpack('>I', sock.recv(4))[0]
        resp = sock.recv(resp_len)

        # Parse response (SSH2_AGENT_IDENTITIES_ANSWER = 12)
        if resp[0] != 12:
            return None

        num_keys = struct.unpack('>I', resp[1:5])[0]
        if num_keys == 0:
            return None

        # Parse first key
        offset = 5
        key_len = struct.unpack('>I', resp[offset:offset+4])[0]
        key_blob = resp[offset+4:offset+4+key_len]
        offset += 4 + key_len
        comment_len = struct.unpack('>I', resp[offset:offset+4])[0]
        offset += 4 + comment_len

        # Request signature (SSH2_AGENTC_SIGN_REQUEST = 13)
        data_bytes = data.encode() if isinstance(data, str) else data
        # flags=4 means RSA-SHA256 (SSH_AGENT_RSA_SHA2_256)
        flags = 4
        msg = struct.pack('>I', key_len) + key_blob + struct.pack('>I', len(data_bytes)) + data_bytes + struct.pack('>I', flags)
        sock.sendall(struct.pack('>IB', len(msg) + 1, 13) + msg)

        # Read signature response
        resp_len = struct.unpack('>I', sock.recv(4))[0]
        resp = sock.recv(resp_len)

        # Parse response (SSH2_AGENT_SIGN_RESPONSE = 14)
        if resp[0] != 14:
            return None

        sig_len = struct.unpack('>I', resp[1:5])[0]
        sig_blob = resp[5:5+sig_len]

        # sig_blob is: 4-byte algo name length + algo name + 4-byte sig length + sig
        algo_len = struct.unpack('>I', sig_blob[0:4])[0]
        actual_sig_offset = 4 + algo_len + 4
        actual_sig = sig_blob[actual_sig_offset:]

        sock.close()
        return actual_sig
    except Exception as e:
        return None

if __name__ == '__main__':
    if len(sys.argv) < 3:
        sys.exit(1)
    audience = sys.argv[1]
    username = sys.argv[2]

    header_payload = create_jwt_header_payload(audience, username)
    signature = sign_with_ssh_agent(header_payload)

    if signature:
        jwt = header_payload + "." + b64url_encode(signature)
        print(jwt)
        sys.exit(0)
    sys.exit(1)
PYTHON_EOF
)
        if [[ -n "$jwt" && "$jwt" == *"."*"."* ]]; then
            echo "$jwt"
            return 0
        fi
    fi

    return 1
}

# Exchange SSH-signed JWT for OIDC token with Dex (RFC 8693)
exchange_ssh_jwt_for_oidc() {
    local ssh_jwt="$1"
    local token_endpoint="$2"
    local client_id="$3"
    local client_secret="$4"
    local connector_id="$5"
    local audience="$6"

    local form_data="grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Atoken-exchange"
    form_data+="&subject_token_type=urn%3Aietf%3Aparams%3Aoauth%3Atoken-type%3Aaccess_token"
    form_data+="&subject_token=${ssh_jwt}"
    form_data+="&requested_token_type=urn%3Aietf%3Aparams%3Aoauth%3Atoken-type%3Aid_token"
    form_data+="&scope=openid%20email%20groups%20profile"
    form_data+="&connector_id=${connector_id}"
    form_data+="&client_id=${client_id}"
    [[ -n "$audience" ]] && form_data+="&audience=${audience}"
    [[ -n "$client_secret" ]] && form_data+="&client_secret=${client_secret}"

    local response
    response=$(curl -sL -X POST "$token_endpoint" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -H "Accept: application/json" \
        -d "$form_data")

    # Check for error
    local error_msg
    error_msg=$(echo "$response" | jq -r '.error // empty' 2>/dev/null)
    if [[ -n "$error_msg" ]]; then
        local error_desc
        error_desc=$(echo "$response" | jq -r '.error_description // .error' 2>/dev/null)
        echo "ERROR: $error_desc" >&2
        return 1
    fi

    # Extract token (prefer id_token, fall back to access_token)
    local token
    token=$(echo "$response" | jq -r '.id_token // .access_token // empty' 2>/dev/null)
    if [[ -n "$token" && "$token" != "null" ]]; then
        echo "$token"
        return 0
    fi

    return 1
}

# Get OIDC token via SSH-OIDC token exchange
get_ssh_oidc_token() {
    if [[ -n "$CACHED_OIDC_TOKEN" ]]; then
        echo "$CACHED_OIDC_TOKEN"
        return 0
    fi

    [[ -z "$OIDC_ISSUER" ]] && return 0

    step "Authenticating via SSH-OIDC token exchange..."

    # Check ssh-agent is available
    if [[ -z "$SSH_AUTH_SOCK" ]]; then
        error "SSH_AUTH_SOCK not set. Start ssh-agent and add your key: ssh-add"
    fi

    # Check for keys in agent
    if ! ssh-add -l >/dev/null 2>&1; then
        error "No SSH keys in agent. Add your key: ssh-add"
    fi

    # Get token endpoint
    local discovery_url="${OIDC_ISSUER}/.well-known/openid-configuration"
    local discovery_json
    discovery_json=$(curl -sL "$discovery_url") || error "Failed to fetch OIDC discovery from $discovery_url"

    local token_endpoint
    token_endpoint=$(echo "$discovery_json" | jq -r '.token_endpoint')
    [[ -z "$token_endpoint" || "$token_endpoint" == "null" ]] && error "No token endpoint in OIDC discovery"

    # Create SSH-signed JWT
    local ssh_jwt
    ssh_jwt=$(create_ssh_signed_jwt "$OIDC_ISSUER")
    if [[ -z "$ssh_jwt" ]]; then
        error "Failed to create SSH-signed JWT. Ensure jwt-ssh-agent-go is installed or python3 is available with ssh-agent running."
    fi

    # Exchange JWT for OIDC token
    local token
    token=$(exchange_ssh_jwt_for_oidc "$ssh_jwt" "$token_endpoint" "$OIDC_CLIENT_ID" "$OIDC_CLIENT_SECRET" "$CONNECTOR_ID" "$OIDC_AUDIENCE")
    if [[ -z "$token" ]]; then
        error "SSH-OIDC token exchange failed. Check that your SSH key is authorized in Dex."
    fi

    CACHED_OIDC_TOKEN="$token"
    info "SSH-OIDC authentication successful!"
    echo "$CACHED_OIDC_TOKEN"
}

# Get OIDC token via device code flow
get_device_flow_token() {
    if [[ -n "$CACHED_OIDC_TOKEN" ]]; then
        echo "$CACHED_OIDC_TOKEN"
        return 0
    fi

    [[ -z "$OIDC_ISSUER" ]] && return 0

    step "Authenticating via OIDC device flow..."

    # Discover OIDC endpoints
    local discovery_url="${OIDC_ISSUER}/.well-known/openid-configuration"
    local discovery_json
    discovery_json=$(curl -sL "$discovery_url") || error "Failed to fetch OIDC discovery from $discovery_url"

    local device_endpoint
    device_endpoint=$(echo "$discovery_json" | jq -r '.device_authorization_endpoint // empty')
    local token_endpoint
    token_endpoint=$(echo "$discovery_json" | jq -r '.token_endpoint')

    if [[ -z "$device_endpoint" ]]; then
        error "OIDC issuer does not support device code flow"
    fi

    # Request device code
    local device_response
    device_response=$(curl -sL -X POST "$device_endpoint" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "client_id=${OIDC_CLIENT_ID}" \
        -d "scope=openid profile email groups offline_access" \
        -d "audience=${OIDC_AUDIENCE}")

    local device_code user_code verification_uri interval expires_in
    device_code=$(echo "$device_response" | jq -r '.device_code')
    user_code=$(echo "$device_response" | jq -r '.user_code')
    verification_uri=$(echo "$device_response" | jq -r '.verification_uri // .verification_url')
    interval=$(echo "$device_response" | jq -r '.interval // 5')
    expires_in=$(echo "$device_response" | jq -r '.expires_in // 300')

    if [[ -z "$device_code" || "$device_code" == "null" ]]; then
        error "Failed to get device code: $(echo "$device_response" | jq -r '.error_description // .error // "unknown error"')"
    fi

    # Prompt user
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Open this URL in your browser:"
    echo "  ${verification_uri}"
    echo ""
    echo "  Enter code: ${user_code}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    # Try to open browser automatically (macOS)
    open "$verification_uri" 2>/dev/null || true

    # Poll for token
    local deadline=$((SECONDS + expires_in))
    while [[ $SECONDS -lt $deadline ]]; do
        sleep "$interval"

        local token_response
        token_response=$(curl -sL -X POST "$token_endpoint" \
            -H "Content-Type: application/x-www-form-urlencoded" \
            -d "grant_type=urn:ietf:params:oauth:grant-type:device_code" \
            -d "device_code=$device_code" \
            -d "client_id=${OIDC_CLIENT_ID}")

        local error_code
        error_code=$(echo "$token_response" | jq -r '.error // empty')

        case "$error_code" in
            "")
                # Success - got token
                CACHED_OIDC_TOKEN=$(echo "$token_response" | jq -r '.id_token // .access_token')
                if [[ -n "$CACHED_OIDC_TOKEN" && "$CACHED_OIDC_TOKEN" != "null" ]]; then
                    info "Authentication successful!"
                    echo "$CACHED_OIDC_TOKEN"
                    return 0
                fi
                ;;
            authorization_pending|slow_down)
                # Still waiting - continue polling
                ;;
            *)
                error "OIDC authentication failed: $(echo "$token_response" | jq -r '.error_description // .error')"
                ;;
        esac
    done

    error "OIDC authentication timed out"
}

# Get OIDC token - dispatches to appropriate flow
get_oidc_token() {
    if [[ "$CONNECTOR_ID" == "ssh" ]]; then
        get_ssh_oidc_token
    else
        get_device_flow_token
    fi
}

# Download helper - uses aws s3 cp for S3, curl with optional auth for HTTP
fetch() {
    local src="$1"
    local dst="$2"
    if [[ "$IS_S3" == "true" ]]; then
        aws s3 cp "$src" "$dst" --quiet
    elif [[ -n "$CACHED_OIDC_TOKEN" ]]; then
        curl -fsSL -H "Authorization: Bearer $CACHED_OIDC_TOKEN" -o "$dst" "$src"
    else
        curl -fsSL -o "$dst" "$src"
    fi
}

# Fetch to stdout helper
fetch_content() {
    local src="$1"
    if [[ "$IS_S3" == "true" ]]; then
        aws s3 cp "$src" -
    elif [[ -n "$CACHED_OIDC_TOKEN" ]]; then
        curl -fsSL -H "Authorization: Bearer $CACHED_OIDC_TOKEN" "$src"
    else
        curl -fsSL "$src"
    fi
}

# Derive server name
if [[ -z "$SERVER_NAME" ]]; then
    if [[ "$IS_S3" == "true" ]]; then
        # S3 URL: s3://bucket-name -> use bucket name
        BUCKET=$(echo "$SERVER_URL" | sed -E 's|^s3://||' | cut -d'/' -f1)
        # Strip common prefixes/suffixes
        SERVER_NAME=$(echo "$BUCKET" | sed -E 's/^(terrace-|company-)?dbt(-prod|-dev|-staging)?$/\2/' | sed 's/^-//' | sed 's/-$//')
        [[ -z "$SERVER_NAME" ]] && SERVER_NAME="default"
    else
        # HTTP URL
        HOSTNAME=$(echo "$SERVER_URL" | sed -E 's|^https?://||' | cut -d'/' -f1 | cut -d':' -f1)
        FIRST=$(echo "$HOSTNAME" | cut -d'.' -f1)
        if [[ "$FIRST" == "dbt" ]]; then
            SECOND=$(echo "$HOSTNAME" | cut -d'.' -f2)
            [[ -n "$SECOND" && "$SECOND" != "example" && "$SECOND" != "com" ]] && SERVER_NAME="$SECOND" || SERVER_NAME="default"
        else
            SERVER_NAME="$FIRST"
        fi
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

# Authenticate if OIDC configured (for HTTP servers)
if [[ "$IS_S3" != "true" && -n "$OIDC_ISSUER" ]]; then
    get_oidc_token >/dev/null
fi

# Get latest version
step "Fetching latest version..."
LATEST_URL="${SERVER_URL}/latest"
VERSION=$(fetch_content "$LATEST_URL" 2>/dev/null)
if [[ -z "$VERSION" ]]; then
    error "Failed to fetch latest version from $LATEST_URL"
fi
info "Latest version: $VERSION"

# Download
step "Downloading dbt..."
DBT_URL="${SERVER_URL}/${VERSION}/darwin/${ARCH}/dbt"
CHECKSUM_URL="${DBT_URL}.sha256"

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

if ! fetch "$DBT_URL" "$TEMP_DIR/dbt" 2>/dev/null; then
    error "Failed to download from $DBT_URL"
fi

# Verify checksum
if fetch "$CHECKSUM_URL" "$TEMP_DIR/dbt.sha256" 2>/dev/null; then
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

# Build server config (uses CONFIG_URL for HTTP access)
build_server_json() {
    local j='{'
    if [[ "$IS_S3" == "true" ]]; then
        # S3 structure: no /dbt prefix, truststore at root, separate tools bucket
        j+="\"repository\":\"${CONFIG_URL}\""
        j+=",\"truststore\":\"${CONFIG_URL}/truststore\""
        j+=",\"toolsRepository\":\"${TOOLS_CONFIG_URL}\""
    else
        # HTTP server structure: /dbt prefix
        j+="\"repository\":\"${CONFIG_URL}/dbt\""
        j+=",\"truststore\":\"${CONFIG_URL}/dbt/truststore\""
        j+=",\"toolsRepository\":\"${TOOLS_CONFIG_URL}\""
    fi
    if [[ -n "$OIDC_ISSUER" ]]; then
        j+=",\"authType\":\"oidc\""
        j+=",\"issuerUrl\":\"${OIDC_ISSUER}\""
        j+=",\"oidcAudience\":\"${OIDC_AUDIENCE}\""
        j+=",\"oidcClientId\":\"${OIDC_CLIENT_ID}\""
        [[ -n "$CONNECTOR_ID" ]] && j+=",\"connectorId\":\"${CONNECTOR_ID}\""
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
