#!/bin/bash
# install_dbt.sh - Install dbt (Dynamic Binary Toolkit)
#
# =============================================================================
# INSTALLATION OVERVIEW
# =============================================================================
#
# This installer supports three authentication methods:
#
# 1. S3 (AWS Authentication)
#    - Uses AWS CLI with configured credentials
#    - No additional auth needed beyond AWS setup
#
# 2. OIDC Device Flow (Browser-based)
#    - Opens browser for user authentication
#    - Works on any system with a browser
#
# 3. SSH-OIDC Token Exchange (Non-interactive)
#    - Uses SSH keys to authenticate via Dex (RFC 8693)
#    - Requires ssh-agent with loaded keys
#    - Ideal for automation and CI/CD
#
# =============================================================================
# SSH-OIDC AUTHENTICATION: HOW IT WORKS
# =============================================================================
#
# SSH-OIDC bridges SSH key-based authentication with OIDC token issuance.
# This allows users who have SSH keys registered in Dex to obtain OIDC tokens
# without browser interaction.
#
# The flow is:
#
# 1. CLIENT: Creates a JWT (JSON Web Token) with claims:
#    - iss: Issuer (e.g., "kubectl-ssh-oidc")
#    - sub: Subject (username)
#    - aud: Audience (Dex instance URL)
#    - exp/iat/nbf: Expiration/issued-at/not-before timestamps
#    - jti: Unique token identifier
#
# 2. CLIENT: Signs the JWT using an SSH private key
#    - The SSH agent holds the private key
#    - We use the agent's binary protocol to request a signature
#    - The signature is over: base64url(header) + "." + base64url(payload)
#
# 3. CLIENT -> DEX: Sends the signed JWT via RFC 8693 token exchange
#    - grant_type: urn:ietf:params:oauth:grant-type:token-exchange
#    - subject_token: The SSH-signed JWT
#    - connector_id: "ssh" to route to the SSH connector
#
# 4. DEX: Verifies the JWT signature
#    - Extracts the subject (username) from claims
#    - Looks up registered public keys for that user
#    - Verifies the signature using each registered key
#    - If valid, issues an OIDC token
#
# 5. CLIENT: Receives OIDC token for accessing protected resources
#
# =============================================================================
# SSH AGENT PROTOCOL
# =============================================================================
#
# The ssh-agent communicates via a Unix socket using a binary protocol.
# To sign data, we send a SSH2_AGENTC_SIGN_REQUEST message:
#
#   byte    SSH2_AGENTC_SIGN_REQUEST (13)
#   string  key_blob (public key in SSH wire format)
#   string  data_to_sign
#   uint32  flags (0 for default, 2 for RSA SHA-256)
#
# The agent responds with SSH2_AGENT_SIGN_RESPONSE:
#
#   byte    SSH2_AGENT_SIGN_RESPONSE (14)
#   string  signature_blob
#
# The signature blob itself contains:
#   string  algorithm (e.g., "ssh-ed25519", "rsa-sha2-256")
#   string  raw_signature
#
# For Ed25519: raw_signature is 64 bytes
# For RSA: raw_signature is the RSA signature (typically 256-512 bytes)
#
# =============================================================================
# JWT STRUCTURE
# =============================================================================
#
# A JWT consists of three base64url-encoded parts separated by dots:
#
#   header.payload.signature
#
# Header example:
#   {"alg":"SSH","typ":"JWT"}
#
# Payload example:
#   {"iss":"kubectl-ssh-oidc","sub":"username","aud":"https://dex.example.com",
#    "jti":"random-id","exp":1234567890,"iat":1234567590,"nbf":1234567590}
#
# The signature is computed over: base64url(header) + "." + base64url(payload)
#
# For SSH-OIDC, we use "SSH" as the algorithm identifier, and the signature
# is the raw SSH signature base64url-encoded.
#
# =============================================================================

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
    --oidc-audience AUD     OIDC audience (default: server URL)
    --oidc-client-id ID     OIDC client ID (default: dbt-ssh for SSH, dbt for device flow)
    --oidc-client-secret S  OIDC client secret (optional)
    --oidc-username USER    Username for OIDC token (for SSH-OIDC; default: system username)
    --connector-id ID       OIDC connector ID (use 'ssh' for SSH-OIDC token exchange)
    --s3-region REGION      AWS region for S3 URL (default: auto-detect or us-east-1)
    --tools-url URL         Tools repository URL (default: derived from --url)
    --install-dir DIR       Install directory (default: ~/.local/bin)
    --no-modify-profile     Don't modify shell profile
    -y, --yes               Non-interactive mode
    -h, --help              Show this help

