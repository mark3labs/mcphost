package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// MessageType represents the type of message
type MessageType int

const (
	UserMessage MessageType = iota
	AssistantMessage
	ToolMessage
	ToolCallMessage // New type for showing tool calls in progress
	SystemMessage   // New type for MCPHost system messages (help, tools, etc.)
	ErrorMessage    // New type for error messages
)

// UIMessage represents a rendered message for display
type UIMessage struct {
	ID        string
	Type      MessageType
	Position  int
	Height    int
	Content   string
	Timestamp time.Time
}

// Color constants
var (
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#06B6D4") // Cyan
	systemColor    = lipgloss.Color("#10B981") // Green for MCPHost system messages
	textColor      = lipgloss.Color("#FFFFFF") // White
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	errorColor     = lipgloss.Color("#EF4444") // Red
	toolColor      = lipgloss.Color("#F59E0B") // Orange/Amber for tool calls
)

// MessageRenderer handles rendering of messages with proper styling
type MessageRenderer struct {
	width int
}

// NewMessageRenderer creates a new message renderer
func NewMessageRenderer(width int) *MessageRenderer {
	return &MessageRenderer{
		width: width,
	}
}

// SetWidth updates the renderer width
func (r *MessageRenderer) SetWidth(width int) {
	r.width = width
}

