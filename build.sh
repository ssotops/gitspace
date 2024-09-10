#!/bin/bash

set -e

# Function to install gum
install_gum() {
    echo "Installing gum..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install gum
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        echo "For Ubuntu/Debian:"
        echo "sudo mkdir -p /etc/apt/keyrings"
        echo "curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg"
        echo 'echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list'
        echo "sudo apt update && sudo apt install gum"
        echo ""
        echo "For other Linux distributions, please visit: https://github.com/charmbracelet/gum#installation"
        read -p "Do you want to proceed with the installation for Ubuntu/Debian? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            sudo mkdir -p /etc/apt/keyrings
            curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
            echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list
            sudo apt update && sudo apt install gum
        else
            echo "Please install gum manually and run this script again."
            exit 1
        fi
    else
        echo "Unsupported operating system. Please install gum manually:"
        echo "https://github.com/charmbracelet/gum#installation"
        exit 1
    fi
}

# Check if gum is installed
if ! command -v gum &> /dev/null; then
    echo "gum is not installed."
    read -p "Do you want to install gum? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        install_gum
    else
        echo "Please install gum manually and run this script again."
        exit 1
    fi
fi

# ASCII Art for gitspace builder using gum
gum style \
    --foreground 212 --border-foreground 212 --border double \
    --align center --width 70 --margin "1 2" --padding "1 2" \
    "Gitspace Builder"
# Build main Gitspace application
gum spin --spinner dot --title "Building Gitspace main application..." -- sleep 2
CGO_ENABLED=1 go build -buildmode=pie -o gitspace .

# Build hello-world plugin
gum spin --spinner dot --title "Building hello-world plugin..." -- sleep 2
cd examples/plugins/hello-world
go mod edit -replace=github.com/ssotops/gitspace=../../../
go mod tidy
CGO_ENABLED=1 go build -buildmode=plugin -o hello-world.so .
cd ../../..

# Build templater plugin
# gum spin --spinner dot --title "Building templater plugin..." -- sleep 2
# cd examples/plugins/templater
# go mod edit -replace=github.com/ssotops/gitspace=../../../
# go mod tidy
# CGO_ENABLED=1 go build -buildmode=plugin -o templater.so .
# cd ../../..

gum style \
    --foreground 212 --border-foreground 212 --border normal \
    --align left --width 70 --margin "1 2" --padding "1 2" \
    "Build complete!
Gitspace executable: ./gitspace
Plugins directory: ~/.ssot/gitspace/plugins"

# Ask for confirmation using gum
if gum confirm "Do you want to copy plugins to the plugins directory?"; then
    # Create plugins directory if it doesn't exist
    mkdir -p ~/.ssot/gitspace/plugins

    # Copy plugins to the plugins directory
    cp examples/plugins/hello-world/hello-world.so ~/.ssot/gitspace/plugins/
    cp examples/plugins/templater/templater.so ~/.ssot/gitspace/plugins/

    gum style \
        --foreground 82 --border-foreground 82 --border normal \
        --align center --width 70 --margin "1 2" --padding "1 2" \
        "Plugins copied to ~/.ssot/gitspace/plugins/"
else
    gum style \
        --foreground 208 --border-foreground 208 --border normal \
        --align center --width 70 --margin "1 2" --padding "1 2" \
        "Plugins were not copied to the plugins directory."
fi

# Print installed plugins
echo "Installed plugins:"
for plugin in ~/.ssot/gitspace/plugins/*.so; do
    if [ -f "$plugin" ]; then
        plugin_name=$(basename "$plugin" .so)
        gum style \
            --foreground 39 --border-foreground 39 --border normal \
            --align left --width 50 --margin "0 2" --padding "0 1" \
            "🔌 $plugin_name"
    fi
done
