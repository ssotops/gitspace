#!/bin/bash

set -e

# Function to print styled log message
log() {
    echo "➡ $1"
}

# Function to print styled success message
success() {
    echo "✓ $1"
}

# Function to print styled error message
error() {
    echo "✗ $1" >&2
}

# Clean build artifacts
log "Cleaning build artifacts..."
rm -f gitspace
rm -rf vendor/

# Build SDK with debug flags
log "Building gitspace-plugin-sdk..."
(
    cd ./gs/gitspace-plugin-sdk
    ./build.sh
)

if [ $? -ne 0 ]; then
    error "Failed to build gitspace-plugin-sdk"
    exit 1
fi

# Build Catalog Plugins with debug flags
log "Building gitspace-catalog plugins..."
(
    cd ./gs/gitspace-catalog/plugins
    GODEBUG=x509roots=1 ./build-all-plugins.sh
)

if [ $? -ne 0 ]; then
    error "Failed to build gitspace-catalog plugins"
    exit 1
fi

# Update Gitspace dependencies
log "Updating Gitspace dependencies..."
go get -u github.com/ssotops/gitspace-plugin-sdk
go mod tidy
go mod vendor

# Update the replace directive
go mod edit -replace github.com/ssotops/gitspace-plugin-sdk=./gs/gitspace-plugin-sdk

if [ $? -ne 0 ]; then
    error "Failed to update Gitspace dependencies"
    exit 1
fi

# Build Gitspace with debug flags
log "Building Gitspace..."
GODEBUG=x509roots=1 go build -o gitspace -gcflags="all=-N -l" .

if [ $? -ne 0 ]; then
    error "Failed to build Gitspace"
    exit 1
fi

# Ensure proper permissions
log "Setting executable permissions..."
chmod +x gitspace
chmod -R +x ./gs/gitspace-catalog/plugins/*/
chmod -R +x ./gs/gitspace-plugin-sdk/

# Run tests
log "Running Gitspace tests..."
go test -v ./...

if [ $? -ne 0 ]; then
    error "Some Gitspace tests failed"
    exit 1
fi

success "Build process completed successfully!"
