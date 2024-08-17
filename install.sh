#!/bin/bash

set -e

# Determine OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64 | arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Set GitHub repo and binary name
REPO="ssotops/gitspace"
BINARY="gitspace"

# Fetch the latest release information
echo "Fetching latest release information..."
RELEASE_INFO=$(curl -s "https://api.github.com/repos/$REPO/releases/latest")

# Extract the tag name (version) and release ID
VERSION=$(echo "$RELEASE_INFO" | grep -m 1 '"tag_name":' | cut -d'"' -f4)
RELEASE_ID=$(echo "$RELEASE_INFO" | grep -m 1 '"id":' | cut -d':' -f2 | tr -d ' ,')

if [ -z "$VERSION" ] || [ -z "$RELEASE_ID" ]; then
    echo "Failed to fetch latest release information"
    exit 1
fi

echo "Latest version: $VERSION"

# Construct the download URL for the specific asset
ASSET_NAME="${BINARY}_${OS}_${ARCH}"
if [ "$OS" = "windows" ]; then
    ASSET_NAME="${ASSET_NAME}.exe"
fi

DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET_NAME"

# Create a temporary file for downloading
TEMP_FILE=$(mktemp)

# Download the binary
echo "Downloading $BINARY $VERSION for ${OS}_${ARCH}..."
if curl -L -o "$TEMP_FILE" "$DOWNLOAD_URL"; then
    echo "Download completed successfully."
else
    echo "Failed to download $BINARY"
    rm -f "$TEMP_FILE"
    exit 1
fi

# Make it executable (skip for Windows)
if [ "$OS" != "windows" ]; then
    chmod +x "$TEMP_FILE"
fi

# Move to a directory in PATH
if [ "$OS" = "windows" ]; then
    mv "$TEMP_FILE" "${BINARY}.exe"
    echo "Please move ${BINARY}.exe to a directory in your PATH"
else
    sudo mv "$TEMP_FILE" "/usr/local/bin/$BINARY"
fi

echo "$BINARY $VERSION has been installed successfully!"
