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

# Run the update_versions tool with error handling
gum style --foreground 39 --align left "Running update_versions tool..."
output=$(go run cmd/update_versions.go 2>&1)
exit_code=$?

echo "$output"

if [ $exit_code -ne 0 ]; then
    gum style \
        --foreground 196 --border-foreground 196 --border normal \
        --align center --width 70 --margin "1 2" --padding "1 2" \
        "Error: Failed to run update_versions tool. Check the error message above."
    exit 1
fi

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
    if ! go build -buildmode=plugin -o "${plugin_name}.so" .; then
        gum style \
            --foreground 196 --border-foreground 196 --border normal \
            --align center --width 70 --margin "1 2" --padding "1 2" \
            "Error: Failed to rebuild plugin $plugin_name. Check the error message above."
        return 1
    fi
    
    cd - > /dev/null
}

# Update all plugins in examples/plugins directory
plugins_dir="examples/plugins"
updated_plugins=()
failed_plugins=()

for plugin_dir in "$plugins_dir"/*; do
    if [ -d "$plugin_dir" ]; then
        if update_plugin "$plugin_dir"; then
            updated_plugins+=("$(basename "$plugin_dir")")
        else
            failed_plugins+=("$(basename "$plugin_dir")")
        fi
    fi
done

# Print summary
if [ ${#updated_plugins[@]} -gt 0 ]; then
    gum style \
        --foreground 82 --border-foreground 82 --border normal \
        --align left --width 70 --margin "1 2" --padding "1 2" \
        "Successfully updated plugins:
$(printf " - %s\n" "${updated_plugins[@]}")"
fi

if [ ${#failed_plugins[@]} -gt 0 ]; then
    gum style \
        --foreground 196 --border-foreground 196 --border normal \
        --align left --width 70 --margin "1 2" --padding "1 2" \
        "Failed to update plugins:
$(printf " - %s\n" "${failed_plugins[@]}")"
fi
