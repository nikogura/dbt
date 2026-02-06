#!/bin/bash
# Publish dbt artifacts from GitHub releases to downstream repositories (S3, HTTP, etc.)
#
# This script:
#   1. Fetches the latest (or specified) release from GitHub
#   2. Downloads release artifacts
#   3. Signs artifacts with local GPG key
#   4. Generates checksums (md5, sha1, sha256)
#   5. Uploads to configured destination (S3 or HTTP PUT)
#
# No git clone needed - works entirely from GitHub release artifacts.
#
# USAGE:
#   Copy this script to your downstream repo and customize the configuration
#   section below with your destination settings.

set -e

# =============================================================================
# CONFIGURATION - Customize these for your downstream
# =============================================================================
GITHUB_REPO="nikogura/dbt"
GPG_IDENTITY="${GPG_IDENTITY:-}"                            # Leave empty to use default GPG key

# S3 Configuration (used when UPLOAD_METHOD=s3)
DBT_S3_BUCKET="${DBT_S3_BUCKET:-your-dbt-bucket}"           # CHANGEME: Your S3 bucket for dbt binaries
TOOLS_S3_BUCKET="${TOOLS_S3_BUCKET:-your-dbt-tools-bucket}" # CHANGEME: Your S3 bucket for tools
S3_REGION="${S3_REGION:-us-east-1}"                         # CHANGEME: Your AWS region

# HTTP Configuration (used when UPLOAD_METHOD=http)
REPOSERVER_URL="${REPOSERVER_URL:-}"                        # e.g., https://dbt.example.com
REPOSERVER_AUTH="${REPOSERVER_AUTH:-none}"                  # none, basic, bearer, oidc
REPOSERVER_USER="${REPOSERVER_USER:-}"                      # For basic auth
REPOSERVER_PASS="${REPOSERVER_PASS:-}"                      # For basic auth
REPOSERVER_TOKEN="${REPOSERVER_TOKEN:-}"                    # For bearer auth
OIDC_ISSUER="${OIDC_ISSUER:-}"                              # For OIDC auth
OIDC_AUDIENCE="${OIDC_AUDIENCE:-dbt-server}"                # For OIDC auth
OIDC_CONNECTOR="${OIDC_CONNECTOR:-local}"                   # For OIDC auth
OIDC_USER="${OIDC_USER:-}"                                  # For OIDC auth (if using password grant)
OIDC_PASS="${OIDC_PASS:-}"                                  # For OIDC auth (if using password grant)

# Upload method: http or s3
UPLOAD_METHOD="${UPLOAD_METHOD:-http}"

# Token cache (populated on first OIDC auth)
CACHED_OIDC_TOKEN=""

# Artifacts to publish (can be overridden with --include/--exclude)
DEFAULT_ARTIFACTS=(
    "dbt:dbt_darwin_amd64:darwin/amd64/dbt"
    "dbt:dbt_darwin_arm64:darwin/arm64/dbt"
    "dbt:dbt_linux_amd64:linux/amd64/dbt"
    "catalog:catalog_darwin_amd64:darwin/amd64/catalog"
    "catalog:catalog_darwin_arm64:darwin/arm64/catalog"
    "catalog:catalog_linux_amd64:linux/amd64/catalog"
)

# Installer scripts to publish (from local scripts/ directory)
INSTALLER_SCRIPTS=(
    "install_dbt.sh"
    "install_dbt_mac_keychain.sh"
)

# Publish installer scripts by default
PUBLISH_INSTALLERS=true

# =============================================================================
# END CONFIGURATION
# =============================================================================

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }
debug() { [[ "$VERBOSE" == "true" ]] && echo -e "${BLUE}[DEBUG]${NC} $1" || true; }

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Download dbt release artifacts from GitHub, sign, and publish to S3 or HTTP reposerver.

Options:
    -v, --version VERSION   Version to publish (default: latest release)
    -d, --dry-run           Show what would be done without uploading
    -y, --yes               Skip confirmation prompts
    --include PATTERN       Only include artifacts matching pattern (can repeat)
    --exclude PATTERN       Exclude artifacts matching pattern (can repeat)
    --no-installers         Don't publish installer scripts
    --installers-only       Only publish installer scripts (skip binaries)
    --list-artifacts        List available artifacts and exit
    --verbose               Verbose output
    -h, --help              Show this help message

    Upload method (choose one):
    --http URL              Upload via HTTP PUT to reposerver (default)
    --s3                    Upload to S3 buckets
    --auth TYPE             Auth type for HTTP: none, basic, bearer, oidc (default: none)
    --token TOKEN           Bearer token for --auth bearer
    --oidc-issuer URL       OIDC issuer URL (for --auth oidc)
    --oidc-audience AUD     OIDC audience (default: dbt-server)
    --oidc-connector ID     OIDC connector ID (default: local)

