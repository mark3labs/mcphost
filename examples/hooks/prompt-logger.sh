#!/bin/bash
# Logs all user prompts with timestamp

# Read JSON input
input=$(cat)

# Extract prompt using jq (ensure jq is installed)
prompt=$(echo "$input" | jq -r '.prompt // empty')

if [ -n "$prompt" ]; then
    # Use XDG_CONFIG_HOME if set, otherwise default to ~/.config
    CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}"
    LOG_DIR="$CONFIG_DIR/mcphost/logs"
    
    # Create log directory if it doesn't exist
    mkdir -p "$LOG_DIR"
    
    # Log with timestamp
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $prompt" >> "$LOG_DIR/prompts.log"
fi

# Always allow prompt to continue
exit 0