#!/bin/bash

set -e

# Build main Gitspace application
echo "Building Gitspace main application..."
CGO_ENABLED=1 go build -buildmode=pie -o gitspace .

# Build hello_world plugin
echo "Building hello_world plugin..."
cd examples/plugins/hello_world
CGO_ENABLED=1 go build -buildmode=plugin -o hello_world.so .
cd ../../..

echo "Build complete!"
echo "Gitspace executable: ./gitspace"
echo "Plugins directory: ./plugins"
