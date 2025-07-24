#!/usr/bin/env python3
"""
Validates bash commands before execution.
Blocks dangerous commands and suggests alternatives.
"""
import json
import sys
import re

# Define validation rules
DANGEROUS_PATTERNS = [
    (r'\brm\s+-rf\s+/', "Dangerous command: rm -rf /"),
    (r'\bdd\s+.*\bof=/dev/[sh]d[a-z]', "Direct disk write detected"),
    (r'>\s*/dev/null\s+2>&1', "Consider using proper error handling instead of discarding stderr"),
]

SUGGEST_ALTERNATIVES = {
    r'\bgrep\b': "Use 'rg' (ripgrep) for better performance",
    r'\bfind\s+.*-name': "Use 'fd' for faster file finding",
}

def main():
    try:
        # Read input
        input_data = json.load(sys.stdin)
        
        # Only process bash commands
        if input_data.get('tool_name') != 'bash':
            sys.exit(0)
        
        command = json.loads(input_data.get('tool_input', '{}')).get('command', '')
        
        # Check dangerous patterns
        for pattern, message in DANGEROUS_PATTERNS:
            if re.search(pattern, command, re.IGNORECASE):
                print(message, file=sys.stderr)
                sys.exit(2)  # Block execution
        
        # Suggest alternatives
        suggestions = []
        for pattern, suggestion in SUGGEST_ALTERNATIVES.items():
            if re.search(pattern, command):
                suggestions.append(suggestion)
        
        if suggestions:
            output = {
                "decision": "approve",
                "reason": "Command approved. Suggestions: " + "; ".join(suggestions)
            }
            print(json.dumps(output))
        
    except Exception as e:
        print(f"Hook error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()