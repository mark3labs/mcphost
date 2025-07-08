package ui

import (
	"fmt"
	"os"
	"os/user"
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
	Streaming bool
}

// Helper functions to get theme colors
func getTheme() Theme {
	return GetTheme()
}

// MessageRenderer handles rendering of messages with proper styling
type MessageRenderer struct {
	width int
	debug bool
}

// getSystemUsername returns the current system username, fallback to "User"
func getSystemUsername() string {
	if currentUser, err := user.Current(); err == nil && currentUser.Username != "" {
		return currentUser.Username
	}
	// Fallback to environment variable
	if username := os.Getenv("USER"); username != "" {
		return username
	}
	if username := os.Getenv("USERNAME"); username != "" {
		return username
	}
	return "User"
}

// NewMessageRenderer creates a new message renderer
func NewMessageRenderer(width int, debug bool) *MessageRenderer {
	return &MessageRenderer{
		width: width,
		debug: debug,
	}
}

// SetWidth updates the renderer width
func (r *MessageRenderer) SetWidth(width int) {
	r.width = width
}

// RenderUserMessage renders a user message with right border and background header
func (r *MessageRenderer) RenderUserMessage(content string, timestamp time.Time) UIMessage {
	// Format timestamp and username
	timeStr := timestamp.Local().Format("15:04")
	username := getSystemUsername()

	// Render the message content
	messageContent := r.renderMarkdown(content, r.width-8) // Account for padding and borders

	// Create info line
	info := fmt.Sprintf(" %s (%s)", username, timeStr)

	// Combine content and info
	theme := getTheme()
	fullContent := strings.TrimSuffix(messageContent, "\n") + "\n" +
		lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)

	// Use the new block renderer
	rendered := renderContentBlock(
		fullContent,
		r.width,
		WithAlign(lipgloss.Right),
		WithBorderColor(theme.Secondary),
		WithMarginBottom(1),
	)

	return UIMessage{
		Type:      UserMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderAssistantMessage renders an assistant message with left border and background header
func (r *MessageRenderer) RenderAssistantMessage(content string, timestamp time.Time, modelName string) UIMessage {
	// Format timestamp and model info with better defaults
	timeStr := timestamp.Local().Format("15:04")
	if modelName == "" {
		modelName = "Assistant"
	}

	// Handle empty content with better styling
	theme := getTheme()
	var messageContent string
	if strings.TrimSpace(content) == "" {
		messageContent = lipgloss.NewStyle().
			Italic(true).
			Foreground(theme.Muted).
			Align(lipgloss.Center).
			Render("Finished without output")
	} else {
		messageContent = r.renderMarkdown(content, r.width-8) // Account for padding and borders
	}

	// Create info line
	info := fmt.Sprintf(" %s (%s)", modelName, timeStr)

	// Combine content and info
	fullContent := strings.TrimSuffix(messageContent, "\n") + "\n" +
		lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)

	// Use the new block renderer
	rendered := renderContentBlock(
		fullContent,
		r.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(theme.Primary),
		WithMarginBottom(1),
	)

	return UIMessage{
		Type:      AssistantMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderSystemMessage renders a system message with left border and background header
func (r *MessageRenderer) RenderSystemMessage(content string, timestamp time.Time) UIMessage {
	// Format timestamp
	timeStr := timestamp.Local().Format("15:04")

	// Handle empty content with better styling
	theme := getTheme()
	var messageContent string
	if strings.TrimSpace(content) == "" {
		messageContent = lipgloss.NewStyle().
			Italic(true).
			Foreground(theme.Muted).
			Align(lipgloss.Center).
			Render("No content available")
	} else {
		messageContent = r.renderMarkdown(content, r.width-8) // Account for padding and borders
	}

	// Create info line
	info := fmt.Sprintf(" MCPHost System (%s)", timeStr)

	// Combine content and info
	fullContent := strings.TrimSuffix(messageContent, "\n") + "\n" +
		lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)

	// Use the new block renderer
	rendered := renderContentBlock(
		fullContent,
		r.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(theme.System),
		WithMarginBottom(1),
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
	theme := getTheme()
	style := baseStyle.
		Width(r.width - 1).
		BorderLeft(true).
		Foreground(theme.Muted).
		BorderForeground(theme.Tool).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(1)

	// Format timestamp
	timeStr := timestamp.Local().Format("02 Jan 2006 03:04 PM")

	// Create header with debug icon
	header := baseStyle.
		Foreground(theme.Tool).
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
		Foreground(theme.Muted).
		Render(strings.Join(configLines, "\n"))

	// Create info line
	info := baseStyle.
		Width(r.width - 1).
		Foreground(theme.Muted).
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

// RenderErrorMessage renders an error message with left border and background header
func (r *MessageRenderer) RenderErrorMessage(errorMsg string, timestamp time.Time) UIMessage {
	// Format timestamp
	timeStr := timestamp.Local().Format("15:04")

	// Format error content
	theme := getTheme()
	errorContent := lipgloss.NewStyle().
		Foreground(theme.Error).
		Bold(true).
		Render(errorMsg)

	// Create info line
	info := fmt.Sprintf(" Error (%s)", timeStr)

	// Combine content and info
	fullContent := errorContent + "\n" +
		lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)

	// Use the new block renderer
	rendered := renderContentBlock(
		fullContent,
		r.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(theme.Error),
		WithMarginBottom(1),
	)

	return UIMessage{
		Type:      ErrorMessage,
		Content:   rendered,
		Height:    lipgloss.Height(rendered),
		Timestamp: timestamp,
	}
}

// RenderToolCallMessage renders a tool call in progress with left border and background header
func (r *MessageRenderer) RenderToolCallMessage(toolName, toolArgs string, timestamp time.Time) UIMessage {
	// Format timestamp
	timeStr := timestamp.Local().Format("15:04")

	// Format arguments with better presentation
	theme := getTheme()
	var argsContent string
	if toolArgs != "" && toolArgs != "{}" {
		argsContent = lipgloss.NewStyle().
			Foreground(theme.Muted).
			Italic(true).
			Render(fmt.Sprintf("Arguments: %s", r.formatToolArgs(toolArgs)))
	}

	// Create info line
	info := fmt.Sprintf(" Executing %s (%s)", toolName, timeStr)

	// Combine parts
	var fullContent string
	if argsContent != "" {
		fullContent = argsContent + "\n" +
			lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)
	} else {
		fullContent = lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(info)
	}

	// Use the new block renderer
	rendered := renderContentBlock(
		fullContent,
		r.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(theme.Tool),
		WithMarginBottom(1),
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
	theme := getTheme()

	// Tool result styling - no header since command is already shown in "Executing" message
	var fullContent string
	if isError {
		fullContent = lipgloss.NewStyle().
			Foreground(theme.Error).
			Render(fmt.Sprintf("Error: %s", toolResult))
	} else {
		// Format result based on tool type
		fullContent = r.formatToolResult(toolName, toolResult, r.width-8)
	}

	// Handle empty content
	if strings.TrimSpace(fullContent) == "" {
		fullContent = lipgloss.NewStyle().
			Italic(true).
			Foreground(theme.Muted).
			Render("(no output)")
	}

	// Use the new block renderer
	rendered := renderContentBlock(
		strings.TrimSuffix(fullContent, "\n"),
		r.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(theme.Muted),
		WithMarginBottom(1),
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

	// Truncate if too long, but skip truncation in debug mode
	if !r.debug {
		maxLen := 100
		if len(args) > maxLen {
			return args[:maxLen] + "..."
		}
	}

	return args
}

// formatToolResult formats tool results based on tool type
func (r *MessageRenderer) formatToolResult(toolName, result string, width int) string {
	baseStyle := lipgloss.NewStyle()

	// Truncate very long results only if not in debug mode
	if !r.debug {
		maxLines := 10
		lines := strings.Split(result, "\n")
		if len(lines) > maxLines {
			result = strings.Join(lines[:maxLines], "\n") + "\n... (truncated)"
		}
	}

	// Format bash/command output with better formatting
	if strings.Contains(toolName, "bash") || strings.Contains(toolName, "command") || strings.Contains(toolName, "shell") || toolName == "run_shell_cmd" {
		theme := getTheme()

		// Split result into sections if it contains both stdout and stderr
		if strings.Contains(result, "<stdout>") || strings.Contains(result, "<stderr>") {
			return r.formatBashOutput(result, width, theme)
		}

		// For simple output, just render as monospace text with proper line breaks
		return baseStyle.
			Width(width).
			Foreground(theme.Muted).
			Render(result)
	}

	// For other tools, render as muted text
	theme := getTheme()
	return baseStyle.
		Width(width).
		Foreground(theme.Muted).
		Render(result)
}

// formatBashOutput formats bash command output with proper section handling
func (r *MessageRenderer) formatBashOutput(result string, width int, theme Theme) string {
	baseStyle := lipgloss.NewStyle()

	// Replace tag pairs with styled content
	var formattedResult strings.Builder
	remaining := result

	for {
		// Find stderr tags
		stderrStart := strings.Index(remaining, "<stderr>")
		stderrEnd := strings.Index(remaining, "</stderr>")

		// Find stdout tags
		stdoutStart := strings.Index(remaining, "<stdout>")
		stdoutEnd := strings.Index(remaining, "</stdout>")

		// Process whichever comes first
		if stderrStart != -1 && stderrEnd != -1 && stderrEnd > stderrStart &&
			(stdoutStart == -1 || stderrStart < stdoutStart) {
			// Process stderr
			// Add content before the tag
			if stderrStart > 0 {
				formattedResult.WriteString(remaining[:stderrStart])
			}
			// Extract and style stderr content
			stderrContent := remaining[stderrStart+8 : stderrEnd]
			// Trim leading/trailing newlines but preserve internal ones
			stderrContent = strings.Trim(stderrContent, "\n")
			if len(stderrContent) > 0 {
				styledContent := baseStyle.Foreground(theme.Error).Render(stderrContent)
				formattedResult.WriteString(styledContent)
			}

			// Continue with remaining content
			remaining = remaining[stderrEnd+9:] // Skip past </stderr>

		} else if stdoutStart != -1 && stdoutEnd != -1 && stdoutEnd > stdoutStart {
			// Process stdout
			// Add content before the tag
			if stdoutStart > 0 {
				formattedResult.WriteString(remaining[:stdoutStart])
			}

			// Extract stdout content (no special styling needed)
			stdoutContent := remaining[stdoutStart+8 : stdoutEnd]
			// Trim leading/trailing newlines but preserve internal ones
			stdoutContent = strings.Trim(stdoutContent, "\n")
			if len(stdoutContent) > 0 {
				formattedResult.WriteString(stdoutContent)
			}

			// Continue with remaining content
			remaining = remaining[stdoutEnd+9:] // Skip past </stdout>

		} else {
			// No more tags, add remaining content
			formattedResult.WriteString(remaining)
			break
		}
	}

	// Trim any leading/trailing whitespace from the final result
	finalResult := strings.TrimSpace(formattedResult.String())

	return baseStyle.
		Width(width).
		Foreground(theme.Muted).
		Render(finalResult)
}

// truncateText truncates text to fit within the specified width
func (r *MessageRenderer) truncateText(text string, maxWidth int) string {
	// In debug mode, don't truncate - just replace newlines with spaces
	if r.debug {
		return strings.ReplaceAll(text, "\n", " ")
	}

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
	messages    []UIMessage
	width       int
	height      int
	compactMode bool   // Add compact mode flag
	modelName   string // Store current model name
}

// NewMessageContainer creates a new message container
func NewMessageContainer(width, height int, compact bool) *MessageContainer {
	return &MessageContainer{
		messages:    make([]UIMessage, 0),
		width:       width,
		height:      height,
		compactMode: compact,
	}
}

// AddMessage adds a message to the container
func (c *MessageContainer) AddMessage(msg UIMessage) {
	c.messages = append(c.messages, msg)
}

// SetModelName sets the current model name for the container
func (c *MessageContainer) SetModelName(modelName string) {
	c.modelName = modelName
}

// UpdateLastMessage updates the content of the last message efficiently
func (c *MessageContainer) UpdateLastMessage(content string) {
	if len(c.messages) == 0 {
		return
	}

	lastIdx := len(c.messages) - 1
	lastMsg := &c.messages[lastIdx]

	// Only re-render if content actually changed and it's an assistant message
	if lastMsg.Type == AssistantMessage {
		// Create appropriate renderer based on compact mode
		var newMsg UIMessage
		if c.compactMode {
			compactRenderer := NewCompactRenderer(c.width, false)
			newMsg = compactRenderer.RenderAssistantMessage(content, lastMsg.Timestamp, c.modelName)
		} else {
			renderer := NewMessageRenderer(c.width, false)
			newMsg = renderer.RenderAssistantMessage(content, lastMsg.Timestamp, c.modelName)
		}
		newMsg.Streaming = lastMsg.Streaming // Preserve streaming state
		c.messages[lastIdx] = newMsg
	}
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
		if c.compactMode {
			return c.renderCompactEmptyState()
		}
		return c.renderEmptyState()
	}

	if c.compactMode {
		return c.renderCompactMessages()
	}

	var parts []string

	for i, msg := range c.messages {
		// Center each message horizontally
		centeredMsg := lipgloss.PlaceHorizontal(
			c.width,
			lipgloss.Center,
			msg.Content,
		)
		parts = append(parts, centeredMsg)

		// Add spacing between messages (except after the last one)
		if i < len(c.messages)-1 {
			parts = append(parts, "")
		}
	}

	style := lipgloss.NewStyle().
		Width(c.width)

	// No padding needed between messages

	return style.Render(
		lipgloss.JoinVertical(lipgloss.Top, parts...),
	)
}

// renderEmptyState renders an enhanced initial empty state
func (c *MessageContainer) renderEmptyState() string {
	baseStyle := lipgloss.NewStyle()

	// Create a welcome box with border
	theme := getTheme()
	welcomeBox := baseStyle.
		Width(c.width-4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.System).
		Padding(2, 4).
		Align(lipgloss.Center)

	// Main title
	title := baseStyle.
		Foreground(theme.System).
		Bold(true).
		Render("MCPHost")

	// Subtitle with better typography
	subtitle := baseStyle.
		Foreground(theme.Primary).
		Bold(true).
		MarginTop(1).
		Render("AI Assistant with MCP Tools")

	// Feature highlights
	features := []string{
		"Natural language conversations",
		"Powerful tool integrations",
		"Multi-provider LLM support",
		"Usage tracking & analytics",
	}

	var featureList []string
	for _, feature := range features {
		featureList = append(featureList, baseStyle.
			Foreground(theme.Muted).
			MarginLeft(2).
			Render("• "+feature))
	}

	// Getting started prompt
	prompt := baseStyle.
		Foreground(theme.Accent).
		Italic(true).
		MarginTop(2).
		Render("Start by typing your message below or use /help for commands")

	// Combine all elements
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		subtitle,
		"",
		lipgloss.JoinVertical(lipgloss.Left, featureList...),
		"",
		prompt,
	)

	welcomeContent := welcomeBox.Render(content)

	// Center the welcome box vertically
	return baseStyle.
		Width(c.width).
		Height(c.height).
		Align(lipgloss.Center).
		AlignVertical(lipgloss.Center).
		Render(welcomeContent)
}

// renderCompactMessages renders messages in compact format
func (c *MessageContainer) renderCompactMessages() string {
	var lines []string

	for _, msg := range c.messages {
		lines = append(lines, msg.Content)
	}

	return strings.Join(lines, "\n")
}

// renderCompactEmptyState renders a simple empty state for compact mode
func (c *MessageContainer) renderCompactEmptyState() string {
	theme := getTheme()

	// Simple compact welcome
	welcome := lipgloss.NewStyle().
		Foreground(theme.System).
		Bold(true).
		Render("MCPHost - AI Assistant with MCP Tools")

	help := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Render("Type your message or /help for commands")

	return fmt.Sprintf("%s\n%s\n\n", welcome, help)
}