Patterns for --include/--exclude:
    dbt         All dbt binaries
    catalog     All catalog binaries
    darwin      All Darwin/macOS artifacts
    linux       All Linux artifacts
    amd64       All amd64 artifacts
    arm64       All arm64 artifacts

Environment variables:
    UPLOAD_METHOD       Upload method: http or s3 (default: http)
    GPG_IDENTITY        GPG key identity for signing (default: use default key)
    GITHUB_TOKEN        GitHub token for API access (optional, for rate limits)

    S3 (when UPLOAD_METHOD=s3):
    DBT_S3_BUCKET       S3 bucket for dbt binaries
    TOOLS_S3_BUCKET     S3 bucket for tools
    S3_REGION           AWS region

    HTTP (when UPLOAD_METHOD=http):
    REPOSERVER_URL      Base URL for reposerver (e.g., https://dbt.example.com)
    REPOSERVER_AUTH     Auth type: none, basic, bearer, oidc
    REPOSERVER_USER     Username for basic auth
    REPOSERVER_PASS     Password for basic auth
    REPOSERVER_TOKEN    Token for bearer auth
    OIDC_ISSUER         OIDC issuer URL (e.g., https://dex.example.com)
    OIDC_AUDIENCE       OIDC audience (default: dbt-server)
    OIDC_CONNECTOR      OIDC connector ID (default: local)

Examples:
    # HTTP upload with OIDC (default method)
    $0 --http https://dbt.example.com --auth oidc --oidc-issuer https://dex.example.com
    $0 --http https://dbt.example.com --auth oidc --oidc-issuer https://dex.example.com -v 3.7.5

    # HTTP upload with bearer token
    $0 --http https://dbt.example.com --auth bearer --token "\$MY_TOKEN"

    # S3 upload
    $0 --s3                         # Publish latest release to S3
    $0 --s3 -v 3.7.5                # Publish specific version to S3

    # Filter artifacts
    $0 --include dbt                # Only dbt binaries
    $0 --include linux              # Only Linux artifacts
EOF
}

list_artifacts() {
    echo "Available artifacts:"
    echo ""
    printf "%-12s %-35s %s\n" "TYPE" "SOURCE" "DESTINATION"
    printf "%-12s %-35s %s\n" "----" "------" "-----------"
    for artifact in "${DEFAULT_ARTIFACTS[@]}"; do
        IFS=':' read -r type source dest <<< "$artifact"
        printf "%-12s %-35s %s\n" "$type" "$source" "$dest"
    done
}

# Parse arguments
VERSION=""
DRY_RUN=false
SKIP_CONFIRM=false
VERBOSE=false
INCLUDE_PATTERNS=()
EXCLUDE_PATTERNS=()
INSTALLERS_ONLY=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -y|--yes)
            SKIP_CONFIRM=true
            shift
            ;;
        --include)
            INCLUDE_PATTERNS+=("$2")
            shift 2
            ;;
        --exclude)
            EXCLUDE_PATTERNS+=("$2")
            shift 2
            ;;
        --list-artifacts)
            list_artifacts
            exit 0
            ;;
        --no-installers)
            PUBLISH_INSTALLERS=false
            shift
            ;;
        --installers-only)
            INSTALLERS_ONLY=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --s3)
            UPLOAD_METHOD="s3"
            shift
            ;;
        --http)
            UPLOAD_METHOD="http"
            REPOSERVER_URL="$2"
            shift 2
            ;;
        --auth)
            REPOSERVER_AUTH="$2"
            shift 2
            ;;
        --token)
            REPOSERVER_TOKEN="$2"
            shift 2
            ;;
        --oidc-issuer)
            OIDC_ISSUER="$2"
            shift 2
            ;;
        --oidc-audience)
            OIDC_AUDIENCE="$2"
            shift 2
            ;;
        --oidc-connector)
            OIDC_CONNECTOR="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

# Verify prerequisites
command -v curl >/dev/null 2>&1 || error "curl not found"
command -v jq >/dev/null 2>&1 || error "jq not found"
command -v gpg >/dev/null 2>&1 || error "gpg not found"

