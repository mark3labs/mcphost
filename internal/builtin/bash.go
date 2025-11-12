package builtin

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	maxOutputLength = 30000
	defaultTimeout  = 2 * time.Minute
	maxTimeout      = 10 * time.Minute
)

var bannedCommands = []string{
	"alias",
	"curl",
	"curlie",
	"wget",
	"axel",
	"aria2c",
	"nc",
	"telnet",
	"lynx",
	"w3m",
	"links",
	"httpie",
	"xh",
	"http-prompt",
	"chrome",
	"firefox",
	"safari",
}

// NewBashServer creates a new MCP server that provides bash command execution capabilities.
// The server includes a single tool "run_shell_cmd" that executes shell commands with
// security restrictions, timeout controls, and output truncation. Returns an error if
// server initialization fails.
func NewBashServer() (*server.MCPServer, error) {
	s := server.NewMCPServer("bash-server", "1.0.0", server.WithToolCapabilities(true))

	// Register the run_shell_cmd tool using the builder pattern
	bashTool := mcp.NewTool("run_shell_cmd",
		mcp.WithDescription(bashDescription),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("The command to execute"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Optional timeout in milliseconds"),
			mcp.Min(0),
			mcp.Max(600000),
		),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("Clear, concise description of what this command does in 5-10 words. Examples:\nInput: ls\nOutput: Lists files in current directory\n\nInput: git status\nOutput: Shows working tree status\n\nInput: npm install\nOutput: Installs package dependencies\n\nInput: mkdir foo\nOutput: Creates directory 'foo'"),
		),
	)

	s.AddTool(bashTool, executeBash)

	return s, nil
}

// executeBash executes a bash command with security restrictions
func executeBash(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters using the helper methods
	command, err := request.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError("command parameter is required and must be a string"), nil
	}

	description, err := request.RequireString("description")
	if err != nil {
		return mcp.NewToolResultError("description parameter is required and must be a string"), nil
	}

	// Parse timeout (optional)
	timeout := defaultTimeout
	if timeoutMs := request.GetFloat("timeout", 0); timeoutMs > 0 {
		timeoutDuration := time.Duration(timeoutMs) * time.Millisecond
		if timeoutDuration > maxTimeout {
			timeout = maxTimeout
		} else {
			timeout = timeoutDuration
		}
	}

	// Check for banned commands
	for _, banned := range bannedCommands {
		if strings.HasPrefix(command, banned) {
			return mcp.NewToolResultError(fmt.Sprintf("Command '%s' is not allowed", command)), nil
		}
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute the command
	cmd := exec.CommandContext(cmdCtx, "bash", "-c", command)

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	// Truncate output if too long
	outputStr := string(output)
	if len(outputStr) > maxOutputLength {
		outputStr = outputStr[:maxOutputLength] + "\n... (output truncated)"
	}

	// Prepare the result
	var stdout, stderr string
	if err != nil {
		// If there's an error, treat the output as stderr
		stderr = outputStr
		if _, ok := err.(*exec.ExitError); ok {
			// Command ran but exited with non-zero status
			stdout = ""
		} else {
			// Command failed to start or other error
			stderr = fmt.Sprintf("Failed to execute command: %v\n%s", err, outputStr)
		}
	} else {
		// Command succeeded
		stdout = outputStr
		stderr = ""
	}

	// Format output similar to the TypeScript version
	result := fmt.Sprintf("<stdout>\n%s\n</stdout>\n<stderr>\n%s\n</stderr>", stdout, stderr)

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	// Create result with metadata
	toolResult := mcp.NewToolResultText(result)
	toolResult.Meta = &mcp.Meta{
		AdditionalFields: map[string]any{
			"stderr":      stderr,
			"stdout":      stdout,
			"exit":        exitCode,
			"description": description,
			"title":       command,
		},
	}
	return toolResult, nil
}

const bashDescription = `Executes a given bash command in a persistent shell session with optional timeout, ensuring proper handling and security measures.

Before executing the command, please follow these steps:

1. Directory Verification:
   - If the command will create new directories or files, first use the LS tool to verify the parent directory exists and is the correct location
   - For example, before running "mkdir foo/bar", first use LS to check that "foo" exists and is the intended parent directory

2. Command Execution:
   - Always quote file paths that contain spaces with double quotes (e.g., cd "path with spaces/file.txt")
   - Examples of proper quoting:
     - cd "/Users/name/My Documents" (correct)
     - cd /Users/name/My Documents (incorrect - will fail)
     - python "/path/with spaces/script.py" (correct)
     - python /path/with spaces/script.py (incorrect - will fail)
   - After ensuring proper quoting, execute the command.
   - Capture the output of the command.

Usage notes:
  - The command argument is required.
  - You can specify an optional timeout in milliseconds (up to 600000ms / 10 minutes). If not specified, commands will timeout after 120000ms (2 minutes).
  - It is very helpful if you write a clear, concise description of what this command does in 5-10 words.
  - If the output exceeds 30000 characters, output will be truncated before being returned to you.
  - VERY IMPORTANT: You MUST avoid using search commands like find and grep. Instead use Grep, Glob, or Task to search. You MUST avoid read tools like cat, head, tail, and ls, and use Read and LS to read files.
  - If you _still_ need to run grep, STOP. ALWAYS USE ripgrep at rg (or /usr/bin/rg) first, which all mcphost users have pre-installed.
  - When issuing multiple commands, use the ';' or '&&' operator to separate them. DO NOT use newlines (newlines are ok in quoted strings).
  - Try to maintain your current working directory throughout the session by using absolute paths and avoiding usage of cd. You may use cd if the User explicitly requests it.
    <good-example>
    pytest /foo/bar/tests
    </good-example>
    <bad-example>
    cd /foo/bar && pytest tests
    </bad-example>`
