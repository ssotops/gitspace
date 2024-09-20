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

# Function to install jq
install_jq() {
    echo "Installing jq..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install jq
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        sudo apt-get update && sudo apt-get install -y jq
    else
        echo "Unsupported operating system. Please install jq manually:"
        echo "https://stedolan.github.io/jq/download/"
        exit 1
    fi
}

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "jq is not installed."
    read -p "Do you want to install jq? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        install_jq
    else
        echo "Please install jq manually and run this script again."
        exit 1
    fi
fi

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

# Function to update gitspace-plugin version for main project
update_gitspace_plugin() {
    local dir=$1
    cd "$dir"
    
    # Force-download the latest version
    latest_version=$(curl -s https://api.github.com/repos/ssotops/gitspace-plugin/releases/latest | jq -r .tag_name)
    
    gum style \
        --foreground 208 --border-foreground 208 --border normal \
        --align center --width 70 --margin "1 2" --padding "1 2" \
        "Updating gitspace-plugin to version: $latest_version in $dir"
    
    go get -u "github.com/ssotops/gitspace-plugin@$latest_version"
    go mod tidy
    cd -
}

# Function to update gitspace-plugin version for plugins
update_plugin_gitspace_plugin() {
    local dir=$1
    local version=$2
    cd "$dir"
    
    gum style \
        --foreground 208 --border-foreground 208 --border normal \
        --align center --width 70 --margin "1 2" --padding "1 2" \
        "Updating gitspace-plugin to version: $version in $dir"
    
    # Update go.mod file if it exists
    if [ -f "go.mod" ]; then
        sed -i '' "s|github.com/ssotops/gitspace-plugin v.*|github.com/ssotops/gitspace-plugin $version|g" go.mod
        go mod tidy
    else
        echo "No go.mod file found in $dir. Skipping update."
    fi
    cd -
}

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
update_gitspace_plugin .

# Force update gitspace-plugin to the latest version in main project
gum spin --spinner dot --title "Updating gitspace-plugin in main project..." -- bash -c '
    go get -u github.com/ssotops/gitspace-plugin@latest
    go mod tidy
'

# Ensure we're using the latest version
latest_version=$(go list -m -json github.com/ssotops/gitspace-plugin | jq -r .Version)
current_version=$(grep "github.com/ssotops/gitspace-plugin" go.mod | awk '{print $2}')

if [ "$latest_version" != "$current_version" ]; then
    gum style \
        --foreground 208 --border-foreground 208 --border normal \
        --align center --width 70 --margin "1 2" --padding "1 2" \
        "Warning: gitspace-plugin version mismatch detected.
Current: $current_version
Latest: $latest_version
Forcing update to the latest version..."
    
    go get -u github.com/ssotops/gitspace-plugin@$latest_version
    go mod tidy
fi

gum style \
    --foreground 82 --border-foreground 82 --border normal \
    --align center --width 70 --margin "1 2" --padding "1 2" \
    "Using gitspace-plugin version: $latest_version"

# Update plugins
plugins_dir="$HOME/.ssot/gitspace/plugins"
for plugin_dir in "$plugins_dir"/*; do
    if [ -d "$plugin_dir" ]; then
        update_plugin_gitspace_plugin "$plugin_dir" "$latest_version"
    fi
done

# Build main Gitspace application
gum spin --spinner dot --title "Building Gitspace main application..." -- sleep 2
CGO_ENABLED=1 go build -tags pluginload -buildmode=pie -o gitspace .

# In gitspace's build script, for `gitspace-plugin`
go mod graph | awk '{print $2}' | sort | uniq | awk -F@ '{print "\""$1"\": \""$2"\""}' | jq -s 'reduce .[] as $item ({}; . + $item)' > ~/.ssot/gitspace/canonical-deps.json

# Rebuild plugins
for plugin_dir in "$plugins_dir"/*; do
    if [ -d "$plugin_dir" ]; then
        plugin_name=$(basename "$plugin_dir")
        gum spin --spinner dot --title "Rebuilding plugin: $plugin_name" -- bash -c "
            cd '$plugin_dir'
            go build -buildmode=plugin -o ${plugin_name}.so .
        "
    fi
done

# Verify versions
main_version=$(grep "github.com/ssotops/gitspace-plugin" go.mod | awk '{print $2}')
gum style \
    --foreground 82 --border-foreground 82 --border normal \
    --align center --width 70 --margin "1 2" --padding "1 2" \
    "Main Gitspace using gitspace-plugin version: $main_version"

echo "Plugin versions:"
for plugin_dir in "$plugins_dir"/*; do
    if [ -d "$plugin_dir" ]; then
        plugin_name=$(basename "$plugin_dir")
        plugin_version=$(grep "github.com/ssotops/gitspace-plugin" "$plugin_dir/go.mod" | awk '{print $2}')
        echo "  $plugin_name: $plugin_version"
    fi
done

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