if [[ "$UPLOAD_METHOD" == "s3" ]]; then
    command -v aws >/dev/null 2>&1 || error "AWS CLI not found (required for S3 upload)"
fi

# Validate HTTP configuration
if [[ "$UPLOAD_METHOD" == "http" ]]; then
    [[ -z "$REPOSERVER_URL" ]] && error "REPOSERVER_URL required for HTTP upload (use --http URL)"

    case "$REPOSERVER_AUTH" in
        none)
            ;;
        basic)
            [[ -z "$REPOSERVER_USER" ]] && error "REPOSERVER_USER required for basic auth"
            [[ -z "$REPOSERVER_PASS" ]] && error "REPOSERVER_PASS required for basic auth"
            ;;
        bearer)
            [[ -z "$REPOSERVER_TOKEN" ]] && error "REPOSERVER_TOKEN required for bearer auth"
            ;;
        oidc)
            [[ -z "$OIDC_ISSUER" ]] && error "OIDC_ISSUER required for OIDC auth (use --oidc-issuer URL)"
            ;;
        *)
            error "Unknown auth type: $REPOSERVER_AUTH (use: none, basic, bearer, oidc)"
            ;;
    esac
fi

# Create temp directory
WORK_DIR=$(mktemp -d)
trap "rm -rf $WORK_DIR" EXIT
debug "Working directory: $WORK_DIR"

# Get release info from GitHub
get_release_info() {
    local api_url="https://api.github.com/repos/$GITHUB_REPO/releases"
    local auth_header=""

    if [[ -n "$GITHUB_TOKEN" ]]; then
        auth_header="-H \"Authorization: token $GITHUB_TOKEN\""
    fi

    if [[ -z "$VERSION" ]]; then
        info "Fetching latest release from GitHub..."
        RELEASE_JSON=$(curl -sL $auth_header "$api_url/latest")
    else
        info "Fetching release $VERSION from GitHub..."
        RELEASE_JSON=$(curl -sL $auth_header "$api_url/tags/$VERSION")
    fi

    if echo "$RELEASE_JSON" | jq -e '.message' >/dev/null 2>&1; then
        error "GitHub API error: $(echo "$RELEASE_JSON" | jq -r '.message')"
    fi

    VERSION=$(echo "$RELEASE_JSON" | jq -r '.tag_name')
    RELEASE_URL=$(echo "$RELEASE_JSON" | jq -r '.html_url')

    if [[ -z "$VERSION" || "$VERSION" == "null" ]]; then
        error "Could not determine version from release"
    fi

    info "Release: $VERSION"
    debug "Release URL: $RELEASE_URL"
}

