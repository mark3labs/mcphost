# MCPHost 🤖

A CLI host application that enables Large Language Models (LLMs) to interact with external tools through the Model Context Protocol (MCP). Currently supports both Claude 3.5 Sonnet and Ollama models.

Discuss the Project on [Discord](https://discord.gg/RqSS2NQVsY)

## Overview 🌟

MCPHost acts as a host in the MCP client-server architecture, where:
- **Hosts** (like MCPHost) are LLM applications that manage connections and interactions
- **Clients** maintain 1:1 connections with MCP servers
- **Servers** provide context, tools, and capabilities to the LLMs

This architecture allows language models to:
- Access external tools and data sources 🛠️
- Maintain consistent context across interactions 🔄
- Execute commands and retrieve information safely 🔒

Currently supports:
- Claude 3.5 Sonnet (claude-3-5-sonnet-20240620)
- Any Ollama-compatible model with function calling support
- Google Gemini models
- Any OpenAI-compatible local or online model with function calling support

## Features ✨

- Interactive conversations with support models
- **Non-interactive mode** for scripting and automation
- **Script mode** for executable YAML-based automation scripts
- Support for multiple concurrent MCP servers
- **Tool filtering** with `allowedTools` and `excludedTools` per server
- Dynamic tool discovery and integration
- Tool calling capabilities for both model types
- Configurable MCP server locations and arguments
- Consistent command interface across model types
- Configurable message history window for context management

## Requirements 📋

- Go 1.23 or later
- For OpenAI/Anthropic: API key for the respective provider
- For Ollama: Local Ollama installation with desired models
- For Google/Gemini: Google API key (see https://aistudio.google.com/app/apikey)
- One or more MCP-compatible tool servers

## Environment Setup 🔧

1. API Keys:
```bash
# For all providers (use --provider-api-key flag or these environment variables)
export OPENAI_API_KEY='your-openai-key'        # For OpenAI
export ANTHROPIC_API_KEY='your-anthropic-key'  # For Anthropic
export GOOGLE_API_KEY='your-google-key'        # For Google/Gemini
```

2. Ollama Setup:
- Install Ollama from https://ollama.ai
- Pull your desired model:
```bash
ollama pull mistral
```
- Ensure Ollama is running:
```bash
ollama serve
```

You can also configure the Ollama client using standard environment variables, such as `OLLAMA HOST` for the Ollama base URL.

3. Google API Key (for Gemini):
```bash
export GOOGLE_API_KEY='your-api-key'
```

4. OpenAI Compatible Setup:
- Get your API server base URL, API key and model name
- Use `--provider-url` and `--provider-api-key` flags or set environment variables

## Installation 📦

```bash
go install github.com/mark3labs/mcphost@latest
```

## Configuration ⚙️

### MCP-server
MCPHost will automatically create a configuration file in your home directory if it doesn't exist. It looks for config files in this order:
- `.mcphost.yml` or `.mcphost.json` (preferred)
- `.mcp.yml` or `.mcp.json` (backwards compatibility)

**Config file locations by OS:**
- **Linux/macOS**: `~/.mcphost.yml`, `~/.mcphost.json`, `~/.mcp.yml`, `~/.mcp.json`
- **Windows**: `%USERPROFILE%\.mcphost.yml`, `%USERPROFILE%\.mcphost.json`, `%USERPROFILE%\.mcp.yml`, `%USERPROFILE%\.mcp.json`

You can also specify a custom location using the `--config` flag.

#### STDIO
The configuration for an STDIO MCP-server should be defined as the following:
```json
{
  "mcpServers": {
    "sqlite": {
      "command": "uvx",
      "args": [
        "mcp-server-sqlite",
        "--db-path",
        "/tmp/foo.db"
      ]
    },
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem",
        "/tmp"
      ],
      "allowedTools": ["read_file", "write_file"],
      "excludedTools": ["delete_file"]
    }
  }
}
```

Each STDIO entry requires:
- `command`: The command to run (e.g., `uvx`, `npx`) 
- `args`: Array of arguments for the command:
  - For SQLite server: `mcp-server-sqlite` with database path
  - For filesystem server: `@modelcontextprotocol/server-filesystem` with directory path
- `allowedTools`: (Optional) Array of tool names to include (whitelist)
- `excludedTools`: (Optional) Array of tool names to exclude (blacklist)

**Note**: `allowedTools` and `excludedTools` are mutually exclusive - you can only use one per server.

### Server Side Events (SSE) 

For SSE the following config should be used:
```json
{
  "mcpServers": {
    "server_name": {
      "url": "http://some_jhost:8000/sse",
      "headers":[
        "Authorization: Bearer my-token"
       ]
    }
  }
}
```

Each SSE entry requires:
- `url`: The URL where the MCP server is accessible. 
- `headers`: (Optional) Array of headers that will be attached to the requests

### System Prompt

You can specify a custom system prompt using the `--system-prompt` flag. You can either:

1. **Pass the prompt directly as text:**
   ```bash
   mcphost --system-prompt "You are a helpful assistant that responds in a friendly tone."
   ```

2. **Pass a path to a text file containing the prompt:**
   ```bash
   mcphost --system-prompt ./prompts/assistant.md
   ```

   Example `assistant.md` file:
   ```markdown
   You are a helpful coding assistant. 
   
   Please:
   - Write clean, readable code
   - Include helpful comments
   - Follow best practices
   - Explain your reasoning
   ```

Usage:
```bash
mcphost --system-prompt ./my-system-prompt.json
```


## Usage 🚀

MCPHost is a CLI tool that allows you to interact with various AI models through a unified interface. It supports various tools through MCP servers and can run in both interactive and non-interactive modes.

### Interactive Mode (Default)

Start an interactive conversation session:

```bash
mcphost
```

### Script Mode

Run executable YAML-based automation scripts with variable substitution support:

```bash
# Using the script subcommand
mcphost script myscript.sh

# With variables
mcphost script myscript.sh --args:directory /tmp --args:name "John"

# Direct execution (if executable and has shebang)
./myscript.sh
```

#### Script Format

Scripts combine YAML configuration with prompts in a single executable file:

```yaml
#!/usr/local/bin/mcphost script
# This script uses the container-use MCP server from https://github.com/dagger/container-use
mcpServers:
  container-use:
    command: cu
    args:
      - "stdio"
prompt: |
  Create 2 variations of a simple hello world app using Flask and FastAPI. 
  Each in their own environment. Give me the URL of each app
```

#### Variable Substitution

Scripts support variable substitution using `${variable}` syntax. Variables must be provided via command line arguments:

```bash
# Script with variables
mcphost script myscript.sh --args:directory /tmp --args:name "John"
```

Example script with variables:
```yaml
#!/usr/local/bin/mcphost script
mcpServers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "${directory}"]
prompt: |
  Hello ${name}! Please list the files in ${directory} and tell me about them.
```

**Important**: All declared variables (e.g., `${directory}`, `${name}`) must be provided using `--args:variable value` syntax, or the script will exit with an error listing the missing variables.

#### Script Features

- **Executable**: Use shebang line for direct execution
- **YAML Configuration**: Define MCP servers directly in the script
- **Embedded Prompts**: Include the prompt in the YAML
- **Variable Substitution**: Use `${variable}` syntax with `--args:variable value`
- **Variable Validation**: Missing variables cause script to exit with helpful error
- **Interactive Mode**: If prompt is empty, drops into interactive mode (handy for setup scripts)
- **Config Fallback**: If no `mcpServers` defined, uses default config
- **Tool Filtering**: Supports `allowedTools`/`excludedTools` per server
- **Clean Exit**: Automatically exits after completion

#### Script Examples

See `examples/scripts/` for sample scripts:
- `example-script.sh` - Script with custom MCP servers
- `simple-script.sh` - Script using default config fallback

### Non-Interactive Mode

Run a single prompt and exit - perfect for scripting and automation:

```bash
# Basic non-interactive usage
mcphost -p "What is the weather like today?"

# Quiet mode - only output the AI response (no UI elements)
mcphost -p "What is 2+2?" --quiet

# Use with different models
mcphost -m ollama:qwen2.5:3b -p "Explain quantum computing" --quiet
```

### Model Generation Parameters

MCPHost supports fine-tuning model behavior through various parameters:

```bash
# Control response length
mcphost -p "Explain AI" --max-tokens 1000

# Adjust creativity (0.0 = focused, 1.0 = creative)
mcphost -p "Write a story" --temperature 0.9

# Control diversity with nucleus sampling
mcphost -p "Generate ideas" --top-p 0.8

# Limit token choices for more focused responses
mcphost -p "Answer precisely" --top-k 20

# Set custom stop sequences
mcphost -p "Generate code" --stop-sequences "```","END"
```

These parameters work with all supported providers (OpenAI, Anthropic, Google, Ollama) where supported by the underlying model.

### Available Models
Models can be specified using the `--model` (`-m`) flag:
- Anthropic Claude (default): `anthropic:claude-3-5-sonnet-latest`
- OpenAI or OpenAI-compatible: `openai:gpt-4`
- Ollama models: `ollama:modelname`
- Google: `google:gemini-2.0-flash`

### Examples

#### Interactive Mode
```bash
# Use Ollama with Qwen model
mcphost -m ollama:qwen2.5:3b

# Use OpenAI's GPT-4
mcphost -m openai:gpt-4

# Use OpenAI-compatible model with custom URL and API key
mcphost --model openai:<your-model-name> \
--provider-url <your-base-url> \
--provider-api-key <your-api-key>
```

#### Non-Interactive Mode
```bash
# Single prompt with full UI
mcphost -p "List files in the current directory"

# Quiet mode for scripting (only AI response output)
mcphost -p "What is the capital of France?" --quiet

# Use in shell scripts
RESULT=$(mcphost -p "Calculate 15 * 23" --quiet)
echo "The answer is: $RESULT"

# Pipe to other commands
mcphost -p "Generate a random UUID" --quiet | tr '[:lower:]' '[:upper:]'
```

### Flags
- `--provider-url string`: Base URL for the provider API (applies to OpenAI, Anthropic, Ollama, and Google)
- `--provider-api-key string`: API key for the provider (applies to OpenAI, Anthropic, and Google)
- `--config string`: Config file location (default is $HOME/.mcphost.yml)
- `--system-prompt string`: system-prompt file location
- `--debug`: Enable debug logging
- `--max-steps int`: Maximum number of agent steps (0 for unlimited, default: 0)
- `-m, --model string`: Model to use (format: provider:model) (default "anthropic:claude-sonnet-4-20250514")
- `-p, --prompt string`: **Run in non-interactive mode with the given prompt**
- `--quiet`: **Suppress all output except the AI response (only works with --prompt)**

#### Model Generation Parameters
- `--max-tokens int`: Maximum number of tokens in the response (default: 4096)
- `--temperature float32`: Controls randomness in responses (0.0-1.0, default: 0.7)
- `--top-p float32`: Controls diversity via nucleus sampling (0.0-1.0, default: 0.95)
- `--top-k int32`: Controls diversity by limiting top K tokens to sample from (default: 40)
- `--stop-sequences strings`: Custom stop sequences (comma-separated)

### Configuration File Support

All command-line flags can be configured via the config file. MCPHost will look for configuration in this order:
1. `~/.mcphost.yml` or `~/.mcphost.json` (preferred)
2. `~/.mcp.yml` or `~/.mcp.json` (backwards compatibility)

Example config file (`~/.mcphost.yml`):
```yaml
# MCP Servers
mcpServers:
  filesystem:
    command: npx
    args: ["@modelcontextprotocol/server-filesystem", "/path/to/files"]

# Application settings
model: "anthropic:claude-sonnet-4-20250514"
max-steps: 20
debug: false
system-prompt: "/path/to/system-prompt.txt"

# Model generation parameters
max-tokens: 4096
temperature: 0.7
top-p: 0.95
top-k: 40
stop-sequences: ["Human:", "Assistant:"]

# API Configuration
provider-api-key: "your-api-key"      # For OpenAI, Anthropic, or Google
provider-url: "https://api.openai.com/v1"  # Custom base URL
```

**Note**: Command-line flags take precedence over config file values.


### Interactive Commands

While chatting, you can use:
- `/help`: Show available commands
- `/tools`: List all available tools
- `/servers`: List configured MCP servers
- `/history`: Display conversation history
- `/quit`: Exit the application
- `Ctrl+C`: Exit at any time

### Global Flags
- `--config`: Specify custom config file location

## Automation & Scripting 🤖

MCPHost's non-interactive mode makes it perfect for automation, scripting, and integration with other tools.

### Use Cases

#### Shell Scripts
```bash
#!/bin/bash
# Get weather and save to file
mcphost -p "What's the weather in New York?" --quiet > weather.txt

# Process files with AI
for file in *.txt; do
    summary=$(mcphost -p "Summarize this file: $(cat $file)" --quiet)
    echo "$file: $summary" >> summaries.txt
done
```

#### CI/CD Integration
```bash
# Code review automation
DIFF=$(git diff HEAD~1)
mcphost -p "Review this code diff and suggest improvements: $DIFF" --quiet

# Generate release notes
COMMITS=$(git log --oneline HEAD~10..HEAD)
mcphost -p "Generate release notes from these commits: $COMMITS" --quiet
```

#### Data Processing
```bash
# Process CSV data
mcphost -p "Analyze this CSV data and provide insights: $(cat data.csv)" --quiet

# Generate reports
mcphost -p "Create a summary report from this JSON: $(cat metrics.json)" --quiet
```

#### API Integration
```bash
# Use as a microservice
curl -X POST http://localhost:8080/process \
  -d "$(mcphost -p 'Generate a UUID' --quiet)"
```

### Tips for Scripting
- Use `--quiet` flag to get clean output suitable for parsing
- Combine with standard Unix tools (`grep`, `awk`, `sed`, etc.)
- Set appropriate timeouts for long-running operations
- Handle errors appropriately in your scripts
- Use environment variables for API keys in production

## MCP Server Compatibility 🔌

MCPHost can work with any MCP-compliant server. For examples and reference implementations, see the [MCP Servers Repository](https://github.com/modelcontextprotocol/servers).

## Contributing 🤝

Contributions are welcome! Feel free to:
- Submit bug reports or feature requests through issues
- Create pull requests for improvements
- Share your custom MCP servers
- Improve documentation

Please ensure your contributions follow good coding practices and include appropriate tests.

## License 📄

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments 🙏

- Thanks to the Anthropic team for Claude and the MCP specification
- Thanks to the Ollama team for their local LLM runtime
- Thanks to all contributors who have helped improve this tool
