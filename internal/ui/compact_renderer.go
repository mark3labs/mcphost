package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// CompactRenderer handles rendering messages in compact format
type CompactRenderer struct {
	width int
	debug bool
}

// NewCompactRenderer creates a new compact message renderer
func NewCompactRenderer(width int, debug bool) *CompactRenderer {
	return &CompactRenderer{
		width: width,
		debug: debug,
	}
}

// SetWidth updates the renderer width
func (r *CompactRenderer) SetWidth(width int) {
	r.width = width
}

// RenderUserMessage renders a user message in compact format
func (r *CompactRenderer) RenderUserMessage(content string, timestamp time.Time) UIMessage {
	theme := getTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Secondary).Render(">")
	label := lipgloss.NewStyle().Foreground(theme.Muted).Bold(true).Render("User")
	
	// Format content for compact display
	compactContent := r.formatCompactContent(content)
	
	line := fmt.Sprintf("%s  %-8s %s", symbol, label, compactContent)
	
	return UIMessage{
		Type:      UserMessage,
		Content:   line,
		Height:    1,
		Timestamp: timestamp,
	}
}

// RenderAssistantMessage renders an assistant message in compact format
func (r *CompactRenderer) RenderAssistantMessage(content string, timestamp time.Time, modelName string) UIMessage {
	theme := getTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Primary).Render("<")
	
	// Use the full model name, fallback to "Assistant" if empty
	if modelName == "" {
		modelName = "Assistant"
	}
	label := lipgloss.NewStyle().Foreground(theme.Muted).Bold(true).Render(modelName)
	
	// Format content for compact display
	compactContent := r.formatCompactContent(content)
	if compactContent == "" {
		compactContent = lipgloss.NewStyle().Foreground(theme.Muted).Italic(true).Render("(no output)")
	}
	
	line := fmt.Sprintf("%s  %s %s", symbol, label, compactContent)
	
	return UIMessage{
		Type:      AssistantMessage,
		Content:   line,
		Height:    1,
		Timestamp: timestamp,
	}
}

// RenderToolCallMessage renders a tool call in progress in compact format
func (r *CompactRenderer) RenderToolCallMessage(toolName, toolArgs string, timestamp time.Time) UIMessage {
	theme := getTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Tool).Render("[")
	label := lipgloss.NewStyle().Foreground(theme.Tool).Bold(true).Render(toolName)
	
	// Format args for compact display
	argsDisplay := r.formatToolArgs(toolArgs)
	
	line := fmt.Sprintf("%s  %-8s %s", symbol, label, argsDisplay)
	
	return UIMessage{
		Type:      ToolCallMessage,
		Content:   line,
		Height:    1,
		Timestamp: timestamp,
	}
}

// RenderToolMessage renders a tool result in compact format
func (r *CompactRenderer) RenderToolMessage(toolName, toolArgs, toolResult string, isError bool) UIMessage {
	theme := getTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Muted).Render("]")
	
	// Determine result type and styling
	var label string
	var content string
	
	if isError {
		label = lipgloss.NewStyle().Foreground(theme.Error).Bold(true).Render("Error")
		content = lipgloss.NewStyle().Foreground(theme.Error).Render(r.formatCompactContent(toolResult))
	} else {
		// Determine result type based on tool and content
		resultType := r.determineResultType(toolName, toolResult)
		label = lipgloss.NewStyle().Foreground(theme.Muted).Bold(true).Render(resultType)
		content = r.formatCompactContent(toolResult)
		
		if content == "" {
			content = lipgloss.NewStyle().Foreground(theme.Muted).Italic(true).Render("(no output)")
		}
	}
	
	line := fmt.Sprintf("%s  %-8s %s", symbol, label, content)
	
	return UIMessage{
		Type:    ToolMessage,
		Content: line,
		Height:  1,
	}
}

