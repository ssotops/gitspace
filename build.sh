#!/bin/bash

set -e

SKIP_VENDOR=false

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --skip-vendor) SKIP_VENDOR=true ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to prompt for installation
prompt_install() {
    read -p "Would you like to install $1? (y/n) " choice
    case "$choice" in 
      y|Y ) return 0;;
      n|N ) return 1;;
      * ) echo "Invalid input. Please enter y or n."; prompt_install "$1";;
    esac
}

# Function to install gum
install_gum() {
    echo "Installing gum..."
    if command_exists "brew"; then
        brew install gum
    elif command_exists "apt-get"; then
        sudo mkdir -p /etc/apt/keyrings
        curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
        echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list
        sudo apt update && sudo apt install gum
    else
        echo "Unable to install gum. Please install it manually: https://github.com/charmbracelet/gum#installation"
        exit 1
    fi
}

# Check and install gum if necessary
if ! command_exists gum; then
    echo "gum is not installed."
    if prompt_install "gum"; then
        install_gum
    else
        echo "gum is required for this script. Exiting."
        exit 1
    fi
fi

# Function to print styled header
print_header() {
    gum style \
        --foreground 212 --border-foreground 212 --border double \
        --align center --width 50 --margin "1 2" --padding "2 4" \
        'Gitspace Builder'
}

# Function to print styled log message
log() {
    gum style --foreground 39 "$(gum style --bold "➡")" "$1"
}

# Function to print styled success message
success() {
    gum style --foreground 76 "$(gum style --bold "✓")" "$1"
}

# Function to print styled error message
error() {
    gum style --foreground 196 "$(gum style --bold "✗")" "$1" >&2
}

# Function to handle vendoring
handle_vendoring() {
    if [ "$SKIP_VENDOR" = true ]; then
        log "Skipping vendor directory sync (--skip-vendor flag used)"
        return
    fi
    log "Syncing vendor directory..."
    go mod vendor
    if [ $? -ne 0 ]; then
        error "Failed to sync vendor directory"
        exit 1
    fi
    success "Vendor directory synced successfully"
    changes+=("Vendor directory synced")
}

# Print header
print_header

# Initialize variables to track changes
changes=()

# Update dependencies
log "Updating dependencies..."
go get -u ./...
if [ $? -eq 0 ]; then
    success "Dependencies updated successfully."
    changes+=("Dependencies updated")
else
    error "Failed to update dependencies."
    exit 1
fi

# Tidy up the go.mod file
log "Tidying up go.mod..."
go mod tidy
if [ $? -eq 0 ]; then
    success "go.mod tidied successfully."
    changes+=("go.mod tidied")
else
    error "Failed to tidy go.mod."
    exit 1
fi

# Handle vendoring
handle_vendoring

# Build the project
log "Building the project..."
go build -o gitspace .
if [ $? -eq 0 ]; then
    success "Project built successfully."
    changes+=("Project built")
else
    error "Failed to build the project."
    exit 1
fi

# Run tests
log "Running tests..."
go test ./...
if [ $? -eq 0 ]; then
    success "All tests passed."
    changes+=("Tests passed")
else
    error "Some tests failed."
    exit 1
fi

# Print summary
gum style \
    --foreground 226 --border-foreground 226 --border normal \
    --align left --width 50 --margin "1 2" --padding "1 2" \
    "Summary of Changes:"

for change in "${changes[@]}"; do
    gum style --foreground 226 "• $change"
done

success "Build process completed successfully!"
