#!/bin/bash

echo "Testing coordinate tracking with tool calls..."
echo "This should show tool calls and usage info without overlap"

# Test with a command that will trigger tool calls
echo "list files" | timeout 15s ./output/mcphost --compact --prompt "list files in current directory"

echo ""
echo "Exit code: $?"