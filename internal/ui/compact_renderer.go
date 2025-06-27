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
	label := lipgloss.NewStyle().Foreground(theme.Secondary).Bold(true).Render("User")
	
	// Format content for user messages (preserve formatting, no truncation)
	compactContent := r.formatUserAssistantContent(content)
	
	// Handle multi-line content
	lines := strings.Split(compactContent, "\n")
	var formattedLines []string
	
	for i, line := range lines {
		if i == 0 {
			// First line includes symbol and label
			formattedLines = append(formattedLines, fmt.Sprintf("%s  %s %s", symbol, label, line))
		} else {
			// Subsequent lines without indentation for compact mode
			formattedLines = append(formattedLines, line)
		}
	}
	
	return UIMessage{
		Type:      UserMessage,
		Content:   strings.Join(formattedLines, "\n"),
		Height:    len(formattedLines),
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
	label := lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(modelName)
	
	// Format content for assistant messages (preserve formatting, no truncation)
	compactContent := r.formatUserAssistantContent(content)
	if compactContent == "" {
		compactContent = lipgloss.NewStyle().Foreground(theme.Muted).Italic(true).Render("(no output)")
	}
	
	// Handle multi-line content
	lines := strings.Split(compactContent, "\n")
	var formattedLines []string
	
	for i, line := range lines {
		if i == 0 {
			// First line includes symbol and label
			formattedLines = append(formattedLines, fmt.Sprintf("%s  %s %s", symbol, label, line))
		} else {
			// Subsequent lines without indentation for compact mode
			formattedLines = append(formattedLines, line)
		}
	}
	
	return UIMessage{
		Type:      AssistantMessage,
		Content:   strings.Join(formattedLines, "\n"),
		Height:    len(formattedLines),
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
	if argsDisplay != "" {
		argsDisplay = lipgloss.NewStyle().Foreground(theme.Muted).Render(argsDisplay)
	}
	
	line := fmt.Sprintf("%s  %s %s", symbol, label, argsDisplay)
	
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
	var labelText string
	
	if isError {
		labelText = "Error"
		label = lipgloss.NewStyle().Foreground(theme.Muted).Bold(true).Render(labelText)
		content = lipgloss.NewStyle().Foreground(theme.Muted).Render(r.formatToolResult(toolResult))
	} else {
		// Determine result type based on tool and content
		labelText = r.determineResultType(toolName, toolResult)
		label = lipgloss.NewStyle().Foreground(theme.Muted).Bold(true).Render(labelText)
		content = lipgloss.NewStyle().Foreground(theme.Muted).Render(r.formatToolResult(toolResult))
		
		if r.formatToolResult(toolResult) == "" {
			content = lipgloss.NewStyle().Foreground(theme.Muted).Italic(true).Render("(no output)")
		}
	}
	
	// Handle multi-line tool results
	contentLines := strings.Split(content, "\n")
	var formattedLines []string
	
	for i, line := range contentLines {
		if i == 0 {
			// First line includes symbol and label
			formattedLines = append(formattedLines, fmt.Sprintf("%s  %s %s", symbol, label, line))
		} else {
			// Subsequent lines without indentation for compact mode
			formattedLines = append(formattedLines, line)
		}
	}
	
	return UIMessage{
		Type:    ToolMessage,
		Content: strings.Join(formattedLines, "\n"),
		Height:  len(formattedLines),
	}
}

// RenderSystemMessage renders a system message in compact format
func (r *CompactRenderer) RenderSystemMessage(content string, timestamp time.Time) UIMessage {
	theme := getTheme()
	symbol := lipgloss.NewStyle().Foreground(theme.System).Render("*")
	label := lipgloss.NewStyle().Foreground(theme.System).Bold(true).Render("System")
	
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
	maxLen := r.width - 28 // Reserve space for symbol and label more conservatively
	if maxLen < 40 {
		maxLen = 40 // Minimum width for readability
	}
	if !r.debug && len(content) > maxLen {
		content = content[:maxLen-3] + "..."
	}
	
	return content
}

// formatUserAssistantContent formats user and assistant content using glamour markdown rendering
func (r *CompactRenderer) formatUserAssistantContent(content string) string {
	if content == "" {
		return ""
	}

	// Calculate available width more conservatively
	// Account for: symbol (1) + spaces (2) + label (up to 20 chars) + space (1) + margin (4)
	availableWidth := r.width - 28
	if availableWidth < 40 {
		availableWidth = 40 // Minimum width for readability
	}

	// Use glamour to render markdown content with proper width
	rendered := toMarkdown(content, availableWidth)
	return strings.TrimSuffix(rendered, "\n")
}

// wrapText wraps text to the specified width, preserving existing line breaks
func (r *CompactRenderer) wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	lines := strings.Split(text, "\n")
	var wrappedLines []string

	for _, line := range lines {
		if len(line) <= width {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		// Wrap long lines
		words := strings.Fields(line)
		if len(words) == 0 {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		currentLine := ""
		for _, word := range words {
			// If adding this word would exceed the width, start a new line
			if len(currentLine)+len(word)+1 > width && currentLine != "" {
				wrappedLines = append(wrappedLines, currentLine)
				currentLine = word
			} else {
				if currentLine == "" {
					currentLine = word
				} else {
					currentLine += " " + word
				}
			}
		}
		if currentLine != "" {
			wrappedLines = append(wrappedLines, currentLine)
		}
	}

	return strings.Join(wrappedLines, "\n")
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
	
	// Remove parameter names (e.g., "command: ls" -> "ls", "path: /home" -> "/home")
	// Look for pattern "key: value" and extract just the value
	if colonIndex := strings.Index(args, ":"); colonIndex != -1 {
		args = strings.TrimSpace(args[colonIndex+1:])
	}
	
	return r.formatCompactContent(args)
}

// formatToolResult formats tool results preserving formatting but limiting to 5 lines
func (r *CompactRenderer) formatToolResult(result string) string {
	if result == "" {
		return ""
	}

	// Calculate available width more conservatively
	availableWidth := r.width - 28
	if availableWidth < 40 {
		availableWidth = 40 // Minimum width for readability
	}

	// First wrap the text to prevent long lines (tool results are usually plain text, not markdown)
	wrappedResult := r.wrapText(result, availableWidth)

	// Then limit to 5 lines
	lines := strings.Split(wrappedResult, "\n")
	if len(lines) > 5 {
		lines = lines[:5]
		// Add truncation indicator
		if len(lines) == 5 && lines[4] != "" {
			lines[4] = lines[4] + "..."
		} else {
			lines = append(lines, "...")
		}
	}

	return strings.Join(lines, "\n")
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