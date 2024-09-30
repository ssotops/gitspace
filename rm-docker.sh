#!/bin/bash

# Function to remove Docker resources
remove_docker_resources() {
    local resource_type=$1
    local command=$2
    local filter=$3

    # Get the list of resources to remove
    resources=$(docker $command --filter "$filter" -q)

    if [ -n "$resources" ]; then
        echo "$resources" | xargs docker $resource_type rm -f
        echo "Removed $(echo "$resources" | wc -w) $resource_type(s)"
    else
        echo "No $resource_type(s) matching the filter to remove"
    fi
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

gum spin --spinner dot --title "Removing containers..." -- sleep 2
remove_docker_resources "container" "ps -a" "name=scmtea|name=gitea"

gum spin --spinner dot --title "Removing images..." -- sleep 2
remove_docker_resources "image" "images" "reference=scmtea|reference=gitea"

gum spin --spinner dot --title "Removing volumes..." -- sleep 2
remove_docker_resources "volume" "volume ls" "name=scmtea|name=gitea"

# Print summary
echo ""
gum style \
    --border normal \
    --align left \
    --width 70 \
    --margin "1 2" \
    --padding "1 2" \
    "$(gum style --foreground 212 "Summary of changes:")\n\n$(gum style --foreground 214 "Remaining Containers:")\n$(docker ps -a)\n\n$(gum style --foreground 214 "Remaining Images:")\n$(docker images)\n\n$(gum style --foreground 214 "Remaining Volumes:")\n$(docker volume ls)"
