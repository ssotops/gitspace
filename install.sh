#!/bin/bash

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

# Determine latest release
RELEASE_URL="https://api.github.com/repos/$REPO/releases/latest"
DOWNLOAD_URL=$(curl -s $RELEASE_URL | grep "browser_download_url.*${BINARY}_${OS}_${ARCH}" | cut -d '"' -f 4)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "Failed to find download URL for ${OS}_${ARCH}"
    exit 1
fi

# Download the binary
echo "Downloading $BINARY..."
curl -L $DOWNLOAD_URL -o $BINARY

# Make it executable (skip for Windows)
if [ "$OS" != "windows" ]; then
    chmod +x $BINARY
fi

# Move to a directory in PATH
if [ "$OS" = "windows" ]; then
    mv $BINARY $BINARY.exe
    echo "Please move $BINARY.exe to a directory in your PATH"
else
    sudo mv $BINARY /usr/local/bin/
fi

echo "$BINARY has been installed successfully!"
