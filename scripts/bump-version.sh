#!/bin/bash
# Bump dbt version
#
# Usage:
#   ./scripts/bump-version.sh           # Bump patch version (3.7.0 -> 3.7.1)
#   ./scripts/bump-version.sh 4.0.0     # Set specific version
#   ./scripts/bump-version.sh minor     # Bump minor version (3.7.0 -> 3.8.0)
#   ./scripts/bump-version.sh major     # Bump major version (3.7.0 -> 4.0.0)

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Get current version from metadata.json
get_current_version() {
    grep -o '"version": *"[^"]*"' "$PROJECT_ROOT/metadata.json" | grep -o '[0-9]\+\.[0-9]\+\.[0-9]\+'
}

# Parse semver into components
parse_semver() {
    local version="$1"
    echo "$version" | sed 's/\./ /g'
}

# Bump version based on type
bump_version() {
    local current="$1"
    local bump_type="$2"

    read -r major minor patch <<< "$(parse_semver "$current")"

    case "$bump_type" in
        major)
            echo "$((major + 1)).0.0"
            ;;
        minor)
            echo "${major}.$((minor + 1)).0"
            ;;
        patch|"")
            echo "${major}.${minor}.$((patch + 1))"
            ;;
        *)
            # Assume it's a full version string
            if [[ "$bump_type" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                echo "$bump_type"
            else
                error "Invalid version or bump type: $bump_type"
            fi
            ;;
    esac
}

# Get the "latest" version from test fixtures (the one we're replacing)
get_latest_test_version() {
    # Look at dbt_setup_test.go for latestVersion
    grep 'latestVersion.*=' "$PROJECT_ROOT/pkg/dbt/dbt_setup_test.go" | grep -o '[0-9]\+\.[0-9]\+\.[0-9]\+'
}

# Update version in a file using sed
update_file() {
    local file="$1"
    local pattern="$2"
    local replacement="$3"

    if [[ ! -f "$file" ]]; then
        warn "File not found: $file"
        return
    fi

    # Use different sed syntax for macOS vs Linux
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "$pattern" "$file"
    else
        sed -i "$pattern" "$file"
    fi

    info "Updated $file"
}

# Main logic
main() {
    local bump_arg="${1:-patch}"

    CURRENT_VERSION=$(get_current_version)
    if [[ -z "$CURRENT_VERSION" ]]; then
        error "Could not determine current version from metadata.json"
    fi

    NEW_VERSION=$(bump_version "$CURRENT_VERSION" "$bump_arg")
    OLD_LATEST=$(get_latest_test_version)

    info "Current version: $CURRENT_VERSION"
    info "New version: $NEW_VERSION"
    info "Replacing test fixture version: $OLD_LATEST -> $NEW_VERSION"
    echo ""

    # Confirm
    read -p "Proceed with version bump? [y/N] " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        info "Aborted"
        exit 0
    fi

    echo ""
    info "=== Updating version references ==="

    # 1. metadata.json
    update_file "$PROJECT_ROOT/metadata.json" \
        "s/\"version\": *\"[^\"]*\"/\"version\": \"$NEW_VERSION\"/"

    # 2. pkg/dbt/dbt.go - VERSION constant
    update_file "$PROJECT_ROOT/pkg/dbt/dbt.go" \
        "s/VERSION = \"[^\"]*\"/VERSION = \"$NEW_VERSION\"/"

    # 3. cmd/dbt/cmd/root.go - Version field
    update_file "$PROJECT_ROOT/cmd/dbt/cmd/root.go" \
        "s/Version: *\"[^\"]*\"/Version: \"$NEW_VERSION\"/"

    # 4. cmd/dbt/cmd/root_test.go - expected version
    update_file "$PROJECT_ROOT/cmd/dbt/cmd/root_test.go" \
        "s/expected := \"[^\"]*\"/expected := \"$NEW_VERSION\"/"

    # 5. pkg/dbt/dbt_setup_test.go - latestVersion
    update_file "$PROJECT_ROOT/pkg/dbt/dbt_setup_test.go" \
        "s/latestVersion *= *\"[^\"]*\"/latestVersion = \"$NEW_VERSION\"/"

    # 6. test/integration/integration_test.go - version references
    # Replace old latest version with new version
    update_file "$PROJECT_ROOT/test/integration/integration_test.go" \
        "s/$OLD_LATEST/$NEW_VERSION/g"

    echo ""
    info "=== Updating test fixtures ==="

    # 7. Update fixtures.go - replace old version embeds with new version
    update_file "$PROJECT_ROOT/pkg/dbt/testfixtures/fixtures.go" \
        "s|repo/dbt/$OLD_LATEST/|repo/dbt/$NEW_VERSION/|g"
    update_file "$PROJECT_ROOT/pkg/dbt/testfixtures/fixtures.go" \
        "s|repo/dbt-tools/catalog/$OLD_LATEST/|repo/dbt-tools/catalog/$NEW_VERSION/|g"

    # Update variable names (e.g., Dbt370 -> Dbt380)
    OLD_VAR=$(echo "$OLD_LATEST" | tr -d '.')
    NEW_VAR=$(echo "$NEW_VERSION" | tr -d '.')
    update_file "$PROJECT_ROOT/pkg/dbt/testfixtures/fixtures.go" \
        "s/Dbt${OLD_VAR}/Dbt${NEW_VAR}/g"
    update_file "$PROJECT_ROOT/pkg/dbt/testfixtures/fixtures.go" \
        "s/Catalog${OLD_VAR}/Catalog${NEW_VAR}/g"

    # 8. Update setup.go - replace version references
    update_file "$PROJECT_ROOT/pkg/dbt/testfixtures/setup.go" \
        "s|\"$OLD_LATEST\"|\"$NEW_VERSION\"|g"
    update_file "$PROJECT_ROOT/pkg/dbt/testfixtures/setup.go" \
        "s/Dbt${OLD_VAR}/Dbt${NEW_VAR}/g"
    update_file "$PROJECT_ROOT/pkg/dbt/testfixtures/setup.go" \
        "s/Catalog${OLD_VAR}/Catalog${NEW_VAR}/g"

    # 9. Update generate-test-fixtures.sh
    update_file "$PROJECT_ROOT/scripts/generate-test-fixtures.sh" \
        "s/$OLD_LATEST/$NEW_VERSION/g"

    echo ""
    info "=== Updating fixture directories ==="

    # 10. Rename fixture directories
    FIXTURE_DIR="$PROJECT_ROOT/pkg/dbt/testfixtures/repo"

    # Move dbt version directory
    if [[ -d "$FIXTURE_DIR/dbt/$OLD_LATEST" ]]; then
        mv "$FIXTURE_DIR/dbt/$OLD_LATEST" "$FIXTURE_DIR/dbt/$NEW_VERSION"
        info "Moved dbt/$OLD_LATEST -> dbt/$NEW_VERSION"
    fi

    # Move catalog version directory
    if [[ -d "$FIXTURE_DIR/dbt-tools/catalog/$OLD_LATEST" ]]; then
        mv "$FIXTURE_DIR/dbt-tools/catalog/$OLD_LATEST" "$FIXTURE_DIR/dbt-tools/catalog/$NEW_VERSION"
        info "Moved dbt-tools/catalog/$OLD_LATEST -> dbt-tools/catalog/$NEW_VERSION"
    fi

    # 11. Update the version string inside the fixture binaries
    DBT_BINARY="$FIXTURE_DIR/dbt/$NEW_VERSION/linux/amd64/dbt"
    if [[ -f "$DBT_BINARY" ]]; then
        cat > "$DBT_BINARY" <<EOF
#!/bin/bash
# DBT Test Binary v$NEW_VERSION
# This is a test fixture, not a real executable
echo "dbt version $NEW_VERSION"
EOF
        chmod 755 "$DBT_BINARY"
        info "Updated dbt binary content"
    fi

    CATALOG_BINARY="$FIXTURE_DIR/dbt-tools/catalog/$NEW_VERSION/linux/amd64/catalog"
    if [[ -f "$CATALOG_BINARY" ]]; then
        cat > "$CATALOG_BINARY" <<EOF
#!/bin/bash
# Catalog Test Binary v$NEW_VERSION
# This is a test fixture, not a real executable
echo "catalog version $NEW_VERSION"
EOF
        chmod 755 "$CATALOG_BINARY"
        info "Updated catalog binary content"
    fi

    echo ""
    info "=== Regenerating checksums and signatures ==="

    # Regenerate test fixtures (this will update checksums and signatures)
    "$SCRIPT_DIR/generate-test-fixtures.sh"

    echo ""
    info "=== Version bump complete ==="
    echo ""
    info "New version: $NEW_VERSION"
    info "Run 'make lint && make test' to verify the changes"
    info "Then commit with: git add -A && git commit -m 'Bump version to $NEW_VERSION'"
}

main "$@"
