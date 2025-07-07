package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cloudwego/eino/schema"
)

var (
	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)

// CLI handles the command line interface with improved message rendering
type CLI struct {
	messageRenderer    *MessageRenderer
	compactRenderer    *CompactRenderer
	messageContainer   *MessageContainer
	usageTracker       *UsageTracker
	terminalRenderer   *TerminalRenderer // NEW: Add terminal renderer
	width              int
	height             int
	compactMode        bool
	modelName          string
	lastMessageRow     int                // NEW: Track last message position
	streamingActive    bool               // NEW: Track streaming state
	streamingStartRow  int                // NEW: Track where streaming message starts
	streamingLineCount int                // NEW: Track how many lines streaming message occupies
	currentPrompt      *InteractivePrompt // NEW: Current interactive prompt component
}

// NewCLI creates a new CLI instance with message container
func NewCLI(debug bool, compact bool) (*CLI, error) {
	cli := &CLI{
		compactMode: compact,
	}
	cli.updateSize()

	// Initialize renderers
	cli.messageRenderer = NewMessageRenderer(cli.width, debug)
	cli.compactRenderer = NewCompactRenderer(cli.width, debug)
	cli.messageContainer = NewMessageContainer(cli.width, cli.height-4, compact)

	// NEW: Initialize terminal renderer
	cli.terminalRenderer = NewTerminalRenderer(os.Stdout)

	return cli, nil
}

// SetUsageTracker sets the usage tracker for the CLI
func (c *CLI) SetUsageTracker(tracker *UsageTracker) {
	c.usageTracker = tracker
	if c.usageTracker != nil {
		c.usageTracker.SetWidth(c.width)
	}
}

// SetModelName sets the current model name for the CLI
func (c *CLI) SetModelName(modelName string) {
	c.modelName = modelName
	if c.messageContainer != nil {
		c.messageContainer.SetModelName(modelName)
	}
}

// GetPrompt gets user input using the comprehensive interactive component
func (c *CLI) GetPrompt() (string, error) {
	// Create the interactive prompt component with usage display
	c.currentPrompt = NewInteractivePrompt(c.usageTracker, c.width, c.compactMode)

	// Run the Bubble Tea program
	program := tea.NewProgram(c.currentPrompt)
	model, err := program.Run()

	// Clear the current prompt reference
	defer func() {
		c.currentPrompt = nil
	}()

	if err != nil {
		return "", err
	}

	// Extract the result
	if p, ok := model.(*InteractivePrompt); ok {
		if !p.WasSubmitted() {
			return "", io.EOF // User cancelled
		}
		return p.GetPrompt(), nil
	}

	return "", errors.New("unexpected model type")
}

// ShowSpinner displays a spinner with the given message and executes the action
func (c *CLI) ShowSpinner(message string, action func() error) error {
	// If we have an active interactive prompt, use its spinner
	if c.currentPrompt != nil {
		// Show spinner in the interactive prompt
		c.currentPrompt.ShowSpinner(message)

		// Execute the action
		err := action()

		// Hide spinner when done
		c.currentPrompt.HideSpinner()

		return err
	}

	// Fallback to standalone spinner for non-interactive contexts
	spinner := NewSpinner(message)
	spinner.Start()

	err := action()

	spinner.Stop()

	return err
}

// DisplayUserMessage displays the user's message using the appropriate renderer
func (c *CLI) DisplayUserMessage(message string) {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderUserMessage(message, time.Now())
	} else {
		msg = c.messageRenderer.RenderUserMessage(message, time.Now())
	}

	// Use unified display logic that works in both streaming and non-streaming modes
	c.DisplayMessageWithStreaming(msg)

	// Always scroll up by 100% AFTER displaying the new user message for clean slate
	c.terminalRenderer.ScrollUpPercent(100.0)
	// Reset our position tracking since everything shifted up
	c.lastMessageRow = 0
}

// DisplayAssistantMessage displays the assistant's message using the new renderer
func (c *CLI) DisplayAssistantMessage(message string) error {
	return c.DisplayAssistantMessageWithModel(message, "")
}

// DisplayAssistantMessageWithModel displays the assistant's message with model info
func (c *CLI) DisplayAssistantMessageWithModel(message, modelName string) error {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderAssistantMessage(message, time.Now(), modelName)
	} else {
		msg = c.messageRenderer.RenderAssistantMessage(message, time.Now(), modelName)
	}

	// Use unified display logic
	c.DisplayMessageWithStreaming(msg)
	return nil
}

