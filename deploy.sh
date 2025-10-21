#!/bin/bash
# MCPHost Deployment Script
# This script makes it easy to deploy and run MCPHost with your preferred model
#
# SETUP:
# 1. Create a .env file with your API keys (see README for details)
# 2. Edit the MODEL variable below to your preferred model
# 3. Run: ./deploy.sh

# 🖊️ EDIT THIS: Choose your model from OpenRouter (or other providers)
# Popular free OpenRouter models:
# - deepseek/deepseek-chat-v3.1:free
# - microsoft/wizardlm-2-8x22b
# - meta-llama/llama-3.1-8b-instruct:free
# - qwen/qwen2.5-coder-32b-instruct
MODEL="openrouter:deepseek/deepseek-chat-v3.1:free"

# 🖊️ EDIT THIS: Set your provider (openrouter, openai, anthropic, etc.)
PROVIDER="openrouter"

echo "🚀 Deploying MCPHost with $MODEL..."
echo "Loading environment variables from .env..."

# Check if .env exists
if [ ! -f ".env" ]; then
    echo "❌ Error: .env file not found!"
    echo "Please create a .env file with your API key (see README for details)"
    exit 1
fi

# Source environment variables
source .env

# Check if API key is available
API_KEY_VAR="${PROVIDER^^}_API_KEY"
API_KEY="${!API_KEY_VAR}"

if [ -z "$API_KEY" ] && [ "$PROVIDER" = "openrouter" ]; then
    API_KEY="$OPENROUTER_API_KEY"
fi

if [ -z "$API_KEY" ]; then
    echo "❌ Error: API key not found in .env file!"
    echo "Make sure your .env contains: $API_KEY_VAR=your_api_key"
    exit 1
fi

# Build if binary doesn't exist
if [ ! -f "output/mcphost" ]; then
    echo "🔨 Building MCPHost binary..."
    ./contribute/build.sh
fi

# Run MCPHost
echo "✅ Starting MCPHost with $MODEL..."
echo ""
exec ./output/mcphost --provider-api-key="$API_KEY" --model="$MODEL" "$@"