// RenderSystemMessage renders a system message in compact format
func (r *CompactRenderer) RenderSystemMessage(content string, timestamp time.Time) UIMessage {
	theme := getTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.System).Render("*")
	label := lipgloss.NewStyle().Foreground(theme.Muted).Bold(true).Render("System")
	
	compactContent := r.formatCompactContent(content)
	
	line := fmt.Sprintf("%s  %-8s %s", symbol, label, compactContent)
	
	return UIMessage{
		Type:      SystemMessage,
		Content:   line,
		Height:    1,
		Timestamp: timestamp,
	}
}

// RenderErrorMessage renders an error message in compact format
func (r *CompactRenderer) RenderErrorMessage(errorMsg string, timestamp time.Time) UIMessage {
	theme := getTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Error).Render("!")
	label := lipgloss.NewStyle().Foreground(theme.Error).Bold(true).Render("Error")
	
	compactContent := lipgloss.NewStyle().Foreground(theme.Error).Render(r.formatCompactContent(errorMsg))
	
	line := fmt.Sprintf("%s  %-8s %s", symbol, label, compactContent)
	
	return UIMessage{
		Type:      ErrorMessage,
		Content:   line,
		Height:    1,
		Timestamp: timestamp,
	}
}

// RenderDebugConfigMessage renders debug config in compact format
func (r *CompactRenderer) RenderDebugConfigMessage(config map[string]any, timestamp time.Time) UIMessage {
	theme := getTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.Tool).Render("*")
	label := lipgloss.NewStyle().Foreground(theme.Tool).Bold(true).Render("Debug")
	
	// Format config as compact key=value pairs
	var configPairs []string
	for key, value := range config {
		if value != nil {
			configPairs = append(configPairs, fmt.Sprintf("%s=%v", key, value))
		}
	}
	
	content := strings.Join(configPairs, ", ")
	if len(content) > r.width-20 {
		content = content[:r.width-23] + "..."
	}
	
	line := fmt.Sprintf("%s  %-8s %s", symbol, label, content)
	
	return UIMessage{
		Type:      SystemMessage,
		Content:   line,
		Height:    1,
		Timestamp: timestamp,
	}
}

// formatCompactContent formats content for compact single-line display
func (r *CompactRenderer) formatCompactContent(content string) string {
	if content == "" {
		return ""
	}
	
	// Remove markdown formatting for compact display
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\t", " ")
	
	// Collapse multiple spaces
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}
	
	content = strings.TrimSpace(content)
	
	// Truncate if too long (unless in debug mode)
	maxLen := r.width - 20 // Reserve space for symbol and label
	if !r.debug && len(content) > maxLen {
		content = content[:maxLen-3] + "..."
	}
	
	return content
}

// formatToolArgs formats tool arguments for compact display
func (r *CompactRenderer) formatToolArgs(args string) string {
	if args == "" || args == "{}" {
		return ""
	}
	
	// Remove JSON braces and format compactly
	args = strings.TrimSpace(args)
	if strings.HasPrefix(args, "{") && strings.HasSuffix(args, "}") {
		args = strings.TrimPrefix(args, "{")
		args = strings.TrimSuffix(args, "}")
		args = strings.TrimSpace(args)
	}
	
	// Remove quotes around simple values
	args = strings.ReplaceAll(args, `"`, "")
	
	return r.formatCompactContent(args)
}

// determineResultType determines the display type for tool results
func (r *CompactRenderer) determineResultType(toolName, result string) string {
	toolName = strings.ToLower(toolName)
	
	switch {
	case strings.Contains(toolName, "read"):
		return "Text"
	case strings.Contains(toolName, "write"):
		return "Write"
	case strings.Contains(toolName, "bash") || strings.Contains(toolName, "command"):
		return "Bash"
	case strings.Contains(toolName, "list") || strings.Contains(toolName, "ls"):
		return "List"
	case strings.Contains(toolName, "search") || strings.Contains(toolName, "grep"):
		return "Search"
	case strings.Contains(toolName, "fetch") || strings.Contains(toolName, "http"):
		return "Fetch"
	default:
		return "Result"
	}
}