// DisplayToolCallMessage displays a tool call in progress
func (c *CLI) DisplayToolCallMessage(toolName, toolArgs string) {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderToolCallMessage(toolName, toolArgs, time.Now())
	} else {
		msg = c.messageRenderer.RenderToolCallMessage(toolName, toolArgs, time.Now())
	}

	// Use unified display logic
	c.DisplayMessageWithStreaming(msg)
}

// DisplayToolMessage displays a tool call message
func (c *CLI) DisplayToolMessage(toolName, toolArgs, toolResult string, isError bool) {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderToolMessage(toolName, toolArgs, toolResult, isError)
	} else {
		msg = c.messageRenderer.RenderToolMessage(toolName, toolArgs, toolResult, isError)
	}

	// Use unified display logic
	c.DisplayMessageWithStreaming(msg)
}

// StartStreamingMessage starts a streaming assistant message
func (c *CLI) StartStreamingMessage(modelName string) {
	// Mark streaming as active
	c.streamingActive = true

	// Add an empty assistant message that we'll update during streaming
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderAssistantMessage("", time.Now(), modelName)
	} else {
		msg = c.messageRenderer.RenderAssistantMessage("", time.Now(), modelName)
	}
	c.messageContainer.AddMessage(msg)

	// Calculate where this streaming message starts
	// This is where we'll clear and re-render during updates
	content := c.messageContainer.Render()
	lines := strings.Split(content, "\n")

	// Find the start of the last message (the streaming message)
	// We'll work backwards to find where this message begins
	c.streamingStartRow = c.calculateStreamingMessageStart(lines)
	c.streamingLineCount = 0 // Will be set during first update

	// Display initial state using terminal renderer
	c.displayContainer()
}

// calculateStreamingMessageStart finds where the streaming message starts in the rendered content
func (c *CLI) calculateStreamingMessageStart(_ []string) int {
	// For now, simple approach: streaming message starts after the last non-empty line
	// before the current message. This can be refined based on message container logic.
	return c.lastMessageRow + 1
}

// UpdateStreamingMessage updates the streaming message with new content using clear-and-rerender
func (c *CLI) UpdateStreamingMessage(content string) {
	if !c.streamingActive {
		return
	}

	// Hide cursor during update
	c.terminalRenderer.HideCursor()
	defer c.terminalRenderer.ShowCursor()

	// Update the message container with new content
	c.messageContainer.UpdateLastMessage(content)

	// Clear the existing streaming message area and re-render
	c.updateStreamingMessageOnly("")
}

// EndStreamingMessage marks the end of streaming and performs final cleanup
func (c *CLI) EndStreamingMessage() {
	c.streamingActive = false

	// Reset streaming position tracking
	c.streamingStartRow = 0
	c.streamingLineCount = 0

	// Show cursor
	c.terminalRenderer.ShowCursor()
}

// DisplayMessageWithStreaming displays any message type using terminal renderer during streaming
func (c *CLI) DisplayMessageWithStreaming(msg UIMessage) {
	if !c.streamingActive {
		// Not in streaming mode, use normal display
		c.messageContainer.AddMessage(msg)
		c.displayContainer()
		return
	}

	// In streaming mode, add message and use terminal renderer
	c.messageContainer.AddMessage(msg)

	// Hide cursor during update
	c.terminalRenderer.HideCursor()
	defer c.terminalRenderer.ShowCursor()

	// Get the rendered content for this message
	content := c.messageContainer.Render()

	// Calculate the position for this new message
	lines := strings.Split(content, "\n")
	messageStartRow := c.lastMessageRow + 1

	// Render the new message lines
	for i, line := range lines[messageStartRow:] {
		if line != "" {
			if c.compactMode {
				// Compact mode: no padding, direct output
				// Format: "symbol  Label content"
				c.terminalRenderer.WriteAt(messageStartRow+i, 0, line)
			} else {
				// Normal mode: 2-space left padding + message container formatting
				// The message container already handles the box formatting
				paddedLine := strings.Repeat(" ", 2) + line
				c.terminalRenderer.WriteAt(messageStartRow+i, 0, paddedLine)
			}
		}
	}

	// Update last message row and terminal cursor position
	c.lastMessageRow = len(lines) - 1
	c.terminalRenderer.MoveTo(c.lastMessageRow, 0)
}

