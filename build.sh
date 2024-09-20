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

# Function to update Charm library versions
update_charm_versions() {
    local dir=$1
    cd "$dir"
    go get github.com/charmbracelet/huh@latest
    go get github.com/charmbracelet/log@latest
    go mod tidy
    cd -
}

# Update main application
update_charm_versions .

# Update gitspace-plugin to the latest version
gum spin --spinner dot --title "Updating gitspace-plugin..." -- bash -c '
    go get -u github.com/ssotops/gitspace-plugin@latest
    go mod tidy
    go mod download
'

# Ensure we're using the latest version
latest_version=$(go list -m -f "{{.Version}}" github.com/ssotops/gitspace-plugin)
gum style \
    --foreground 82 --border-foreground 82 --border normal \
    --align center --width 70 --margin "1 2" --padding "1 2" \
    "Using gitspace-plugin version: $latest_version"

# Build main Gitspace application
gum spin --spinner dot --title "Building Gitspace main application..." -- sleep 2
CGO_ENABLED=1 go build -tags pluginload -buildmode=pie -o gitspace .

# Print installed local plugins
echo "Currently installed local plugins:"
for plugin in ~/.ssot/gitspace/plugins/*.so; do
    if [ -f "$plugin" ]; then
        plugin_name=$(basename "$plugin" .so)
        gum style \
            --foreground 39 --border-foreground 39 --border normal \
            --align left --width 50 --margin "0 2" --padding "0 1" \
            "🔌 $plugin_name"
    fi
done

# Print tree structure of plugins directory
tree_output=$(tree -L 2 ~/.ssot/gitspace/plugins)
gum style \
    --foreground 226 --border-foreground 226 --border double \
    --align left --width 70 --margin "1 2" --padding "1 2" \
    "Local Plugins Directory Structure:

$tree_output"

# Inform about remote plugins and potential updates to gitspace-catalog
gum style \
    --foreground 214 --border-foreground 214 --border normal \
    --align center --width 70 --margin "1 2" --padding "1 2" \
    "Note: Remote plugins like 'templater' are managed through the Gitspace Catalog. If there were version updates, a new branch may have been created in the gitspace-catalog repository. Please check and create a pull request if necessary."