# Check if artifact matches include/exclude patterns
should_include_artifact() {
    local artifact="$1"
    IFS=':' read -r type source dest <<< "$artifact"

    # If include patterns specified, artifact must match at least one
    if [[ ${#INCLUDE_PATTERNS[@]} -gt 0 ]]; then
        local matched=false
        for pattern in "${INCLUDE_PATTERNS[@]}"; do
            if [[ "$type" == *"$pattern"* ]] || [[ "$source" == *"$pattern"* ]] || [[ "$dest" == *"$pattern"* ]]; then
                matched=true
                break
            fi
        done
        if [[ "$matched" == "false" ]]; then
            return 1
        fi
    fi

    # Check exclude patterns
    for pattern in "${EXCLUDE_PATTERNS[@]}"; do
        if [[ "$type" == *"$pattern"* ]] || [[ "$source" == *"$pattern"* ]] || [[ "$dest" == *"$pattern"* ]]; then
            return 1
        fi
    done

    return 0
}

# Download artifact from GitHub release
download_artifact() {
    local source="$1"
    local dest_path="$2"

    # Find asset URL
    local asset_url=$(echo "$RELEASE_JSON" | jq -r ".assets[] | select(.name == \"$source\") | .browser_download_url")

    if [[ -z "$asset_url" || "$asset_url" == "null" ]]; then
        warn "Asset not found in release: $source"
        return 1
    fi

    debug "Downloading $source from $asset_url"
    mkdir -p "$(dirname "$dest_path")"

    if ! curl -sL -o "$dest_path" "$asset_url"; then
        warn "Failed to download: $source"
        return 1
    fi

    return 0
}

# Sign file with GPG
sign_file() {
    local file="$1"
    local sig_file="${file}.asc"

    debug "Signing $file"

    local gpg_args=("--armor" "--detach-sign" "--output" "$sig_file")
    if [[ -n "$GPG_IDENTITY" ]]; then
        gpg_args+=("--local-user" "$GPG_IDENTITY")
    fi
    gpg_args+=("$file")

    if ! gpg "${gpg_args[@]}" 2>/dev/null; then
        error "Failed to sign: $file"
    fi
}

# Generate checksums
generate_checksums() {
    local file="$1"
    local base_name=$(basename "$file")
    local dir_name=$(dirname "$file")

    debug "Generating checksums for $file"

    # Use printf to avoid trailing newline - dbt compares checksums byte-for-byte
    (cd "$dir_name" && printf '%s' "$(md5sum "$base_name" | cut -d' ' -f1)" > "${base_name}.md5")
    (cd "$dir_name" && printf '%s' "$(sha1sum "$base_name" | cut -d' ' -f1)" > "${base_name}.sha1")
    (cd "$dir_name" && printf '%s' "$(sha256sum "$base_name" | cut -d' ' -f1)" > "${base_name}.sha256")
}

# Upload to S3
upload_to_s3() {
    local local_file="$1"
    local s3_path="$2"

    if [[ "$DRY_RUN" == "true" ]]; then
        info "[DRY-RUN] Would upload: $local_file -> $s3_path"
        return 0
    fi

    debug "Uploading $local_file to $s3_path"

    if ! aws s3 cp "$local_file" "$s3_path" --region "$S3_REGION" >/dev/null; then
        error "Failed to upload: $local_file"
    fi
}

# Get OIDC token via device code flow
# This is the most user-friendly flow for CLI tools - opens browser for auth
get_oidc_token() {
    if [[ -n "$CACHED_OIDC_TOKEN" ]]; then
        echo "$CACHED_OIDC_TOKEN"
        return 0
    fi

    info "Authenticating via OIDC device code flow..."

    # Discover OIDC endpoints
    local discovery_url="${OIDC_ISSUER}/.well-known/openid-configuration"
    local discovery_json
    discovery_json=$(curl -sL "$discovery_url") || error "Failed to fetch OIDC discovery from $discovery_url"

    local device_endpoint
    device_endpoint=$(echo "$discovery_json" | jq -r '.device_authorization_endpoint // empty')
    local token_endpoint
    token_endpoint=$(echo "$discovery_json" | jq -r '.token_endpoint')

    if [[ -z "$device_endpoint" ]]; then
        # Fallback: try password grant if we have credentials
        if [[ -n "$OIDC_USER" && -n "$OIDC_PASS" ]]; then
            debug "Device endpoint not available, using password grant"
            local token_response
            token_response=$(curl -sL -X POST "$token_endpoint" \
                -H "Content-Type: application/x-www-form-urlencoded" \
                -d "grant_type=password" \
                -d "client_id=dbt" \
                -d "username=$OIDC_USER" \
                -d "password=$OIDC_PASS" \
                -d "scope=openid profile email groups" \
                -d "audience=$OIDC_AUDIENCE")

            CACHED_OIDC_TOKEN=$(echo "$token_response" | jq -r '.access_token // .id_token // empty')
            if [[ -z "$CACHED_OIDC_TOKEN" ]]; then
                error "OIDC password grant failed: $(echo "$token_response" | jq -r '.error_description // .error // "unknown error"')"
            fi
            echo "$CACHED_OIDC_TOKEN"
            return 0
        fi
        error "OIDC issuer does not support device code flow and no password credentials provided"
    fi

    # Request device code
    local device_response
    device_response=$(curl -sL -X POST "$device_endpoint" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "client_id=dbt" \
        -d "scope=openid profile email groups offline_access" \
        -d "audience=$OIDC_AUDIENCE")

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
            -d "client_id=dbt")

        local error_code
        error_code=$(echo "$token_response" | jq -r '.error // empty')

        case "$error_code" in
            "")
                # Success - got token
                CACHED_OIDC_TOKEN=$(echo "$token_response" | jq -r '.access_token // .id_token')
                if [[ -n "$CACHED_OIDC_TOKEN" && "$CACHED_OIDC_TOKEN" != "null" ]]; then
                    info "Authentication successful!"
                    echo "$CACHED_OIDC_TOKEN"
                    return 0
                fi
                ;;
            authorization_pending|slow_down)
                # Still waiting - continue polling
                debug "Waiting for user authorization..."
                ;;
            *)
                error "OIDC authentication failed: $(echo "$token_response" | jq -r '.error_description // .error')"
                ;;
        esac
    done

    error "OIDC authentication timed out"
}

