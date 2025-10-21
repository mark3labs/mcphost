#!/bin/bash
# Script to run MCPHost with proper environment variables
source .env

# Get the API key from the environment
API_KEY="$OPENROUTER_API_KEY"

# Pass the API key as a flag to avoid environment issues
exec ./output/mcphost --provider-api-key="$API_KEY" "$@"
