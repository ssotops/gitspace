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

# ASCII Art for Gitspace Version Updater using gum
gum style \
    --foreground 212 --border-foreground 212 --border double \
    --align center --width 70 --margin "1 2" --padding "1 2" \
    "Gitspace Version Updater"

# Run the update_versions tool
gum spin --spinner dot --title "Updating versions..." -- go run cmd/update_versions/main.go

# Function to update a single plugin
update_plugin() {
    local plugin_dir="$1"
    local plugin_name=$(basename "$plugin_dir")
    
    gum style \
        --foreground 39 --border-foreground 39 --border normal \
        --align left --width 70 --margin "1 2" --padding "1 2" \
        "Updating plugin: $plugin_name"

    cd "$plugin_dir"
    
    # Update go.mod
    gum spin --spinner dot --title "Updating go.mod..." -- go mod tidy
    
    # Rebuild plugin
    gum spin --spinner dot --title "Rebuilding plugin..." -- go build -buildmode=plugin -o "${plugin_name}.so" .
    
    cd - > /dev/null
}

# Update all plugins in examples/plugins directory
plugins_dir="examples/plugins"
for plugin_dir in "$plugins_dir"/*; do
    if [ -d "$plugin_dir" ]; then
        update_plugin "$plugin_dir"
    fi
done

# Print summary
gum style \
    --foreground 82 --border-foreground 82 --border normal \
    --align center --width 70 --margin "1 2" --padding "1 2" \
    "Version update complete!
All local plugins in $plugins_dir have been updated."

# Inform about potential updates to gitspace-catalog
gum style \
    --foreground 214 --border-foreground 214 --border normal \
    --align center --width 70 --margin "1 2" --padding "1 2" \
    "Note: If there were version updates, a new branch may have been created in the gitspace-catalog repository for remote plugins. Please check and create a pull request if necessary."
