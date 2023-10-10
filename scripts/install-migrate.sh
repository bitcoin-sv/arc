#!/bin/bash

# Define the desired version of migrate
MIGRATE_VERSION="v4.16.2"

# Define the URL to download the migrate binary
MIGRATE_URL="https://github.com/golang-migrate/migrate/releases/download/$MIGRATE_VERSION/migrate.darwin-arm64.tar.gz"

## Define the location of your GOPATH/bin directory
GOPATH_BIN="$GOPATH/bin"
#
## Create a temporary directory for unpacking
TEMP_DIR="$(mktemp -d)"
#
## Download and unpack the migrate binary
echo "Downloading and unpacking migrate binary..."
curl -LJ "$MIGRATE_URL" | tar -xz -C "$TEMP_DIR"

## Move the binary to GOPATH/bin
echo "Moving migrate binary to $GOPATH_BIN..."
mv "$TEMP_DIR/migrate" "$GOPATH_BIN/migrate"

rm -rf $TEMP_DIR

# Verify the installation
if command -v migrate &>/dev/null; then
    echo "migrate $MIGRATE_VERSION has been installed to $GOPATH_BIN"
else
    echo "Installation of migrate failed. Please check your GOPATH and try again."
fi
