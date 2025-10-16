<!-- OPENSPEC:START -->
# OpenSpec Instructions

These instructions are for AI assistants working in this project.

Always open `@/openspec/AGENTS.md` when the request:
- Mentions planning or proposals (words like proposal, spec, change, plan)
- Introduces new capabilities, breaking changes, architecture shifts, or big performance/security work
- Sounds ambiguous and you need the authoritative spec before coding

Use `@/openspec/AGENTS.md` to learn:
- How to create and apply change proposals
- Spec format and conventions
- Project structure and guidelines

Keep this managed block so 'openspec update' can refresh the instructions.

<!-- OPENSPEC:END -->

# MCPHost Agent Guidelines

## Build/Test Commands
- **Build**: `go build -o output/mcphost`
- **Test all**: `go test -race ./...`
- **Test single**: `go test -race ./cmd -run TestScriptExecution`
- **Lint**: `go vet ./...`
- **Format**: `go fmt ./...`

## Code Style
- **Imports**: stdlib → third-party → local (blank lines between)
- **Naming**: camelCase (unexported), PascalCase (exported)
- **Errors**: Always check, wrap with `fmt.Errorf("context: %w", err)`
- **Logging**: Use `github.com/charmbracelet/log` structured logging
- **Types**: Prefer `any` over `interface{}`
- **JSON**: snake_case tags with `omitempty` where appropriate
- **Context**: First parameter for blocking operations

## Architecture
- Multi-provider LLM support via `llm.Provider` interface
- MCP client-server for tool integration
- Builtin servers: bash, fetch, todo, fs