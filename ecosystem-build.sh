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

# Build SDK
log "Building gitspace-plugin-sdk..."
(
    cd ./gs/gitspace-plugin-sdk
    ./build.sh
)

if [ $? -ne 0 ]; then
    error "Failed to build gitspace-plugin-sdk"
    exit 1
fi

# Build Catalog Plugins
log "Building gitspace-catalog plugins..."
(
    cd ./gs/gitspace-catalog/plugins
    ./build-all-plugins.sh
)

if [ $? -ne 0 ]; then
    error "Failed to build gitspace-catalog plugins"
    exit 1
fi

# Update Gitspace dependencies
log "Updating Gitspace dependencies..."
go get -u github.com/ssotops/gitspace-plugin-sdk
go mod tidy
go mod vendor  # Add this line

if [ $? -ne 0 ]; then
    error "Failed to update Gitspace dependencies"
    exit 1
fi

# Build Gitspace
log "Building Gitspace..."
go build -o gitspace .

if [ $? -ne 0 ]; then
    error "Failed to build Gitspace"
    exit 1
fi

# Run tests
log "Running Gitspace tests..."
go test ./...

if [ $? -ne 0 ]; then
    error "Some Gitspace tests failed"
    exit 1
fi

success "Build process completed successfully!"
