#!/bin/bash

# Test script to verify tool call/response coordinate tracking
# This will test that usage info appears after all messages, not in the middle

echo "Testing tool call coordinate tracking..."

# Create a simple test that will trigger tool calls
echo "What files are in the current directory?" | ./output/mcphost --prompt --compact

echo ""
echo "Test completed. Check that usage info appeared at the end, not in the middle of tool responses."