Authentication modes:
    S3 URLs (s3://...)      Uses AWS credentials (no --oidc-* needed)
    HTTP with SSH-OIDC      Use --connector-id ssh (requires ssh-agent)
    HTTP with device flow   Default OIDC mode (opens browser)

Quick install (HTTP):
    curl -fsSL https://dbt.example.com/install_dbt.sh | bash -s -- --url https://dbt.example.com

Quick install (S3):
    aws s3 cp s3://your-dbt-bucket/install_dbt.sh - | bash -s -- --url s3://your-dbt-bucket

Quick install (SSH-OIDC):
    curl -fsSL https://dbt.example.com/install_dbt.sh | bash -s -- \\
        --url https://dbt.example.com \\
        --oidc-issuer https://dex.example.com \\
        --connector-id ssh

Examples:
    $0 --url https://dbt.example.com
    $0 --url https://dbt.example.com --oidc-issuer https://dex.example.com
    $0 --url https://dbt.example.com --oidc-issuer https://dex.example.com --connector-id ssh
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
OIDC_USERNAME=""
CONNECTOR_ID=""
S3_REGION=""
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
        --oidc-username) OIDC_USERNAME="$2"; shift 2 ;;
        --connector-id) CONNECTOR_ID="$2"; shift 2 ;;
        --s3-region) S3_REGION="$2"; shift 2 ;;
        --tools-url) TOOLS_URL="$2"; shift 2 ;;
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

# For SSH-OIDC, username is required for the JWT subject claim
# Prompt if not provided via --oidc-username
if [[ "$CONNECTOR_ID" == "ssh" && -z "$OIDC_USERNAME" ]]; then
    # Default to system username
    DEFAULT_USERNAME="${USER:-$(whoami)}"

    if [[ "$NON_INTERACTIVE" == "true" ]]; then
        OIDC_USERNAME="$DEFAULT_USERNAME"
    else
        echo ""
        echo "SSH-OIDC requires a username for authentication."
        echo "This should match your identity in the OIDC provider (Dex)."
        echo ""
        read -p "Enter OIDC username [$DEFAULT_USERNAME]: " OIDC_USERNAME
        OIDC_USERNAME="${OIDC_USERNAME:-$DEFAULT_USERNAME}"
    fi
fi

# OIDC token cache
CACHED_OIDC_TOKEN=""

# =============================================================================
# BASE64URL ENCODING
# =============================================================================
# JWT uses base64url encoding (RFC 4648 Section 5):
# - Standard base64 with '+' -> '-' and '/' -> '_'
# - No padding ('=' characters removed)
# =============================================================================

# Base64url encode from stdin (no padding, URL-safe)
base64url_encode() {
    base64 | tr '+/' '-_' | tr -d '=' | tr -d '\n'
}

# Base64url encode a string directly
base64url_encode_str() {
    printf '%s' "$1" | base64url_encode
}

# =============================================================================
# SSH AGENT COMMUNICATION
# =============================================================================
# The ssh-agent uses a binary protocol over a Unix socket.
# We use netcat (nc -U) to communicate with the socket.
#
# Message format:
#   uint32  message_length (big-endian, excludes this field)
#   byte    message_type
#   ...     message_data
#
# String format (used for keys, data, etc.):
#   uint32  length (big-endian)
#   bytes   data
# =============================================================================