// DisplayError displays an error message using the appropriate renderer
func (c *CLI) DisplayError(err error) {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderErrorMessage(err.Error(), time.Now())
	} else {
		msg = c.messageRenderer.RenderErrorMessage(err.Error(), time.Now())
	}

	// Use unified display logic
	c.DisplayMessageWithStreaming(msg)
}

// DisplayInfo displays an informational message using the appropriate renderer
func (c *CLI) DisplayInfo(message string) {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderSystemMessage(message, time.Now())
	} else {
		msg = c.messageRenderer.RenderSystemMessage(message, time.Now())
	}

	// Use unified display logic
	c.DisplayMessageWithStreaming(msg)
}

// DisplayCancellation displays a cancellation message
func (c *CLI) DisplayCancellation() {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderSystemMessage("Generation cancelled by user (ESC pressed)", time.Now())
	} else {
		msg = c.messageRenderer.RenderSystemMessage("Generation cancelled by user (ESC pressed)", time.Now())
	}

	// Use unified display logic
	c.DisplayMessageWithStreaming(msg)
}

// DisplayDebugConfig displays configuration settings using the appropriate renderer
func (c *CLI) DisplayDebugConfig(config map[string]any) {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderDebugConfigMessage(config, time.Now())
	} else {
		msg = c.messageRenderer.RenderDebugConfigMessage(config, time.Now())
	}

	// Use unified display logic
	c.DisplayMessageWithStreaming(msg)
}

// DisplayHelp displays help information in a message block
func (c *CLI) DisplayHelp() {
	help := `## Available Commands

- ` + "`/help`" + `: Show this help message
- ` + "`/tools`" + `: List all available tools
- ` + "`/servers`" + `: List configured MCP servers
- ` + "`/history`" + `: Display conversation history
- ` + "`/usage`" + `: Show token usage and cost statistics
- ` + "`/reset-usage`" + `: Reset usage statistics
- ` + "`/clear`" + `: Clear message history
- ` + "`/quit`" + `: Exit the application
- ` + "`Ctrl+C`" + `: Exit at any time
- ` + "`ESC`" + `: Cancel ongoing LLM generation

You can also just type your message to chat with the AI assistant.`

	// Display as a system message
	msg := c.messageRenderer.RenderSystemMessage(help, time.Now())
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayTools displays available tools in a message block
func (c *CLI) DisplayTools(tools []string) {
	var content strings.Builder
	content.WriteString("## Available Tools\n\n")

	if len(tools) == 0 {
		content.WriteString("No tools are currently available.")
	} else {
		for i, tool := range tools {
			content.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, tool))
		}
	}

	// Display as a system message
	msg := c.messageRenderer.RenderSystemMessage(content.String(), time.Now())
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayServers displays configured MCP servers in a message block
func (c *CLI) DisplayServers(servers []string) {
	var content strings.Builder
	content.WriteString("## Configured MCP Servers\n\n")

	if len(servers) == 0 {
		content.WriteString("No MCP servers are currently configured.")
	} else {
		for i, server := range servers {
			content.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, server))
		}
	}

	// Display as a system message
	msg := c.messageRenderer.RenderSystemMessage(content.String(), time.Now())
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayHistory displays conversation history using the message container
func (c *CLI) DisplayHistory(messages []*schema.Message) {
	// Create a temporary container for history
	historyContainer := NewMessageContainer(c.width, c.height-4, c.compactMode)

	for _, msg := range messages {
		switch msg.Role {
		case schema.User:
			var uiMsg UIMessage
			if c.compactMode {
				uiMsg = c.compactRenderer.RenderUserMessage(msg.Content, time.Now())
			} else {
				uiMsg = c.messageRenderer.RenderUserMessage(msg.Content, time.Now())
			}
			historyContainer.AddMessage(uiMsg)
		case schema.Assistant:
			var uiMsg UIMessage
			if c.compactMode {
				uiMsg = c.compactRenderer.RenderAssistantMessage(msg.Content, time.Now(), c.modelName)
			} else {
				uiMsg = c.messageRenderer.RenderAssistantMessage(msg.Content, time.Now(), c.modelName)
			}
			historyContainer.AddMessage(uiMsg)
		}
	}

	// Use terminal renderer instead of fmt.Println
	headerText := "\nConversation History:\n"
	historyContent := historyContainer.Render()

	c.terminalRenderer.WriteAt(c.lastMessageRow+1, 0, headerText)
	c.terminalRenderer.WriteAt(c.lastMessageRow+3, 0, historyContent)
	c.lastMessageRow += strings.Count(headerText+historyContent, "\n") + 1
}

