#!/usr/bin/env python3
"""
Demo hook showing LLM feedback capabilities
"""
import json
import sys

def main():
    input_data = json.load(sys.stdin)
    hook_event = input_data.get('hook_event_name', '')
    
    output = {}
    
    if hook_event == 'PostToolUse':
        tool_name = input_data.get('tool_name', '')
        tool_response = input_data.get('tool_response', {})
        
        # Analyze tool response
        if 'error' in str(tool_response).lower():
            output['feedback'] = f"The {tool_name} tool encountered an error. Consider checking inputs or using an alternative approach."
            output['context'] = "Tool execution failed"
        
        # Example: Sanitize sensitive data
        if 'password' in str(tool_response).lower():
            output['modifyOutput'] = json.dumps({"result": "Output sanitized for security"})
            output['suppressOutput'] = True
            output['feedback'] = "Sensitive data was detected and removed from output"
    
    elif hook_event == 'UserPromptSubmit':
        prompt = input_data.get('prompt', '')
        
        # Add context based on prompt analysis
        if 'help' in prompt.lower() or '?' in prompt:
            output['context'] = "User is asking for help, provide detailed explanations"
            output['systemPrompt'] = "Be extra helpful and provide examples"
        
        # Example: Rate limiting
        # if check_rate_limit():
        #     output['continue'] = False
        #     output['stopReason'] = "Rate limit exceeded"
    
    print(json.dumps(output))

if __name__ == "__main__":
    main()