# Check if we can communicate with ssh-agent
# Sets SSH_AGENT_TOOL to "nc" or "socat" on success
# Sets SSH_AGENT_ERROR with reason on failure
check_ssh_agent_available() {
    SSH_AGENT_ERROR=""
    SSH_AGENT_TOOL=""

    if [[ -z "$SSH_AUTH_SOCK" ]]; then
        SSH_AGENT_ERROR="SSH_AUTH_SOCK environment variable not set. Start ssh-agent: eval \$(ssh-agent)"
        return 1
    fi

    if [[ ! -S "$SSH_AUTH_SOCK" ]]; then
        SSH_AGENT_ERROR="SSH_AUTH_SOCK ($SSH_AUTH_SOCK) is not a valid socket"
        return 1
    fi

    # Check if xxd is available - required for hex encoding/decoding
    if ! command -v xxd >/dev/null 2>&1; then
        SSH_AGENT_ERROR="SSH-OIDC requires 'xxd' for binary data handling.

Install xxd:
  Ubuntu/Debian: sudo apt-get install xxd  (or vim-common)
  Fedora/RHEL:   sudo dnf install vim-common
  macOS:         xxd is included with vim (usually pre-installed)
  Alpine:        apk add vim"
        return 1
    fi

    # Check for tools that can communicate with Unix sockets
    # We support: nc (netcat-openbsd), ncat (from nmap), or socat
    #
    # Try in order of preference:
    # 1. nc with -U flag (netcat-openbsd)
    # 2. ncat (from nmap package)
    # 3. socat

    # Try nc -U first (OpenBSD netcat)
    if command -v nc >/dev/null 2>&1; then
        if nc -U -z "$SSH_AUTH_SOCK" </dev/null 2>/dev/null; then
            SSH_AGENT_TOOL="nc"
            return 0
        fi
    fi

    # Try ncat (from nmap)
    if command -v ncat >/dev/null 2>&1; then
        SSH_AGENT_TOOL="ncat"
        return 0
    fi

    # Try socat
    if command -v socat >/dev/null 2>&1; then
        SSH_AGENT_TOOL="socat"
        return 0
    fi

    # No suitable tool found
    SSH_AGENT_ERROR="SSH-OIDC requires a tool to communicate with ssh-agent over Unix sockets.

None of the following tools were found or working:
  - nc (netcat) with Unix socket support (-U flag)
  - ncat (from nmap package)
  - socat

Install one of these:

  Arch Linux:    sudo pacman -S openbsd-netcat  (or: nmap for ncat)
  Ubuntu/Debian: sudo apt-get install netcat-openbsd
  Fedora/RHEL:   sudo dnf install nmap-ncat
  macOS:         brew install netcat  (usually pre-installed)
  Alpine:        apk add netcat-openbsd

Or install socat:
  Arch Linux:    sudo pacman -S socat
  Ubuntu/Debian: sudo apt-get install socat
  Fedora/RHEL:   sudo dnf install socat
  macOS:         brew install socat"
    return 1
}

# Send binary data to ssh-agent and receive response
# Uses the tool detected by check_ssh_agent_available (nc, ncat, or socat)
ssh_agent_communicate() {
    local request_hex="$1"
    local response

    # Convert hex to binary and send to agent
    # Each tool has slightly different syntax for Unix sockets
    case "$SSH_AGENT_TOOL" in
        nc)
            # OpenBSD netcat: nc -U <socket>
            response=$(printf '%s' "$request_hex" | xxd -r -p | nc -U "$SSH_AUTH_SOCK" | xxd -p | tr -d '\n')
            ;;
        ncat)
            # nmap's ncat: ncat -U <socket>
            response=$(printf '%s' "$request_hex" | xxd -r -p | ncat -U "$SSH_AUTH_SOCK" | xxd -p | tr -d '\n')
            ;;
        socat)
            # socat: socat - UNIX-CONNECT:<socket>
            response=$(printf '%s' "$request_hex" | xxd -r -p | socat - "UNIX-CONNECT:$SSH_AUTH_SOCK" | xxd -p | tr -d '\n')
            ;;
        *)
            return 1
            ;;
    esac

    echo "$response"
}