// IsSlashCommand checks if the input is a slash command
func (c *CLI) IsSlashCommand(input string) bool {
	return strings.HasPrefix(input, "/")
}

// SlashCommandResult represents the result of handling a slash command
type SlashCommandResult struct {
	Handled      bool
	ClearHistory bool
}

// HandleSlashCommand handles slash commands and returns the result
func (c *CLI) HandleSlashCommand(input string, servers []string, tools []string, history []*schema.Message) SlashCommandResult {
	switch input {
	case "/help":
		c.DisplayHelp()
		return SlashCommandResult{Handled: true}
	case "/tools":
		c.DisplayTools(tools)
		return SlashCommandResult{Handled: true}
	case "/servers":
		c.DisplayServers(servers)
		return SlashCommandResult{Handled: true}
	case "/history":
		c.DisplayHistory(history)
		return SlashCommandResult{Handled: true}
	case "/clear":
		c.ClearMessages()
		c.DisplayInfo("Conversation cleared. Starting fresh.")
		return SlashCommandResult{Handled: true, ClearHistory: true}
	case "/usage":
		c.DisplayUsageStats()
		return SlashCommandResult{Handled: true}
	case "/reset-usage":
		c.ResetUsageStats()
		return SlashCommandResult{Handled: true}
	case "/quit":
		goodbyeText := "\nGoodbye!\n"
		c.terminalRenderer.WriteAt(c.lastMessageRow+1, 0, goodbyeText)
		os.Exit(0)
		return SlashCommandResult{Handled: true}
	default:
		return SlashCommandResult{Handled: false}
	}
}

// ClearMessages clears all messages from the container
func (c *CLI) ClearMessages() {
	c.messageContainer.Clear()
	c.displayContainer()
}

// displayContainer renders and displays the message container using termenv
func (c *CLI) displayContainer() {
	if c.streamingActive {
		// During streaming, use terminal renderer for all updates
		c.displayContainerStreaming()
	} else {
		// Non-streaming mode - use terminal renderer for consistency
		c.displayContainerNormal()
	}
}

// displayContainerNormal handles non-streaming display using terminal renderer
func (c *CLI) displayContainerNormal() {
	// Hide cursor during updates
	c.terminalRenderer.HideCursor()
	defer c.terminalRenderer.ShowCursor()

	// Get the last message that was just added
	lastMessage := c.messageContainer.GetLastMessage()
	if lastMessage == nil {
		return
	}

	// Display the new message incrementally at current cursor position
	c.displayMessageIncremental(lastMessage)
}

// displayMessageIncremental displays a single message at the current cursor position
func (c *CLI) displayMessageIncremental(msg *UIMessage) {
	// Get current cursor position
	currentRow, _ := c.terminalRenderer.GetCursorPosition()
	messageStartRow := currentRow + 1

	// Split message content into lines
	lines := strings.Split(msg.Content, "\n")

	// Render each line
	for i, line := range lines {
		if line != "" {
			if c.compactMode {
				// Compact mode: no padding
				c.terminalRenderer.WriteAt(messageStartRow+i, 0, line)
			} else {
				// Normal mode: 2-space left padding
				paddedLine := strings.Repeat(" ", 2) + line
				c.terminalRenderer.WriteAt(messageStartRow+i, 0, paddedLine)
			}
		}
	}

	// Update last message row and move cursor to end
	c.lastMessageRow = messageStartRow + len(lines) - 1
	c.terminalRenderer.MoveTo(c.lastMessageRow, 0)
}

// displayContainerStreaming handles streaming display with simplified clear-and-rerender
func (c *CLI) displayContainerStreaming() {
	// Hide cursor during updates
	c.terminalRenderer.HideCursor()
	defer c.terminalRenderer.ShowCursor()

	// Get container content
	content := c.messageContainer.Render()

	// For streaming, we only need to update the streaming message area
	// All other messages remain static during streaming
	c.updateStreamingMessageOnly(content)
}

