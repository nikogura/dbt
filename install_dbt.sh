#!/usr/bin/env bash

############################################################
# DBT Install script
############################################################
# The values below can be modified to suit your environment
############################################################

# URL for your trusted repository server
HOST="http://localhost:8081/artifactory"

# Trusted repository name
REPO="dbt"

# Initial version to install
VERSION="1.2.3"

# You shouldn't need to modify anything below this point
############################################################

set -e
echo "Installing DBT $VERSION..."

INSTALL_DIR="/usr/local/bin"
FILE="dbt"

OS=$(echo $(uname -s) | awk '{print tolower($0)}')
ARCH=$(uname -m)

URL="$HOST/$REPO/$FILE/$VERSION/$OS/$ARCH/$FILE"

TMPDIR=$(mktemp -d)

DOWNLOAD_PATH="$TMPDIR/$FILE"

curl -sX GET $URL -o $DOWNLOAD_PATH

if [ -w $INSTALL_DIR ] ; then
    mv $DOWNLOAD_PATH $INSTALL_DIR
    chmod 755 "$INSTALL_DIR/$FILE"
else
    sudo mv $DOWNLOAD_PATH $INSTALL_DIR
    sudo chmod 755 "$INSTALL_DIR/$FILE"

fi

rm -r $TMPDIR

echo "Installation complete."