// RenderUserMessage renders a user message with proper styling
func (r *MessageRenderer) RenderUserMessage(content string, timestamp time.Time) UIMessage {
	baseStyle := lipgloss.NewStyle()

	// Create the main message style with border
	style := baseStyle.
		Width(r.width - 1).
		BorderLeft(true).
		Foreground(mutedColor).
		BorderForeground(secondaryColor).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(1)

	// Format timestamp
	timeStr := timestamp.Local().Format("02 Jan 2006 03:04 PM")
	username := "You"

	// Create info line
	info := baseStyle.
		Width(r.width - 1).
		Foreground(mutedColor).
		Render(fmt.Sprintf(" %s (%s)", username, timeStr))

	// Render the message content
	messageContent := r.renderMarkdown(content, r.width-2)

	// Combine content and info
	parts := []string{
		strings.TrimSuffix(messageContent, "\n"),
		info,
	}

	rendered := style.Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)

	return UIMessage{
		Type:      UserMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderAssistantMessage renders an assistant message with proper styling
func (r *MessageRenderer) RenderAssistantMessage(content string, timestamp time.Time, modelName string) UIMessage {
	baseStyle := lipgloss.NewStyle()

	// Create the main message style with border
	style := baseStyle.
		Width(r.width - 1).
		BorderLeft(true).
		Foreground(mutedColor).
		BorderForeground(primaryColor).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(1)

	// Format timestamp and model info
	timeStr := timestamp.Local().Format("02 Jan 2006 03:04 PM")
	if modelName == "" {
		modelName = "Assistant"
	}

	// Create info line
	info := baseStyle.
		Width(r.width - 1).
		Foreground(mutedColor).
		Render(fmt.Sprintf(" %s (%s)", modelName, timeStr))

	// Render the message content
	messageContent := r.renderMarkdown(content, r.width-2)

	// Handle empty content
	if strings.TrimSpace(content) == "" {
		messageContent = baseStyle.
			Italic(true).
			Foreground(mutedColor).
			Render("*Finished without output*")
	}

	// Combine content and info
	parts := []string{
		strings.TrimSuffix(messageContent, "\n"),
		info,
	}

	rendered := style.Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)

	return UIMessage{
		Type:      AssistantMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderSystemMessage renders a system message (help, tools, etc.) with proper styling
func (r *MessageRenderer) RenderSystemMessage(content string, timestamp time.Time) UIMessage {
	baseStyle := lipgloss.NewStyle()

	// Create the main message style with border
	style := baseStyle.
		Width(r.width - 1).
		BorderLeft(true).
		Foreground(mutedColor).
		BorderForeground(systemColor).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(1)

	// Format timestamp
	timeStr := timestamp.Local().Format("02 Jan 2006 03:04 PM")

	// Create info line with MCPHost label
	info := baseStyle.
		Width(r.width - 1).
		Foreground(mutedColor).
		Render(fmt.Sprintf(" MCPHost (%s)", timeStr))

	// Render the message content with markdown
	messageContent := r.renderMarkdown(content, r.width-2)

	// Handle empty content
	if strings.TrimSpace(content) == "" {
		messageContent = baseStyle.
			Italic(true).
			Foreground(mutedColor).
			Render("*No content*")
	}

	// Combine content and info
	parts := []string{
		strings.TrimSuffix(messageContent, "\n"),
		info,
	}

	rendered := style.Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)

	return UIMessage{
		Type:      SystemMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderDebugConfigMessage renders debug configuration settings with tool response block styling
func (r *MessageRenderer) RenderDebugConfigMessage(config map[string]any, timestamp time.Time) UIMessage {
	baseStyle := lipgloss.NewStyle()

	// Create the main message style with border using tool color
	style := baseStyle.
		Width(r.width - 1).
		BorderLeft(true).
		Foreground(mutedColor).
		BorderForeground(toolColor).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(1)

	// Format timestamp
	timeStr := timestamp.Local().Format("02 Jan 2006 03:04 PM")

	// Create header with debug icon
	header := baseStyle.
		Foreground(toolColor).
		Bold(true).
		Render("🔧 Debug Configuration")

	// Format configuration settings
	var configLines []string
	for key, value := range config {
		if value != nil {
			configLines = append(configLines, fmt.Sprintf("  %s: %v", key, value))
		}
	}

	configContent := baseStyle.
		Foreground(mutedColor).
		Render(strings.Join(configLines, "\n"))

	// Create info line
	info := baseStyle.
		Width(r.width - 1).
		Foreground(mutedColor).
		Render(fmt.Sprintf(" MCPHost (%s)", timeStr))

	// Combine parts
	parts := []string{header}
	if len(configLines) > 0 {
		parts = append(parts, configContent)
	}
	parts = append(parts, info)

	rendered := style.Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)

	return UIMessage{
		Type:      SystemMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderErrorMessage renders an error message with proper styling
func (r *MessageRenderer) RenderErrorMessage(errorMsg string, timestamp time.Time) UIMessage {
	baseStyle := lipgloss.NewStyle()

	// Create the main message style with border
	style := baseStyle.
		Width(r.width - 1).
		BorderLeft(true).
		Foreground(mutedColor).
		BorderForeground(errorColor).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(1)

	// Format timestamp
	timeStr := timestamp.Local().Format("02 Jan 2006 03:04 PM")

	// Create info line with Error label
	info := baseStyle.
		Width(r.width - 1).
		Foreground(mutedColor).
		Render(fmt.Sprintf(" Error (%s)", timeStr))

	// Format error content with error styling
	errorContent := baseStyle.
		Foreground(errorColor).
		Bold(true).
		Render(fmt.Sprintf("❌ %s", errorMsg))

	// Combine content and info
	parts := []string{
		errorContent,
		info,
	}

	rendered := style.Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)

	return UIMessage{
		Type:      ErrorMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderToolCallMessage renders a tool call in progress with proper styling
func (r *MessageRenderer) RenderToolCallMessage(toolName, toolArgs string, timestamp time.Time) UIMessage {
	baseStyle := lipgloss.NewStyle()

	// Create the main message style with border
	style := baseStyle.
		Width(r.width - 1).
		BorderLeft(true).
		Foreground(mutedColor).
		BorderForeground(toolColor).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(1)

	// Format timestamp
	timeStr := timestamp.Local().Format("02 Jan 2006 03:04 PM")

	// Create header with tool icon and name
	toolIcon := "🔧"
	header := baseStyle.
		Foreground(toolColor).
		Bold(true).
		Render(fmt.Sprintf("%s Calling %s", toolIcon, toolName))

	// Format arguments in a more readable way
	var argsContent string
	if toolArgs != "" && toolArgs != "{}" {
		// Try to format JSON args nicely
		argsContent = baseStyle.
			Foreground(mutedColor).
			Render(fmt.Sprintf("Arguments: %s", r.formatToolArgs(toolArgs)))
	}

	// Create info line
	info := baseStyle.
		Width(r.width - 1).
		Foreground(mutedColor).
		Render(fmt.Sprintf(" Tool Call (%s)", timeStr))

	// Combine parts
	parts := []string{header}
	if argsContent != "" {
		parts = append(parts, argsContent)
	}
	parts = append(parts, info)

	rendered := style.Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)

	return UIMessage{
		Type:      ToolCallMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderToolMessage renders a tool call message with proper styling
func (r *MessageRenderer) RenderToolMessage(toolName, toolArgs, toolResult string, isError bool) UIMessage {
	baseStyle := lipgloss.NewStyle()

	// Create the main message style with border
	style := baseStyle.
		Width(r.width - 1).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(1).
		BorderForeground(mutedColor)

	// Tool name styling
	toolNameText := baseStyle.
		Foreground(mutedColor).
		Render(fmt.Sprintf("%s: ", toolName))

	// Tool arguments styling
	argsText := baseStyle.
		Width(r.width - 2 - lipgloss.Width(toolNameText)).
		Foreground(mutedColor).
		Render(r.truncateText(toolArgs, r.width-2-lipgloss.Width(toolNameText)))

	// Tool result styling
	var resultContent string
	if isError {
		resultContent = baseStyle.
			Width(r.width - 2).
			Foreground(errorColor).
			Render(fmt.Sprintf("Error: %s", toolResult))
	} else {
		// Format result based on tool type
		resultContent = r.formatToolResult(toolName, toolResult, r.width-2)
	}

	// Combine parts
	headerLine := lipgloss.JoinHorizontal(lipgloss.Left, toolNameText, argsText)
	parts := []string{headerLine}

	if resultContent != "" {
		parts = append(parts, strings.TrimSuffix(resultContent, "\n"))
	}

	rendered := style.Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)

	return UIMessage{
		Type:    ToolMessage,
		Content: rendered,
		Height:  lipgloss.Height(rendered),
	}
}

// formatToolArgs formats tool arguments for display
func (r *MessageRenderer) formatToolArgs(args string) string {
	// Remove outer braces and clean up JSON formatting
	args = strings.TrimSpace(args)
	if strings.HasPrefix(args, "{") && strings.HasSuffix(args, "}") {
		args = strings.TrimPrefix(args, "{")
		args = strings.TrimSuffix(args, "}")
		args = strings.TrimSpace(args)
	}

	// If it's empty after cleanup, return a placeholder
	if args == "" {
		return "(no arguments)"
	}

	// Truncate if too long
	maxLen := 100
	if len(args) > maxLen {
		return args[:maxLen] + "..."
	}

	return args
}

// formatToolResult formats tool results based on tool type
func (r *MessageRenderer) formatToolResult(toolName, result string, width int) string {
	baseStyle := lipgloss.NewStyle()

	// Truncate very long results
	maxLines := 10
	lines := strings.Split(result, "\n")
	if len(lines) > maxLines {
		result = strings.Join(lines[:maxLines], "\n") + "\n... (truncated)"
	}

	// Format as code block for most tools
	if strings.Contains(toolName, "bash") || strings.Contains(toolName, "command") {
		formatted := fmt.Sprintf("```bash\n%s\n```", result)
		return r.renderMarkdown(formatted, width)
	}

	// For other tools, render as muted text
	return baseStyle.
		Width(width).
		Foreground(mutedColor).
		Render(result)
}

// truncateText truncates text to fit within the specified width
func (r *MessageRenderer) truncateText(text string, maxWidth int) string {
	// Replace newlines with spaces for single-line display
	text = strings.ReplaceAll(text, "\n", " ")

	if lipgloss.Width(text) <= maxWidth {
		return text
	}

	// Simple truncation - could be improved with proper unicode handling
	for i := len(text) - 1; i >= 0; i-- {
		truncated := text[:i] + "..."
		if lipgloss.Width(truncated) <= maxWidth {
			return truncated
		}
	}

	return "..."
}

// renderMarkdown renders markdown content using glamour
func (r *MessageRenderer) renderMarkdown(content string, width int) string {
	rendered := toMarkdown(content, width)
	return strings.TrimSuffix(rendered, "\n")
}

// MessageContainer wraps multiple messages in a container
type MessageContainer struct {
	messages []UIMessage
	width    int
	height   int
}

// NewMessageContainer creates a new message container
func NewMessageContainer(width, height int) *MessageContainer {
	return &MessageContainer{
		messages: make([]UIMessage, 0),
		width:    width,
		height:   height,
	}
}

// AddMessage adds a message to the container
func (c *MessageContainer) AddMessage(msg UIMessage) {
	c.messages = append(c.messages, msg)
}

// Clear clears all messages from the container
func (c *MessageContainer) Clear() {
	c.messages = make([]UIMessage, 0)
}

// SetSize updates the container size
func (c *MessageContainer) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// Render renders all messages in the container
func (c *MessageContainer) Render() string {
	if len(c.messages) == 0 {
		return c.renderEmptyState()
	}

	baseStyle := lipgloss.NewStyle()
	var parts []string

	for _, msg := range c.messages {
		parts = append(parts, msg.Content)
		// Add spacing between messages
		parts = append(parts, baseStyle.Width(c.width).Render(""))
	}

	return baseStyle.
		Width(c.width).
		PaddingBottom(1).
		Render(
			lipgloss.JoinVertical(lipgloss.Top, parts...),
		)
}

// renderEmptyState renders the initial empty state
func (c *MessageContainer) renderEmptyState() string {
	baseStyle := lipgloss.NewStyle()

	header := baseStyle.
		Width(c.width).
		Align(lipgloss.Center).
		Foreground(systemColor).
		Bold(true).
		Render("MCPHost - AI Assistant with MCP Tools")

	subtitle := baseStyle.
		Width(c.width).
		Align(lipgloss.Center).
		Foreground(mutedColor).
		Render("Start a conversation by typing your message below")

	return baseStyle.
		Width(c.width).
		Height(c.height).
		PaddingBottom(1).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Center,
				"",
				header,
				"",
				subtitle,
				"",
			),
		)
}
