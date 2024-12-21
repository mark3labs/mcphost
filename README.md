# MCPHost 🤖

A CLI host application that enables Large Language Models (LLMs) to interact with external tools through the Model Context Protocol (MCP). Currently supports both Claude 3.5 Sonnet and Ollama models.

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

## Features ✨

- Interactive conversations with either Claude 3.5 Sonnet or Ollama models
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

## Installation 📦

```bash
go install github.com/mark3labs/mcphost@latest
```

## Configuration ⚙️

MCPHost will automatically create a configuration file at `~/.mcp.json` if it doesn't exist. You can also specify a custom location using the `--config` flag:

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

Each MCP server entry requires:
- `command`: The command to run (e.g., `uvx`, `npx`) 
- `args`: Array of arguments for the command:
  - For SQLite server: `mcp-server-sqlite` with database path
  - For filesystem server: `@modelcontextprotocol/server-filesystem` with directory path

## Usage 🚀

MCPHost is a CLI tool that allows you to interact with various AI models through a unified interface. It supports various tools through MCP servers and provides streaming responses.

### Available Models
Models can be specified using the `--model` (`-m`) flag:
- Anthropic Claude (default): `anthropic:claude-3-5-sonnet-latest`
- OpenAI: `openai:gpt-4`
- Ollama models: `ollama:modelname`

### Examples
```bash
# Use Ollama with Qwen model
mcphost -m ollama:qwen2.5:3b

# Use OpenAI's GPT-4
mcphost -m openai:gpt-4
```

### Flags
- `--config string`: Config file location (default is $HOME/mcp.json)
- `--debug`: Enable debug logging
- `--message-window int`: Number of messages to keep in context (default: 10)
- `-m, --model string`: Model to use (format: provider:model) (default "anthropic:claude-3-5-sonnet-latest")
- `--openai-url string`: Base URL for OpenAI API (defaults to api.openai.com)

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

## 配置参数

可以通过以下方式配置 API 密钥：

1. 命令行参数:
   ```bash
   mcphost --openai-api-key=<your-openai-key> --anthropic-api-key=<your-anthropic-key>
   ```

2. 环境变量:
   ```bash
   export OPENAI_API_KEY=<your-openai-key>
   export ANTHROPIC_API_KEY=<your-anthropic-key>
   ```