# Upload via HTTP PUT
upload_to_http() {
    local local_file="$1"
    local http_path="$2"

    local url="${REPOSERVER_URL}${http_path}"

    if [[ "$DRY_RUN" == "true" ]]; then
        info "[DRY-RUN] Would upload: $local_file -> $url"
        return 0
    fi

    debug "Uploading $local_file to $url"

    # Build curl args
    local curl_args=("-sL" "-X" "PUT" "--fail-with-body" "-w" "%{http_code}")

    # Add auth headers
    case "$REPOSERVER_AUTH" in
        basic)
            curl_args+=("-u" "${REPOSERVER_USER}:${REPOSERVER_PASS}")
            ;;
        bearer)
            curl_args+=("-H" "Authorization: Bearer ${REPOSERVER_TOKEN}")
            ;;
        oidc)
            local token
            token=$(get_oidc_token)
            curl_args+=("-H" "Authorization: Bearer ${token}")
            ;;
    esac

    # Add checksum headers if this is the main file (not .asc, .md5, etc.)
    local base_file="${local_file%.asc}"
    base_file="${base_file%.md5}"
    base_file="${base_file%.sha1}"
    base_file="${base_file%.sha256}"

    if [[ "$local_file" == "$base_file" ]]; then
        # This is the main binary - add checksum headers
        if [[ -f "${local_file}.md5" ]]; then
            curl_args+=("-H" "X-Checksum-Md5: $(cat "${local_file}.md5")")
        fi
        if [[ -f "${local_file}.sha1" ]]; then
            curl_args+=("-H" "X-Checksum-Sha1: $(cat "${local_file}.sha1")")
        fi
        if [[ -f "${local_file}.sha256" ]]; then
            curl_args+=("-H" "X-Checksum-Sha256: $(cat "${local_file}.sha256")")
        fi
    fi

    # Upload file
    curl_args+=("--data-binary" "@${local_file}" "$url")

    local response http_code
    response=$(curl "${curl_args[@]}" 2>&1)
    http_code="${response: -3}"
    response="${response:0:-3}"

    if [[ "$http_code" -lt 200 || "$http_code" -ge 300 ]]; then
        warn "HTTP $http_code uploading $local_file: $response"
        return 1
    fi

    debug "HTTP $http_code: $local_file uploaded successfully"
    return 0
}

# Process single artifact
process_artifact() {
    local artifact="$1"
    IFS=':' read -r type source dest <<< "$artifact"

    local local_file="$WORK_DIR/$source"

    # Download
    if ! download_artifact "$source" "$local_file"; then
        return 1
    fi

    # Sign
    sign_file "$local_file"

    # Generate checksums
    generate_checksums "$local_file"

    # Determine destination path
    local dest_path
    case "$type" in
        dbt)
            dest_path="/$VERSION/$dest"
            ;;
        catalog)
            dest_path="/dbt-tools/catalog/$VERSION/$dest"
            ;;
        *)
            error "Unknown artifact type: $type"
            ;;
    esac

    # Upload based on method
    if [[ "$UPLOAD_METHOD" == "http" ]]; then
        # HTTP PUT upload
        upload_to_http "$local_file" "/dbt${dest_path}"
        upload_to_http "${local_file}.asc" "/dbt${dest_path}.asc"
        upload_to_http "${local_file}.md5" "/dbt${dest_path}.md5"
        upload_to_http "${local_file}.sha1" "/dbt${dest_path}.sha1"
        upload_to_http "${local_file}.sha256" "/dbt${dest_path}.sha256"
    else
        # S3 upload
        local s3_bucket s3_path
        case "$type" in
            dbt)
                s3_bucket="$DBT_S3_BUCKET"
                s3_path="s3://$s3_bucket/$VERSION/$dest"
                ;;
            catalog)
                s3_bucket="$TOOLS_S3_BUCKET"
                s3_path="s3://$s3_bucket/catalog/$VERSION/$dest"
                ;;
        esac

        upload_to_s3 "$local_file" "$s3_path"
        upload_to_s3 "${local_file}.asc" "${s3_path}.asc"
        upload_to_s3 "${local_file}.md5" "${s3_path}.md5"
        upload_to_s3 "${local_file}.sha1" "${s3_path}.sha1"
        upload_to_s3 "${local_file}.sha256" "${s3_path}.sha256"
    fi

    return 0
}

