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
- Support for multiple concurrent MCP servers
- Dynamic tool discovery and integration
- Tool calling capabilities for both model types
- Configurable MCP server locations and arguments
- Consistent command interface across model types
- Configurable message history window for context management

## Requirements 📋

- Go 1.23 or later
- For Claude: An Anthropic API key
- For Ollama: Local Ollama installation with desired models
- For Google/Gemini: Google API key (see https://aistudio.google.com/app/apikey)
- One or more MCP-compatible tool servers

## Environment Setup 🔧

1. Anthropic API Key (for Claude):
```bash
export ANTHROPIC_API_KEY='your-api-key'
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

4. OpenAI compatible online Setup
- Get your api server base url, api key and model name

## Installation 📦

```bash
go install github.com/mark3labs/mcphost@latest
```

## Configuration ⚙️

### MCP-server
MCPHost will automatically create a configuration file at `~/.mcp.json` if it doesn't exist. You can also specify a custom location using the `--config` flag.

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
      ]
    }
  }
}
```

Each STDIO entry requires:
- `command`: The command to run (e.g., `uvx`, `npx`) 
- `args`: Array of arguments for the command:
  - For SQLite server: `mcp-server-sqlite` with database path
  - For filesystem server: `@modelcontextprotocol/server-filesystem` with directory path

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

### System-Prompt

You can specify a custom system prompt using the `--system-prompt` flag. The system prompt should be a JSON file containing the instructions and context you want to provide to the model. For example:

```json
{
    "systemPrompt": "You're a cat. Name is Neko"
}
```

Usage:
```bash
mcphost --system-prompt ./my-system-prompt.json
```


## Usage 🚀

MCPHost is a CLI tool that allows you to interact with various AI models through a unified interface. It supports various tools through MCP servers.

### Available Models
Models can be specified using the `--model` (`-m`) flag:
- Anthropic Claude (default): `anthropic:claude-3-5-sonnet-latest`
- OpenAI or OpenAI-compatible: `openai:gpt-4`
- Ollama models: `ollama:modelname`
- Google: `google:gemini-2.0-flash`

### Examples
```bash
# Use Ollama with Qwen model
mcphost -m ollama:qwen2.5:3b

# Use OpenAI's GPT-4
mcphost -m openai:gpt-4

# Use OpenAI-compatible model
mcphost --model openai:<your-model-name> \
--openai-url <your-base-url> \
--openai-api-key <your-api-key>
```

### Flags
- `--anthropic-url string`: Base URL for Anthropic API (defaults to api.anthropic.com)
- `--anthropic-api-key string`: Anthropic API key (can also be set via ANTHROPIC_API_KEY environment variable)
- `--config string`: Config file location (default is $HOME/.mcp.json)
- `--system-prompt string`: system-prompt file location
- `--debug`: Enable debug logging
- `--message-window int`: Number of messages to keep in context (default: 10)
- `-m, --model string`: Model to use (format: provider:model) (default "anthropic:claude-3-5-sonnet-latest")
- `--openai-url string`: Base URL for OpenAI API (defaults to api.openai.com)
- `--openai-api-key string`: OpenAI API key (can also be set via OPENAI_API_KEY environment variable)
- `--google-api-key string`: Google API key (can also be set via GOOGLE_API_KEY environment variable)


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
- `--message-window`: Set number of messages to keep in context (default: 10)

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
