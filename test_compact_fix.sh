#!/bin/bash

echo "Testing compact mode coordinate tracking..."
echo "This should show tool calls, responses, and usage info without overlap"

# Test with a command that will trigger tool calls in compact mode
echo "echo test" | timeout 15s ./output/mcphost --compact --prompt "echo test"

echo ""
echo "Exit code: $?"