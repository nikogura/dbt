#!/bin/bash
# Generate test fixtures for dbt tests
# Run this once, commit the results
#
# This script generates pre-built test artifacts to avoid slow gomason compilation
# during tests. The artifacts are signed with a test GPG key.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
FIXTURE_DIR="$PROJECT_ROOT/pkg/dbt/testfixtures"
GPG_DIR="$FIXTURE_DIR/gpg"
REPO_DIR="$FIXTURE_DIR/repo"

# Create directories
mkdir -p "$GPG_DIR"
mkdir -p "$REPO_DIR/dbt/3.0.2/linux/amd64"
mkdir -p "$REPO_DIR/dbt/3.3.4/linux/amd64"
mkdir -p "$REPO_DIR/dbt/3.7.1/linux/amd64"
mkdir -p "$REPO_DIR/dbt-tools/catalog/3.0.2/linux/amd64"
mkdir -p "$REPO_DIR/dbt-tools/catalog/3.3.4/linux/amd64"
mkdir -p "$REPO_DIR/dbt-tools/catalog/3.7.1/linux/amd64"

# Create a temporary GNUPGHOME to avoid polluting user's keyring
export GNUPGHOME=$(mktemp -d)
chmod 700 "$GNUPGHOME"

cleanup() {
    rm -rf "$GNUPGHOME"
}
trap cleanup EXIT

echo "=== Generating GPG test keys ==="

# Generate GPG key batch file
cat > "$GNUPGHOME/keygen.batch" <<EOF
%echo Generating test key for dbt
%no-protection
%transient-key
Key-Type: RSA
Key-Length: 2048
Subkey-Type: RSA
Subkey-Length: 2048
Name-Real: DBT Test Key
Name-Comment: Test signing key
Name-Email: tester@nikogura.com
Expire-Date: 0
%commit
%echo Done
EOF

# Generate the key
gpg --batch --gen-key "$GNUPGHOME/keygen.batch"

# Export public key
gpg --armor --export tester@nikogura.com > "$GPG_DIR/public-key.asc"
cp "$GPG_DIR/public-key.asc" "$REPO_DIR/dbt/truststore"

echo "=== Generating dummy binaries ==="

# Generate dbt binaries for each version
for VERSION in "3.0.2" "3.3.4" "3.7.1"; do
    BINARY="$REPO_DIR/dbt/$VERSION/linux/amd64/dbt"

    # Create a unique binary for each version (just text, tests only check checksums)
    cat > "$BINARY" <<EOF
#!/bin/bash
# DBT Test Binary v$VERSION
# This is a test fixture, not a real executable
echo "dbt version $VERSION"
EOF
    chmod 755 "$BINARY"

    # Generate checksum
    sha256sum "$BINARY" | cut -d' ' -f1 > "$BINARY.sha256"

    # Sign the binary
    gpg --batch --yes --armor --detach-sign --local-user tester@nikogura.com "$BINARY"

    echo "Created dbt $VERSION binary with checksum and signature"
done

# Generate catalog binaries for each version
for VERSION in "3.0.2" "3.3.4" "3.7.1"; do
    CATALOG_DIR="$REPO_DIR/dbt-tools/catalog/$VERSION"
    BINARY="$CATALOG_DIR/linux/amd64/catalog"
    DESC="$CATALOG_DIR/description.txt"

    # Create catalog description (no trailing newline)
    printf "Tool for showing available DBT tools." > "$DESC"

    # Sign description
    gpg --batch --yes --armor --detach-sign --local-user tester@nikogura.com "$DESC"

    # Create catalog binary
    cat > "$BINARY" <<EOF
#!/bin/bash
# Catalog Test Binary v$VERSION
# This is a test fixture, not a real executable
echo "catalog version $VERSION"
EOF
    chmod 755 "$BINARY"

    # Generate checksum
    sha256sum "$BINARY" | cut -d' ' -f1 > "$BINARY.sha256"

    # Sign the binary
    gpg --batch --yes --armor --detach-sign --local-user tester@nikogura.com "$BINARY"

    echo "Created catalog $VERSION binary with checksum and signature"
done

# Generate install scripts
echo "=== Generating install scripts ==="

# install_dbt.sh
cat > "$REPO_DIR/dbt/install_dbt.sh" <<'EOF'
#!/bin/bash
# DBT Installation Script (Test Fixture)
echo "This is a test fixture install script"
exit 0
EOF
chmod 755 "$REPO_DIR/dbt/install_dbt.sh"
sha256sum "$REPO_DIR/dbt/install_dbt.sh" | cut -d' ' -f1 > "$REPO_DIR/dbt/install_dbt.sh.sha256"
gpg --batch --yes --armor --detach-sign --local-user tester@nikogura.com "$REPO_DIR/dbt/install_dbt.sh"

# install_dbt_mac_keychain.sh
cat > "$REPO_DIR/dbt/install_dbt_mac_keychain.sh" <<'EOF'
#!/bin/bash
# DBT Mac Keychain Installation Script (Test Fixture)
echo "This is a test fixture install script for macOS keychain"
exit 0
EOF
chmod 755 "$REPO_DIR/dbt/install_dbt_mac_keychain.sh"
sha256sum "$REPO_DIR/dbt/install_dbt_mac_keychain.sh" | cut -d' ' -f1 > "$REPO_DIR/dbt/install_dbt_mac_keychain.sh.sha256"
gpg --batch --yes --armor --detach-sign --local-user tester@nikogura.com "$REPO_DIR/dbt/install_dbt_mac_keychain.sh"

echo "=== Test fixtures generated successfully ==="
echo ""
echo "Generated files:"
find "$FIXTURE_DIR" -type f | sort
