# MCPHost Development Context

## Build/Test Commands
- **Build**: `go build -o output/mcphost` or use `./contribute/build.sh`
- **Test**: `go test ./...` (run all tests)
- **Test single package**: `go test ./cmd` (test script functionality)
- **Lint**: `go vet ./...` (built-in Go linter)
- **Format**: `go fmt ./...`
- **Dependencies**: `go mod tidy`

## UI and Output Modes
MCPHost supports multiple output modes for different use cases:

### Standard Mode (Default)
- Full-featured terminal UI with rich styling and formatting
- Interactive message display with proper spacing and visual hierarchy
- Suitable for regular interactive usage

### Compact Mode (`--compact`)
- **Location**: `internal/ui/compact_renderer.go`, `internal/ui/cli.go`
- **Purpose**: Simplified output format without fancy styling for better readability in automation contexts
- **Features**: 
  - Single-line message format where possible
  - Minimal visual styling and spacing
  - Consistent symbol-based prefixes (🧑, 🤖, 🔧, etc.)
  - Optimized for scripting and log parsing
- **Usage**: Add `--compact` flag to any mcphost command
- **Note**: Has no effect when combined with `--quiet` (mutually exclusive)

### Quiet Mode (`--quiet`)
- Suppresses all UI elements except the AI response
- Only works with `--prompt` (non-interactive mode)
- Ideal for shell scripting and piping output to other commands
- **Note**: When `--quiet` is used, `--compact` has no effect since no UI elements are shown

## Code Style Guidelines
- **Package structure**: `pkg/` for reusable packages, `cmd/` for CLI commands
- **Imports**: Standard library first, then third-party, then local packages with blank lines between groups
- **Naming**: Use camelCase for unexported, PascalCase for exported; descriptive names (e.g., `CreateMessage`, `mcpClients`)
- **Interfaces**: Keep small and focused (e.g., `llm.Provider`, `llm.Message`)
- **Error handling**: Always check errors, wrap with context using `fmt.Errorf("context: %v", err)`
- **Logging**: Use `github.com/charmbracelet/log` with structured logging: `log.Info("message", "key", value)`
- **Types**: Prefer `any` over `interface{}` (modernize hint from linter)
- **JSON tags**: Use snake_case for JSON fields, include omitempty where appropriate
- **Context**: Always pass `context.Context` as first parameter for operations that may block

## Architecture Notes
- Multi-provider LLM support (Anthropic, OpenAI, Ollama, Google)
- MCP (Model Context Protocol) client-server architecture for tool integration
- Provider pattern for LLM abstraction with common `llm.Provider` interface
- History management with message pruning based on configurable window size
- Tool calling support across all providers with unified `llm.Tool` interface

## MCP Configuration Schema
MCPHost supports a simplified configuration schema with three server types:

### New Simplified Format
- **Local servers** (`"type": "local"`): Run commands locally via stdio transport
  - `command`: Array of command and arguments (e.g., `["npx", "server", "args"]`)
  - `environment`: Key-value map of environment variables
- **Remote servers** (`"type": "remote"`): Connect via StreamableHTTP transport
  - `url`: Server endpoint URL
  - Automatically uses StreamableHTTP for optimal performance
- **Builtin servers** (`"type": "builtin"`): Run in-process for optimal performance
  - `name`: Internal name of the builtin server (e.g., `"fs"`)
  - `options`: Configuration options specific to the builtin server

### Legacy Format Support
- Maintains full backward compatibility with existing configurations
- Automatic detection and conversion of legacy formats
- Custom `UnmarshalJSON` method handles format migration seamlessly
- **Bug Fix**: Improved stdio transport reliability for legacy configurations with external processes (Docker, NPX, etc.)

### Transport Mapping
- `"local"` type → `stdio` transport (launches local processes)
- `"remote"` type → `streamable` transport (StreamableHTTP protocol)
- `"builtin"` type → `inprocess` transport (in-process execution)
- Legacy `transport` field still supported for backward compatibility

### Configuration Files
- Primary: `~/.mcphost.yml` or `~/.mcphost.json`
- Legacy: `~/.mcp.yml` or `~/.mcp.json`
- Custom location via `--config` flag

## Available Builtin Servers
MCPHost includes several builtin MCP servers for common functionality:

### Filesystem Server (`fs`)
- **Location**: `internal/builtin/registry.go` (registration), uses external `mcp-filesystem-server`
- **Purpose**: Secure filesystem access with configurable allowed directories
- **Options**: `allowed_directories` array (defaults to current working directory)

### Bash Server (`bash`)
- **Location**: `internal/builtin/bash.go`
- **Purpose**: Execute bash commands with security restrictions and timeout controls
- **Features**: Banned commands list, configurable timeouts, output size limits
- **Security**: Blocks network commands (curl, wget, etc.), 30KB output limit, 2-10 minute timeouts

### Todo Server (`todo`)
- **Location**: `internal/builtin/todo.go`
- **Purpose**: Manage ephemeral todo lists for task tracking during sessions
- **Features**: In-memory storage, thread-safe operations, JSON-based API
- **Tools**: `todowrite` (create/update todos), `todoread` (read current todos)

### Fetch Server (`fetch`)
- **Location**: `internal/builtin/fetch.go`
- **Purpose**: Fetch web content and convert to text, markdown, or HTML formats
- **Features**: HTTP/HTTPS support, HTML parsing, markdown conversion, size limits
- **Security**: 5MB response limit, configurable timeouts, localhost-aware HTTPS upgrade

