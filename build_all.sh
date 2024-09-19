#!/bin/bash

set -e

# Function to handle errors
handle_error() {
    echo "Error: $1"
    exit 1
}

# Function to run a build script
run_build_script() {
    local project=$1
    local script=$2
    echo "Building $project..."
    if [ -f "$project/$script" ]; then
        (cd "$project" && ./$script) || handle_error "Failed to build $project"
    else
        handle_error "Build script $script not found in $project"
    fi
    echo "$project built successfully"
    echo
}

# Main script
echo "Starting build process for all projects"
echo

# Build gitspace-plugin
run_build_script "gitspace-plugin" "build.sh"

# Build gitspace
run_build_script "gitspace" "build.sh"

# Build templater plugin
run_build_script "gitspace-catalog/plugins/templater" "build-plugin.sh"

echo "All projects built successfully!"