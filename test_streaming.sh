#!/bin/bash

# Test script to check streaming behavior
echo "Testing streaming with compact mode..."
echo "hello" | timeout 10s ./output/mcphost --compact --prompt "hello"
echo ""
echo "Exit code: $?"