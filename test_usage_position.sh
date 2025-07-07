#!/bin/bash

# Test script to verify usage info positioning after tool calls
echo "Testing usage info positioning..."

# Use a simple prompt that should trigger tool usage
echo "What files are in this directory?" | ./output/mcphost --prompt --quiet 2>/dev/null

echo ""
echo "Test completed. Usage info should appear at the end."