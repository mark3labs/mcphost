package ui

import (
	"fmt"
	"strings"
	"time"
)

// CLIDebugLogger implements the tools.DebugLogger interface using CLI rendering.
// It provides debug logging functionality that integrates with the CLI's display
// system, ensuring debug messages are properly formatted and displayed alongside
// other conversation content.
type CLIDebugLogger struct {
	cli *CLI
}

// NewCLIDebugLogger creates and returns a new CLIDebugLogger instance that routes
// debug output through the provided CLI instance. The logger will respect the CLI's
// debug mode setting and display format preferences.
func NewCLIDebugLogger(cli *CLI) *CLIDebugLogger {
	return &CLIDebugLogger{cli: cli}
}

// LogDebug processes and displays a debug message through the CLI's rendering system.
// Messages are formatted with appropriate emojis and tags based on their content type
// (DEBUG, POOL, etc.) and only displayed when debug mode is enabled. The method handles
// multi-line debug output and connection pool status messages with context-aware formatting.
func (l *CLIDebugLogger) LogDebug(message string) {
	if l.cli == nil || !l.cli.debug {
		return
	}

	// Format the message to include all the debug info in a structured way
	var formattedMessage string

	// Check if this is a multi-line debug output (like connection info)
	if strings.Contains(message, "[DEBUG]") || strings.Contains(message, "[POOL]") {
		// Extract the tag and content
		if strings.HasPrefix(message, "[DEBUG]") {
			content := strings.TrimPrefix(message, "[DEBUG]")
			content = strings.TrimSpace(content)
			formattedMessage = fmt.Sprintf("🔍 DEBUG: %s", content)
		} else if strings.HasPrefix(message, "[POOL]") {
			content := strings.TrimPrefix(message, "[POOL]")
			content = strings.TrimSpace(content)

			// Add appropriate emoji based on the message content
			if strings.Contains(content, "Creating new connection") {
				formattedMessage = fmt.Sprintf("🆕 POOL: %s", content)
			} else if strings.Contains(content, "Created connection") || strings.Contains(content, "Initialized") {
				formattedMessage = fmt.Sprintf("✅ POOL: %s", content)
			} else if strings.Contains(content, "Reusing") {
				formattedMessage = fmt.Sprintf("🔄 POOL: %s", content)
			} else if strings.Contains(content, "unhealthy") || strings.Contains(content, "failed") {
				formattedMessage = fmt.Sprintf("❌ POOL: %s", content)
			} else if strings.Contains(content, "closed") {
				formattedMessage = fmt.Sprintf("🛑 POOL: %s", content)
			} else if strings.Contains(content, "Failed to close") {
				formattedMessage = fmt.Sprintf("⚠️ POOL: %s", content)
			} else {
				formattedMessage = fmt.Sprintf("🔍 POOL: %s", content)
			}
		} else {
			formattedMessage = message
		}
	} else {
		formattedMessage = message
	}

	// Use the CLI's debug message rendering
	var msg UIMessage
	if l.cli.compactMode {
		msg = l.cli.compactRenderer.RenderDebugMessage(formattedMessage, time.Now())
	} else {
		msg = l.cli.messageRenderer.RenderDebugMessage(formattedMessage, time.Now())
	}
	l.cli.messageContainer.AddMessage(msg)
	l.cli.displayContainer()
}

// IsDebugEnabled checks whether debug logging is currently active. Returns true
// if the CLI instance exists and has debug mode enabled, allowing callers to
// conditionally perform expensive debug operations only when necessary.
func (l *CLIDebugLogger) IsDebugEnabled() bool {
	return l.cli != nil && l.cli.debug
}
