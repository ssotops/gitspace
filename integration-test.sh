#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Function to print colored output
print_color() {
    color=$1
    message=$2
    echo -e "${color}${message}${NC}"
}

# Function to check if a command was successful
check_success() {
    if [ $? -eq 0 ]; then
        print_color "$GREEN" "Success: $1"
    else
        print_color "$RED" "Error: $1"
        exit 1
    fi
}

# Build gitspace
cd gs/gitspace
go build -o gitspace
check_success "Building gitspace"
cd ../..

# Build gitspace-plugin
cd gs/gitspace-plugin
go build ./...
check_success "Building gitspace-plugin"
cd ../..

# Install hello-world plugin
PLUGIN_DIR="$HOME/.ssot/gitspace/plugins"
mkdir -p "$PLUGIN_DIR"

cd gs/gitspace-plugin/examples/hello-world
go build -buildmode=plugin -o "$PLUGIN_DIR/hello-world.so" .
check_success "Building and installing hello-world plugin"
cd ../../../..

# Run gitspace with hello-world plugin
./gs/gitspace/gitspace -test-plugin -plugin="hello-world"
check_success "Running gitspace with hello-world plugin"

# Verify dependency synchronization
verify_dependencies() {
    plugin=$1
    echo "Verifying dependencies for $plugin"
    gitspace_deps=$(./gs/gitspace/gitspace -print-deps)
    plugin_deps=$(cd gs/gitspace-plugin/examples/$plugin && go list -m all)
    if diff <(echo "$gitspace_deps") <(echo "$plugin_deps") > /dev/null; then
        print_color "$GREEN" "Dependencies match for $plugin"
    else
        print_color "$RED" "Dependency mismatch for $plugin"
        echo "Gitspace deps:"
        echo "$gitspace_deps"
        echo "Plugin deps:"
        echo "$plugin_deps"
        exit 1
    fi
}

verify_dependencies "hello-world"

# Introduce a version mismatch
cd gs/gitspace
go get github.com/charmbracelet/lipgloss@v0.14.0
go mod tidy
check_success "Updating lipgloss version in gitspace"
cd ../..

# Run gitspace again to trigger dependency update
./gs/gitspace/gitspace -update-plugins
check_success "Running gitspace to update plugin dependencies"

# Verify that the mismatch was corrected
verify_dependencies "hello-world"

print_color "$GREEN" "All tests passed successfully!"