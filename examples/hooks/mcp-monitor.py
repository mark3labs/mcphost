#!/usr/bin/env python3
"""
Monitors MCP tool usage and enforces policies.
"""
import json
import sys
import re
import os
from datetime import datetime

# Define MCP tool policies
BLOCKED_MCP_TOOLS = [
    "mcp__github__delete_.*",  # Block all GitHub delete operations
    "mcp__aws__.*_production", # Block production AWS operations
]

RATE_LIMITS = {
    "mcp__openai__.*": (10, 60),  # 10 calls per 60 seconds
}

def check_rate_limit(tool_name, limits):
    # This is a simplified example - real implementation would need persistent storage
    # For now, just log the attempt
    return True

def main():
    try:
        input_data = json.load(sys.stdin)
        tool_name = input_data.get('tool_name', '')
        
        # Check if tool is blocked
        for pattern in BLOCKED_MCP_TOOLS:
            if re.match(pattern, tool_name):
                output = {
                    "decision": "block",
                    "reason": f"Tool {tool_name} is blocked by security policy"
                }
                print(json.dumps(output))
                sys.exit(0)
        
        # Check rate limits
        for pattern, (limit, window) in RATE_LIMITS.items():
            if re.match(pattern, tool_name):
                if not check_rate_limit(tool_name, (limit, window)):
                    output = {
                        "decision": "block",
                        "reason": f"Rate limit exceeded: {limit} calls per {window}s"
                    }
                    print(json.dumps(output))
                    sys.exit(0)
        
        # Log MCP tool usage
        log_entry = {
            "timestamp": datetime.now().isoformat(),
            "tool": tool_name,
            "input": input_data.get('tool_input', {})
        }
        
        # Use XDG_CONFIG_HOME if set, otherwise default to ~/.config
        config_home = os.environ.get('XDG_CONFIG_HOME', os.path.expanduser('~/.config'))
        log_dir = os.path.join(config_home, 'mcphost', 'logs')
        os.makedirs(log_dir, exist_ok=True)
        
        with open(os.path.join(log_dir, "mcp-usage.jsonl"), "a") as f:
            f.write(json.dumps(log_entry) + "\n")
        
    except Exception as e:
        print(f"Hook error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()