## Environment Variable Substitution System
MCPHost includes a comprehensive environment variable substitution system for secure configuration management.

### Implementation Details
- **Location**: `internal/config/substitution.go` (core logic), `internal/config/substitution_test.go` (unit tests)
- **Purpose**: Replace `${env://VAR}` and `${env://VAR:-default}` patterns with environment variable values
- **Integration**: Works in both config files and script frontmatter/prompts

### Substitution Components
- **EnvSubstituter**: Handles environment variable substitution
- **ArgsSubstituter**: Handles script argument substitution (refactored from existing code)
- **Shared Parsing Logic**: `parseVariableWithDefault()` function used by both substituters

### Processing Order
1. **Config Loading**: `cmd/root.go` → `loadConfigWithEnvSubstitution()` → env substitution → YAML/JSON parsing
2. **Script Mode**: `cmd/script.go` → `parseScriptContent()` → env substitution → YAML parsing → args substitution

### Security Features
- **No Shell Execution**: Direct environment variable lookup using `os.Getenv()`
- **Error Handling**: Clear error messages for missing required variables
- **Validation**: Regex-based pattern matching with comprehensive validation

## Script System
MCPHost includes a powerful script system for automation and reusable workflows.

### Script Features
- **Location**: `cmd/script.go` (main implementation), `cmd/script_test.go` (comprehensive tests)
- **Purpose**: Execute YAML-based automation scripts with dual variable substitution
- **Format**: YAML frontmatter + prompt content in single executable files

### Variable Substitution System
- **Environment Variables**: `${env://VAR}` and `${env://VAR:-default}` - Processed first
- **Script Arguments**: `${variable}` and `${variable:-default}` - Processed after environment variables
- **Features**: 
  - Bash-style default syntax for familiarity
  - Empty defaults supported: `${var:-}` and `${env://VAR:-}`
  - Complex defaults: paths, URLs, commands with spaces
  - Full backward compatibility with existing scripts
- **Implementation**: Dual-phase substitution with shared parsing logic

### Script Examples
- **Location**: `examples/scripts/` directory
- **Demo Script**: `default-values-demo.sh` - Showcases script argument default values
- **Env Substitution Script**: `env-substitution-script.sh` - Demonstrates environment variable usage
- **Usage Examples**: Multiple scenarios from simple defaults to complex environment/args combinations

### Testing
- **Unit Tests**: 25+ test cases in `internal/config/substitution_test.go` covering all substitution scenarios
- **Integration Tests**: `internal/config/integration_test.go` and `cmd/script_integration_test.go`
- **Edge Cases**: Empty defaults, complex values, mixed required/optional variables, processing order
- **Backward Compatibility**: Ensures existing scripts and configs continue working unchanged

## Authentication System
MCPHost includes optional OAuth authentication for Anthropic Claude as an alternative to API keys.

### Authentication Commands
- **Location**: `cmd/auth.go` (main implementation)
- **Purpose**: Manage Anthropic OAuth credentials (alternative to API keys)
- **Commands**: `login anthropic`, `logout anthropic`, `status`

### OAuth Implementation
- **Location**: `internal/auth/oauth.go` (OAuth client), `internal/auth/credentials.go` (credential management)
- **Features**: PKCE security, automatic token refresh, encrypted storage, browser-based flow
- **Priority**: OAuth credentials > API keys (environment variables/flags)

## Recent Features

### Environment Variable Substitution (New)
- **Feature**: Added support for `${env://VAR}` and `${env://VAR:-default}` syntax in config files and scripts
- **Implementation**: 
  - `internal/config/substitution.go`: Core substitution logic with shared parsing
  - `cmd/root.go`: Config loading integration with `loadConfigWithEnvSubstitution()`
  - `cmd/script.go`: Script parsing integration with dual-phase substitution
- **Security**: Environment variables processed safely without shell execution
- **Compatibility**: Full backward compatibility with existing configurations and scripts
- **Testing**: Comprehensive test suite with 25+ unit tests and integration tests

### Processing Flow
1. **Config Files**: Raw content → Env substitution → YAML/JSON parsing → Viper config
2. **Scripts**: Raw content → Env substitution → YAML parsing → Args substitution → Final config

## Recent Bug Fixes

### Legacy MCP Server Configuration Fix
- **Issue**: Legacy stdio transport configurations (using `command` + `args` format) were failing with timeout errors
- **Root Cause**: MCP client creation was not properly handling legacy argument parsing when `Command` array was incomplete
- **Fix**: Added fallback logic in `internal/tools/mcp.go` to use legacy `Args` field when `Command` array only contains the command
- **Impact**: Fixes Docker-based MCP servers, NPX-based servers, and other external process configurations
- **Files Modified**: 
  - `internal/tools/mcp.go`: Added legacy args fallback logic
  - `internal/config/config.go`: Enhanced headers support for remote servers
- **Testing**: Verified with both Docker (`ghcr.io/mark3labs/phalcon-mcp:latest`) and NPX (`@modelcontextprotocol/server-filesystem`) servers

### MCP Client Initialization Improvements
- **Timeout**: Increased initialization timeout from 10s to 30s for slower external processes
- **Capabilities**: Added explicit `ClientCapabilities{}` to initialization request for better compatibility
- **Headers**: Enhanced SSE transport to support custom headers for authentication