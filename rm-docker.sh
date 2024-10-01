#!/bin/bash

# Function to remove Docker resources
remove_docker_resources() {
    local resource_type=$1
    local prefix=$2

    case $resource_type in
        "container")
            containers=$(docker ps -a --format '{{.Names}}' | grep "^$prefix")
            if [ -n "$containers" ]; then
                echo "$containers" | xargs docker rm -f
                echo "Removed $(echo "$containers" | wc -w) container(s)"
            else
                echo "No containers matching the prefix '$prefix' to remove"
            fi
            ;;
        "image")
            images=$(docker images --format '{{.Repository}}:{{.Tag}}' | grep "^$prefix")
            if [ -n "$images" ]; then
                echo "$images" | xargs docker rmi -f
                echo "Removed $(echo "$images" | wc -w) image(s)"
            else
                echo "No images matching the prefix '$prefix' to remove"
            fi
            ;;
        "volume")
            volumes=$(docker volume ls --format '{{.Name}}' | grep "^$prefix")
            if [ -n "$volumes" ]; then
                echo "$volumes" | xargs docker volume rm -f
                echo "Removed $(echo "$volumes" | wc -w) volume(s)"
            else
                echo "No volumes matching the prefix '$prefix' to remove"
            fi
            ;;
    esac
}

# Print pretty header
gum style \
    --border double \
    --align center \
    --width 50 \
    --margin "1 2" \
    --padding "2 4" \
    "Docker Cleanup Script"

echo ""

# Stop containers first
docker stop $(docker ps -a --format '{{.Names}}' | grep -E '^(scmtea|gitea)')

gum spin --spinner dot --title "Removing containers..." -- sleep 2
remove_docker_resources "container" "scmtea"
remove_docker_resources "container" "gitea"

gum spin --spinner dot --title "Removing images..." -- sleep 2
remove_docker_resources "image" "scmtea"
remove_docker_resources "image" "gitea"

gum spin --spinner dot --title "Removing volumes..." -- sleep 2
remove_docker_resources "volume" "scmtea"
remove_docker_resources "volume" "gitea"

# Function to format Docker command output
format_output() {
    local output="$1"
    echo "$output" | awk '{printf "%-20s %-20s %-20s\n", $1, $2, $3}'
}

# Print summary
echo ""
gum style \
    --border normal \
    --align left \
    --width 80 \
    --margin "1 2" \
    --padding "1 2" \
    "$(gum style --foreground 212 "Summary of changes:")

$(gum style --foreground 214 "Remaining Containers:")
$(format_output "$(docker ps -a --format '{{.ID}} {{.Image}} {{.Names}}')")

$(gum style --foreground 214 "Remaining Images:")
$(format_output "$(docker images --format '{{.Repository}} {{.Tag}} {{.ID}}')")

$(gum style --foreground 214 "Remaining Volumes:")
$(docker volume ls --format '{{.Name}}')"
