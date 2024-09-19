#!/bin/bash

set -e

# Function to handle errors
handle_error() {
    echo "Error: $1"
    exit 1
}

# Function to run tests for a project
run_tests() {
    local project=$1
    echo "Running tests for $project..."
    (cd "$project" && go test ./... -v) || handle_error "Tests failed for $project"
    echo "Tests passed for $project"
    echo
}

# Main script
echo "Starting test process for all projects"
echo

# Test gitspace-plugin
run_tests "gitspace-plugin"

# Test gitspace
run_tests "gitspace"

# Test templater plugin
run_tests "gitspace-catalog/plugins/templater"

echo "All tests completed successfully!"