# Publish installer scripts
publish_installer_scripts() {
    # Find scripts directory relative to this script
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

    info "Publishing installer scripts..."

    for script_name in "${INSTALLER_SCRIPTS[@]}"; do
        local script_path="$script_dir/$script_name"

        if [[ ! -f "$script_path" ]]; then
            warn "Installer script not found: $script_path"
            continue
        fi

        info "Processing: $script_name"

        # Copy to work directory
        local work_file="$WORK_DIR/$script_name"
        cp "$script_path" "$work_file"
        chmod 755 "$work_file"

        # Sign
        sign_file "$work_file"

        # Generate checksums
        generate_checksums "$work_file"

        # Upload based on method
        if [[ "$UPLOAD_METHOD" == "http" ]]; then
            upload_to_http "$work_file" "/dbt/$script_name"
            upload_to_http "${work_file}.asc" "/dbt/${script_name}.asc"
            upload_to_http "${work_file}.md5" "/dbt/${script_name}.md5"
            upload_to_http "${work_file}.sha1" "/dbt/${script_name}.sha1"
            upload_to_http "${work_file}.sha256" "/dbt/${script_name}.sha256"
        else
            upload_to_s3 "$work_file" "s3://$DBT_S3_BUCKET/$script_name"
            upload_to_s3 "${work_file}.asc" "s3://$DBT_S3_BUCKET/${script_name}.asc"
            upload_to_s3 "${work_file}.md5" "s3://$DBT_S3_BUCKET/${script_name}.md5"
            upload_to_s3 "${work_file}.sha1" "s3://$DBT_S3_BUCKET/${script_name}.sha1"
            upload_to_s3 "${work_file}.sha256" "s3://$DBT_S3_BUCKET/${script_name}.sha256"
        fi
    done
}

# Main
get_release_info

# Filter artifacts (skip if installers-only)
ARTIFACTS=()
if [[ "$INSTALLERS_ONLY" != "true" ]]; then
    for artifact in "${DEFAULT_ARTIFACTS[@]}"; do
        if should_include_artifact "$artifact"; then
            ARTIFACTS+=("$artifact")
        fi
    done
fi

# Check we have something to do
if [[ ${#ARTIFACTS[@]} -eq 0 && "$PUBLISH_INSTALLERS" != "true" ]]; then
    error "No artifacts selected and installer publishing disabled"
fi

# Show what will be published
echo ""
if [[ ${#ARTIFACTS[@]} -gt 0 ]]; then
    info "Will publish ${#ARTIFACTS[@]} artifacts for version $VERSION:"
    for artifact in "${ARTIFACTS[@]}"; do
        IFS=':' read -r type source dest <<< "$artifact"
        echo "  - $source -> $dest"
    done
fi

if [[ "$PUBLISH_INSTALLERS" == "true" ]]; then
    info "Will publish installer scripts:"
    for script in "${INSTALLER_SCRIPTS[@]}"; do
        echo "  - $script"
    done
fi
echo ""

if [[ "$UPLOAD_METHOD" == "http" ]]; then
    info "Upload method: HTTP PUT"
    info "Target: $REPOSERVER_URL"
    info "Auth: $REPOSERVER_AUTH"
else
    info "Upload method: S3"
    info "Target buckets:"
    info "  - s3://$DBT_S3_BUCKET (dbt binaries)"
    info "  - s3://$TOOLS_S3_BUCKET (tools)"
fi

if [[ "$DRY_RUN" == "true" ]]; then
    info "(DRY RUN - no actual uploads)"
fi

# Confirm
if [[ "$SKIP_CONFIRM" != "true" && "$DRY_RUN" != "true" ]]; then
    echo ""
    read -p "Proceed? [y/N] " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        info "Aborted"
        exit 0
    fi
fi

# Process artifacts
echo ""
SUCCESS=0
FAILED=0

if [[ ${#ARTIFACTS[@]} -gt 0 ]]; then
    for artifact in "${ARTIFACTS[@]}"; do
        IFS=':' read -r type source dest <<< "$artifact"
        info "Processing: $source"
        if process_artifact "$artifact"; then
            SUCCESS=$((SUCCESS + 1))
        else
            FAILED=$((FAILED + 1))
        fi
    done
fi

# Publish installer scripts
if [[ "$PUBLISH_INSTALLERS" == "true" ]]; then
    echo ""
    publish_installer_scripts
fi

echo ""
info "=== Publish complete ==="
info "Artifacts: $SUCCESS success, $FAILED failed"
[[ "$PUBLISH_INSTALLERS" == "true" ]] && info "Installer scripts: published"