# Pack a 32-bit big-endian integer as hex
pack_uint32() {
    printf '%08x' "$1"
}

# Pack a string (length-prefixed) as hex
pack_string() {
    local data="$1"
    local len=${#data}
    local hex_data
    hex_data=$(printf '%s' "$data" | xxd -p | tr -d '\n')
    printf '%08x%s' "$((len))" "$hex_data"
}

# Pack binary data (already in hex) with length prefix
pack_string_hex() {
    local hex_data="$1"
    local len=$((${#hex_data} / 2))
    printf '%08x%s' "$len" "$hex_data"
}

# Unpack a 32-bit big-endian integer from hex
unpack_uint32() {
    local hex="$1"
    printf '%d' "0x$hex"
}

# =============================================================================
# SSH KEY HANDLING
# =============================================================================
# SSH keys in the agent are identified by their "key blob" - the public key
# in SSH wire format. For signing, we need to provide this blob to the agent.
#
# Key blob format:
#   string  key_type (e.g., "ssh-ed25519", "ssh-rsa")
#   ...     key-type-specific data
#
# For ssh-ed25519:
#   string  "ssh-ed25519"
#   string  public_key (32 bytes)
#
# For ssh-rsa:
#   string  "ssh-rsa"
#   string  e (public exponent)
#   string  n (modulus)
# =============================================================================

# Get list of keys from ssh-agent
# Returns: newline-separated list of "key_blob_hex:key_type:comment"
get_agent_keys() {
    # SSH2_AGENTC_REQUEST_IDENTITIES = 11
    # Message: length(4) + type(1)
    local request="0000000111"  # length=1, type=11

    local response
    response=$(ssh_agent_communicate "$request")
    [[ -z "$response" ]] && return 1

    # Response format:
    # uint32  response_length
    # byte    response_type (SSH2_AGENT_IDENTITIES_ANSWER = 12)
    # uint32  num_keys
    # For each key:
    #   string  key_blob
    #   string  comment

    local resp_len resp_type num_keys
    resp_len=$(unpack_uint32 "${response:0:8}")
    resp_type=$(unpack_uint32 "000000${response:8:2}")

    # Check response type (12 = SSH2_AGENT_IDENTITIES_ANSWER)
    [[ "$resp_type" -ne 12 ]] && return 1

    num_keys=$(unpack_uint32 "${response:10:8}")
    [[ "$num_keys" -eq 0 ]] && return 1

    local pos=18  # Skip: resp_len(8) + resp_type(2) + num_keys(8)
    local keys=""

    for ((i=0; i<num_keys; i++)); do
        # Read key blob
        local blob_len blob_hex
        blob_len=$(unpack_uint32 "${response:$pos:8}")
        pos=$((pos + 8))
        blob_hex="${response:$pos:$((blob_len * 2))}"
        pos=$((pos + blob_len * 2))

        # Read comment
        local comment_len comment_hex comment
        comment_len=$(unpack_uint32 "${response:$pos:8}")
        pos=$((pos + 8))
        comment_hex="${response:$pos:$((comment_len * 2))}"
        comment=$(printf '%s' "$comment_hex" | xxd -r -p 2>/dev/null)
        pos=$((pos + comment_len * 2))

        # Determine key type from blob
        local key_type_len key_type_hex key_type
        key_type_len=$(unpack_uint32 "${blob_hex:0:8}")
        key_type_hex="${blob_hex:8:$((key_type_len * 2))}"
        key_type=$(printf '%s' "$key_type_hex" | xxd -r -p 2>/dev/null)

        keys+="${blob_hex}:${key_type}:${comment}"$'\n'
    done

    echo "$keys"
}

# Sign data with a specific key via ssh-agent
# Args: key_blob_hex, data_to_sign
# Returns: signature_blob_hex (algorithm:raw_signature format inside)
sign_with_agent() {
    local key_blob_hex="$1"
    local data="$2"
    local key_type="$3"

    # SSH2_AGENTC_SIGN_REQUEST = 13
    # Format: type(1) + key_blob(string) + data(string) + flags(4)

    local data_hex
    data_hex=$(printf '%s' "$data" | xxd -p | tr -d '\n')

    # Determine flags based on key type
    # For RSA, we want SHA-256 signatures (flag 2 = SSH_AGENT_RSA_SHA2_256)
    local flags="00000000"
    if [[ "$key_type" == "ssh-rsa" ]]; then
        flags="00000002"  # SSH_AGENT_RSA_SHA2_256
    fi

    local msg_body="0d"  # type = 13
    msg_body+=$(pack_string_hex "$key_blob_hex")  # key blob
    msg_body+=$(pack_string_hex "$data_hex")      # data to sign
    msg_body+="$flags"                            # flags

    local msg_len=$((${#msg_body} / 2))
    local request
    request=$(pack_uint32 "$msg_len")
    request+="$msg_body"

    local response
    response=$(ssh_agent_communicate "$request")
    [[ -z "$response" ]] && return 1

    # Response format:
    # uint32  response_length
    # byte    response_type (SSH2_AGENT_SIGN_RESPONSE = 14)
    # string  signature_blob

    local resp_type
    resp_type=$(unpack_uint32 "000000${response:8:2}")

    # Check response type (14 = SSH2_AGENT_SIGN_RESPONSE)
    if [[ "$resp_type" -ne 14 ]]; then
        # Type 5 = SSH_AGENT_FAILURE
        return 1
    fi

    # Extract signature blob (skip response_len and type)
    local sig_blob_len sig_blob_hex
    sig_blob_len=$(unpack_uint32 "${response:10:8}")
    sig_blob_hex="${response:18:$((sig_blob_len * 2))}"

    echo "$sig_blob_hex"
}

# Extract raw signature from SSH signature blob
# SSH signature blob format:
#   string  algorithm (e.g., "ssh-ed25519", "rsa-sha2-256")
#   string  raw_signature
extract_raw_signature() {
    local sig_blob_hex="$1"
    local pos=0

    # Skip algorithm string
    local algo_len
    algo_len=$(unpack_uint32 "${sig_blob_hex:$pos:8}")
    pos=$((pos + 8 + algo_len * 2))

    # Read raw signature
    local raw_sig_len raw_sig_hex
    raw_sig_len=$(unpack_uint32 "${sig_blob_hex:$pos:8}")
    pos=$((pos + 8))
    raw_sig_hex="${sig_blob_hex:$pos:$((raw_sig_len * 2))}"

    echo "$raw_sig_hex"
}

# =============================================================================
# JWT CREATION WITH SSH SIGNING
# =============================================================================
# Creates a JWT signed with an SSH key from the agent.
#
# The JWT has:
#   Header: {"alg":"SSH","typ":"JWT"}
#   Payload: Standard claims (iss, sub, aud, exp, iat, nbf, jti)
#   Signature: SSH signature over header.payload, base64url-encoded
#
# We try each key in the agent until Dex accepts one (standard SSH behavior).
# =============================================================================

# Create SSH-signed JWT using ssh-agent
# Follows SSH behavior: tries each key until one works with Dex
create_ssh_signed_jwt() {
    local audience="$1"
    # Use OIDC_USERNAME if set, otherwise fall back to system username
    local username="${OIDC_USERNAME:-${USER:-$(whoami)}}"

    # Create JWT header and payload
    # Using "SSH" as the algorithm identifier (matches kubectl-ssh-oidc)
    local header payload signing_input
    header=$(base64url_encode_str '{"alg":"SSH","typ":"JWT"}')

    # Generate timestamps and unique ID
    local now exp jti
    now=$(date +%s)
    exp=$((now + 300))  # Token valid for 5 minutes

    # Generate random JWT ID (jti) for uniqueness
    if [[ -f /dev/urandom ]]; then
        jti=$(head -c 16 /dev/urandom | xxd -p | tr -d '\n')
    else
        jti=$(date +%s%N)$(printf '%04x' $$)
    fi

    # Build payload JSON
    # Claims match what jwt-ssh-agent-go produces:
    # - iss: Issuer (we use "kubectl-ssh-oidc" for compatibility)
    # - sub: Subject (username)
    # - aud: Audience (Dex instance URL)
    # - jti: JWT ID (unique identifier)
    # - exp: Expiration time
    # - iat: Issued at
    # - nbf: Not before
    local payload_json="{\"iss\":\"kubectl-ssh-oidc\",\"sub\":\"${username}\",\"aud\":\"${audience}\",\"jti\":\"${jti}\",\"exp\":${exp},\"iat\":${now},\"nbf\":${now}}"
    payload=$(base64url_encode_str "$payload_json")

    # The signing input is: base64url(header) + "." + base64url(payload)
    signing_input="${header}.${payload}"

    # Get all keys from ssh-agent
    local keys
    keys=$(get_agent_keys)
    [[ -z "$keys" ]] && { echo "No SSH keys found in agent" >&2; return 1; }

    # Try each key until one produces a valid signature
    local key_entry
    while IFS= read -r key_entry; do
        [[ -z "$key_entry" ]] && continue

        local key_blob_hex key_type comment
        key_blob_hex="${key_entry%%:*}"
        key_entry="${key_entry#*:}"
        key_type="${key_entry%%:*}"
        comment="${key_entry#*:}"

        # Only support RSA and Ed25519 keys
        case "$key_type" in
            ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp256|ecdsa-sha2-nistp384|ecdsa-sha2-nistp521)
                ;;
            *)
                continue
                ;;
        esac

        # Sign the JWT signing input with this key
        local sig_blob_hex
        sig_blob_hex=$(sign_with_agent "$key_blob_hex" "$signing_input" "$key_type")
        [[ -z "$sig_blob_hex" ]] && continue

        # Extract raw signature from SSH signature blob
        local raw_sig_hex
        raw_sig_hex=$(extract_raw_signature "$sig_blob_hex")
        [[ -z "$raw_sig_hex" ]] && continue

        # Convert raw signature to base64url
        local signature
        signature=$(printf '%s' "$raw_sig_hex" | xxd -r -p | base64url_encode)

        # Build complete JWT
        echo "${signing_input}.${signature}"
        return 0

    done <<< "$keys"

    echo "Failed to sign JWT with any available SSH key" >&2
    return 1
}

# =============================================================================
# RFC 8693 TOKEN EXCHANGE WITH DEX
# =============================================================================
# After creating an SSH-signed JWT, we exchange it with Dex for an OIDC token.
#
# The token exchange request includes:
# - grant_type: urn:ietf:params:oauth:grant-type:token-exchange
# - subject_token_type: urn:ietf:params:oauth:token-type:access_token
# - subject_token: The SSH-signed JWT
# - requested_token_type: urn:ietf:params:oauth:token-type:id_token
# - connector_id: "ssh" (routes to Dex's SSH connector)
# - client_id: Application client ID
# - scope: "openid email groups profile"
#
# Dex's SSH connector:
# 1. Parses the JWT
# 2. Extracts the subject (username)
# 3. Looks up registered public keys for that user
# 4. Verifies the JWT signature against each registered key
# 5. If valid, issues an OIDC token with user's groups/claims
# =============================================================================

# Exchange SSH-signed JWT for OIDC token with Dex
exchange_ssh_jwt_for_oidc() {
    local ssh_jwt="$1"
    local token_endpoint="$2"
    local client_id="$3"
    local client_secret="$4"
    local connector_id="$5"
    local audience="$6"

    # Build URL-encoded form data for RFC 8693 token exchange
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

    # Check for error response
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

# =============================================================================
# SSH-OIDC AUTHENTICATION FLOW
# =============================================================================
# Complete flow for SSH-OIDC authentication:
# 1. Verify ssh-agent is available with keys
# 2. Discover OIDC endpoints from issuer
# 3. Create SSH-signed JWT
# 4. Exchange JWT with Dex for OIDC token
# =============================================================================

# Get OIDC token via SSH-OIDC token exchange
get_ssh_oidc_token() {
    if [[ -n "$CACHED_OIDC_TOKEN" ]]; then
        echo "$CACHED_OIDC_TOKEN"
        return 0
    fi

    [[ -z "$OIDC_ISSUER" ]] && return 0

    step "Authenticating via SSH-OIDC token exchange..."

    # Verify ssh-agent is available and accessible
    if ! check_ssh_agent_available; then
        echo ""
        echo -e "${RED}SSH-OIDC Error:${NC} $SSH_AGENT_ERROR" >&2
        echo ""
        return 1
    fi

    # Check for keys in agent
    local keys
    keys=$(get_agent_keys 2>/dev/null)
    if [[ -z "$keys" ]]; then
        error "No SSH keys in agent. Add your key: ssh-add"
    fi

    # Discover OIDC endpoints from issuer
    local discovery_url="${OIDC_ISSUER}/.well-known/openid-configuration"
    local discovery_json
    discovery_json=$(curl -sL "$discovery_url") || error "Failed to fetch OIDC discovery from $discovery_url"

    local token_endpoint
    token_endpoint=$(echo "$discovery_json" | jq -r '.token_endpoint')
    [[ -z "$token_endpoint" || "$token_endpoint" == "null" ]] && error "No token endpoint in OIDC discovery"

    # Create SSH-signed JWT
    # The JWT audience should be the OIDC issuer URL (Dex instance)
    local ssh_jwt
    ssh_jwt=$(create_ssh_signed_jwt "$OIDC_ISSUER")
    if [[ -z "$ssh_jwt" ]]; then
        error "Failed to create SSH-signed JWT"
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

# =============================================================================
# OIDC DEVICE CODE FLOW
# =============================================================================
# Alternative authentication method using browser-based login.
# Useful when SSH keys are not available or for interactive use.
#
# Flow:
# 1. Request device code from OIDC provider
# 2. Display verification URL and user code
# 3. User opens URL in browser and enters code
# 4. Poll token endpoint until user completes auth
# 5. Receive OIDC token
# =============================================================================

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

    # Try to open browser automatically
    if command -v xdg-open >/dev/null 2>&1; then
        xdg-open "$verification_uri" 2>/dev/null || true
    elif command -v open >/dev/null 2>&1; then
        open "$verification_uri" 2>/dev/null || true
    fi

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
        # Try SSH-OIDC first
        if check_ssh_agent_available; then
            get_ssh_oidc_token
        else
            # SSH-OIDC requested but agent communication not available
            # Show detailed error and exit - don't fall back silently
            echo ""
            echo -e "${RED}SSH-OIDC Error:${NC}" >&2
            echo "$SSH_AGENT_ERROR" >&2
            echo ""
            error "Cannot proceed with SSH-OIDC authentication. Fix the above issue or use device flow (remove --connector-id ssh)."
        fi
    else
        get_device_flow_token
    fi
}

# =============================================================================
# FILE DOWNLOAD HELPERS
# =============================================================================

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

# =============================================================================
# INSTALLATION LOGIC
# =============================================================================

# Derive server name from URL if not provided
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

# Download dbt
step "Downloading dbt..."
DBT_URL="${SERVER_URL}/${VERSION}/${OS}/${ARCH}/dbt"
CHECKSUM_URL="${DBT_URL}.sha256"

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

if ! fetch "$DBT_URL" "$TEMP_DIR/dbt" 2>/dev/null; then
    error "Failed to download from $DBT_URL"
fi

# Verify checksum
if fetch "$CHECKSUM_URL" "$TEMP_DIR/dbt.sha256" 2>/dev/null; then
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
        [[ -n "$OIDC_USERNAME" ]] && j+=",\"oidcUsername\":\"${OIDC_USERNAME}\""
        [[ -n "$CONNECTOR_ID" ]] && j+=",\"connectorId\":\"${CONNECTOR_ID}\""
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