// updateStreamingMessageOnly updates only the streaming message using clear-and-rerender
func (c *CLI) updateStreamingMessageOnly(_ string) {
	if !c.streamingActive {
		return
	}

	// Clear the existing streaming message lines
	if c.streamingLineCount > 0 {
		c.terminalRenderer.MoveTo(c.streamingStartRow, 0)
		c.terminalRenderer.ClearLines(c.streamingLineCount)
	}

	// Get the last message (which should be the streaming message)
	lastMessage := c.messageContainer.GetLastMessage()
	if lastMessage == nil {
		return
	}

	// Split the message content into lines
	lines := strings.Split(lastMessage.Content, "\n")

	// Render each line
	for i, line := range lines {
		if line != "" {
			if c.compactMode {
				// Compact mode: no padding
				c.terminalRenderer.WriteAt(c.streamingStartRow+i, 0, line)
			} else {
				// Normal mode: 2-space left padding
				paddedLine := strings.Repeat(" ", 2) + line
				c.terminalRenderer.WriteAt(c.streamingStartRow+i, 0, paddedLine)
			}
		}
	}

	// Update line count for next clear operation
	c.streamingLineCount = len(lines)
}

// UpdateUsage updates the usage tracker with token counts and costs
func (c *CLI) UpdateUsage(inputText, outputText string) {
	if c.usageTracker != nil {
		c.usageTracker.EstimateAndUpdateUsage(inputText, outputText)
	}
}

// UpdateUsageFromResponse updates the usage tracker using token usage from response metadata
func (c *CLI) UpdateUsageFromResponse(response *schema.Message, inputText string) {
	if c.usageTracker == nil {
		return
	}

	// Try to extract token usage from response metadata
	if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
		usage := response.ResponseMeta.Usage

		// Use actual token counts from the response
		inputTokens := int(usage.PromptTokens)
		outputTokens := int(usage.CompletionTokens)

		// Validate that the metadata seems reasonable
		// If token counts are 0 or seem unrealistic, fall back to estimation
		if inputTokens > 0 && outputTokens > 0 {
			// Handle cache tokens if available (some providers support this)
			cacheReadTokens := 0
			cacheWriteTokens := 0

			c.usageTracker.UpdateUsage(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
		} else {
			// Metadata exists but seems incomplete/unreliable, use estimation
			c.usageTracker.EstimateAndUpdateUsage(inputText, response.Content)
		}
	} else {
		// Fallback to estimation if no metadata is available
		c.usageTracker.EstimateAndUpdateUsage(inputText, response.Content)
	}
}

// DisplayUsageStats displays current usage statistics
func (c *CLI) DisplayUsageStats() {
	if c.usageTracker == nil {
		c.DisplayInfo("Usage tracking is not available for this model.")
		return
	}

	sessionStats := c.usageTracker.GetSessionStats()
	lastStats := c.usageTracker.GetLastRequestStats()

	var content strings.Builder
	content.WriteString("## Usage Statistics\n\n")

	if lastStats != nil {
		content.WriteString(fmt.Sprintf("**Last Request:** %d input + %d output tokens = $%.6f\n",
			lastStats.InputTokens, lastStats.OutputTokens, lastStats.TotalCost))
	}

	content.WriteString(fmt.Sprintf("**Session Total:** %d input + %d output tokens = $%.6f (%d requests)\n",
		sessionStats.TotalInputTokens, sessionStats.TotalOutputTokens, sessionStats.TotalCost, sessionStats.RequestCount))

	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderSystemMessage(content.String(), time.Now())
	} else {
		msg = c.messageRenderer.RenderSystemMessage(content.String(), time.Now())
	}
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// ResetUsageStats resets the usage tracking statistics
func (c *CLI) ResetUsageStats() {
	if c.usageTracker == nil {
		c.DisplayInfo("Usage tracking is not available for this model.")
		return
	}

	c.usageTracker.Reset()
	c.DisplayInfo("Usage statistics have been reset.")
}

// DisplayUsageAfterResponse is now a no-op since usage is displayed in the prompt component
func (c *CLI) DisplayUsageAfterResponse() {
	// Usage is now displayed in the GetPrompt() Bubble Tea component
}

// updateSize updates the CLI size based on terminal dimensions
func (c *CLI) updateSize() {
	// Update terminal renderer size first
	if c.terminalRenderer != nil {
		c.terminalRenderer.UpdateSize()
		c.width, c.height = c.terminalRenderer.GetSize()
	} else {
		// Fallback for initialization
		width, height := getTerminalSize()
		c.width = width - 4 // Account for padding
		c.height = height
	}

	// Update renderers if they exist
	if c.messageRenderer != nil {
		c.messageRenderer.SetWidth(c.width)
	}
	if c.compactRenderer != nil {
		c.compactRenderer.SetWidth(c.width)
	}
	if c.messageContainer != nil {
		c.messageContainer.SetSize(c.width, c.height-4)
	}
	if c.usageTracker != nil {
		c.usageTracker.SetWidth(c.width)
	}
}
