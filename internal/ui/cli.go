package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cloudwego/eino/schema"
	"golang.org/x/term"
)

var promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

// CLI manages the command-line interface for MCPHost, providing message rendering,
// user input handling, and display management. It supports both standard and compact
// display modes, handles streaming responses, tracks token usage, and manages the
// overall conversation flow between the user and AI assistants.
type CLI struct {
	messageRenderer  *MessageRenderer
	compactRenderer  *CompactRenderer // Add compact renderer
	messageContainer *MessageContainer
	usageTracker     *UsageTracker
	width            int
	height           int
	compactMode      bool   // Add compact mode flag
	debug            bool   // Add debug mode flag
	modelName        string // Store current model name
	lastStreamHeight int    // track how far back we need to move the cursor to overwrite streaming messages
	usageDisplayed   bool   // track if usage info was displayed after last assistant message
}

// NewCLI creates and initializes a new CLI instance with the specified display modes.
// The debug parameter enables debug message rendering, while compact enables a more
// condensed display format. Returns an initialized CLI ready for interaction or an
// error if initialization fails.
func NewCLI(debug bool, compact bool) (*CLI, error) {
	cli := &CLI{
		compactMode: compact,
		debug:       debug,
	}
	cli.updateSize()
	cli.messageRenderer = NewMessageRenderer(cli.width, debug)
	cli.compactRenderer = NewCompactRenderer(cli.width, debug)
	cli.messageContainer = NewMessageContainer(cli.width, cli.height-4, compact) // Pass compact mode

	return cli, nil
}

// SetUsageTracker attaches a usage tracker to the CLI for monitoring token
// consumption and costs. The tracker will be automatically updated with the
// current display width for proper rendering.
func (c *CLI) SetUsageTracker(tracker *UsageTracker) {
	c.usageTracker = tracker
	if c.usageTracker != nil {
		c.usageTracker.SetWidth(c.width)
	}
}

// GetDebugLogger returns a CLIDebugLogger instance that routes debug output
// through the CLI's rendering system for consistent message formatting and display.
func (c *CLI) GetDebugLogger() *CLIDebugLogger {
	return NewCLIDebugLogger(c)
}

// SetModelName updates the current AI model name being used in the conversation.
// This name is displayed in message headers to indicate which model is responding.
func (c *CLI) SetModelName(modelName string) {
	c.modelName = modelName
	if c.messageContainer != nil {
		c.messageContainer.SetModelName(modelName)
	}
}

// GetPrompt displays an interactive prompt and waits for user input. It provides
// slash command support, multi-line editing, and cancellation handling. Returns
// the user's input as a string, or an error if the operation was cancelled or
// failed. Returns io.EOF for clean exit signals.
func (c *CLI) GetPrompt() (string, error) {
	// Usage info is now displayed immediately after responses via DisplayUsageAfterResponse()
	// No need to display it here to avoid duplication

	c.messageContainer.messages = nil // clear previous messages (they should have been printed already)
	c.lastStreamHeight = 0            // Reset last stream height for new prompt

	// No divider needed - removed for cleaner appearance

	// Create our custom slash command input
	input := NewSlashCommandInput(c.width, "Enter your prompt (Type /help for commands, Ctrl+C to quit, ESC to cancel generation)")

	// Run as a tea program
	p := tea.NewProgram(input)
	finalModel, err := p.Run()

	if err != nil {
		return "", err
	}

	// Get the value from the final model
	if finalInput, ok := finalModel.(*SlashCommandInput); ok {
		// Clear the input field from the display
		linesToClear := finalInput.RenderedLines()
		// We need to clear linesToClear - 1 lines because we're already on the line after the last rendered line
		for i := 0; i < linesToClear-1; i++ {
			fmt.Print("\033[1A\033[2K") // Move up one line and clear it
		}

		if finalInput.Cancelled() {
			return "", io.EOF // Signal clean exit
		}
		value := strings.TrimSpace(finalInput.Value())
		return value, nil
	}

	return "", fmt.Errorf("unexpected model type")
}

// ShowSpinner displays an animated spinner with the specified message while
// executing the provided action function. The spinner automatically stops when
// the action completes. Returns any error returned by the action function.
func (c *CLI) ShowSpinner(message string, action func() error) error {
	spinner := NewSpinner(message)
	spinner.Start()

	err := action()

	spinner.Stop()

	return err
}

// DisplayUserMessage renders and displays a user's message with appropriate
// formatting based on the current display mode (standard or compact). The message
// is timestamped and styled according to the active theme.
func (c *CLI) DisplayUserMessage(message string) {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderUserMessage(message, time.Now())
	} else {
		msg = c.messageRenderer.RenderUserMessage(message, time.Now())
	}
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayAssistantMessage renders and displays an AI assistant's response message
// with appropriate formatting. This method delegates to DisplayAssistantMessageWithModel
// with an empty model name for backward compatibility.
func (c *CLI) DisplayAssistantMessage(message string) error {
	return c.DisplayAssistantMessageWithModel(message, "")
}

// DisplayAssistantMessageWithModel renders and displays an AI assistant's response
// with the specified model name shown in the message header. The message is
// formatted according to the current display mode and includes timestamp information.
func (c *CLI) DisplayAssistantMessageWithModel(message, modelName string) error {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderAssistantMessage(message, time.Now(), modelName)
	} else {
		msg = c.messageRenderer.RenderAssistantMessage(message, time.Now(), modelName)
	}
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
	return nil
}

// DisplayToolCallMessage renders and displays a message indicating that a tool
// is being executed. Shows the tool name and its arguments formatted appropriately
// for the current display mode. This is typically shown while a tool is running.
func (c *CLI) DisplayToolCallMessage(toolName, toolArgs string) {

	c.messageContainer.messages = nil // clear previous messages (they should have been printed already)
	c.lastStreamHeight = 0            // Reset last stream height for new prompt

	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderToolCallMessage(toolName, toolArgs, time.Now())
	} else {
		msg = c.messageRenderer.RenderToolCallMessage(toolName, toolArgs, time.Now())
	}

	// Always display immediately - spinner management is handled externally
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayToolMessage renders and displays the complete result of a tool execution,
// including the tool name, arguments, and result. The isError parameter determines
// whether the result should be displayed as an error or success message.
func (c *CLI) DisplayToolMessage(toolName, toolArgs, toolResult string, isError bool) {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderToolMessage(toolName, toolArgs, toolResult, isError)
	} else {
		msg = c.messageRenderer.RenderToolMessage(toolName, toolArgs, toolResult, isError)
	}

	// Always display immediately - spinner management is handled externally
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// StartStreamingMessage initializes a new streaming message display for real-time
// AI responses. The message will be progressively updated as content arrives.
// The modelName parameter indicates which AI model is generating the response.
func (c *CLI) StartStreamingMessage(modelName string) {
	// Add an empty assistant message that we'll update during streaming
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderAssistantMessage("", time.Now(), modelName)
	} else {
		msg = c.messageRenderer.RenderAssistantMessage("", time.Now(), modelName)
	}
	msg.Streaming = true
	c.lastStreamHeight = 0 // Reset last stream height for new message
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// UpdateStreamingMessage updates the currently streaming message with new content.
// This method should be called after StartStreamingMessage to progressively display
// AI responses as they are generated in real-time.
func (c *CLI) UpdateStreamingMessage(content string) {
	// Update the last message (which should be the streaming assistant message)
	c.messageContainer.UpdateLastMessage(content)
	c.displayContainer()
}

// DisplayError renders and displays an error message with distinctive formatting
// to ensure visibility. The error is timestamped and styled according to the
// current display mode's error theme.
func (c *CLI) DisplayError(err error) {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderErrorMessage(err.Error(), time.Now())
	} else {
		msg = c.messageRenderer.RenderErrorMessage(err.Error(), time.Now())
	}
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayInfo renders and displays an informational system message. These messages
// are typically used for status updates, notifications, or other non-error system
// communications to the user.
func (c *CLI) DisplayInfo(message string) {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderSystemMessage(message, time.Now())
	} else {
		msg = c.messageRenderer.RenderSystemMessage(message, time.Now())
	}
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayCancellation displays a system message indicating that the current
// AI generation has been cancelled by the user (typically via ESC key).
func (c *CLI) DisplayCancellation() {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderSystemMessage("Generation cancelled by user (ESC pressed)", time.Now())
	} else {
		msg = c.messageRenderer.RenderSystemMessage("Generation cancelled by user (ESC pressed)", time.Now())
	}
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayDebugMessage renders and displays a debug message if debug mode is enabled.
// Debug messages are formatted distinctively and only shown when the CLI is
// initialized with debug=true.
func (c *CLI) DisplayDebugMessage(message string) {
	if !c.debug {
		return
	}
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderDebugMessage(message, time.Now())
	} else {
		msg = c.messageRenderer.RenderDebugMessage(message, time.Now())
	}
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayDebugConfig renders and displays configuration settings in a formatted
// debug message. The config parameter should contain key-value pairs representing
// configuration options that will be displayed for debugging purposes.
func (c *CLI) DisplayDebugConfig(config map[string]any) {
	var msg UIMessage
	if c.compactMode {
		msg = c.compactRenderer.RenderDebugConfigMessage(config, time.Now())
	} else {
		msg = c.messageRenderer.RenderDebugConfigMessage(config, time.Now())
	}
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayHelp renders and displays comprehensive help information showing all
// available slash commands, keyboard shortcuts, and usage instructions in a
// formatted system message block.
func (c *CLI) DisplayHelp() {
	help := `## Available Commands

- ` + "`/help`" + `: Show this help message
- ` + "`/tools`" + `: List all available tools
- ` + "`/servers`" + `: List configured MCP servers
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

// DisplayTools renders and displays a formatted list of all available tools
// that can be used by the AI assistant. Each tool is numbered and shown in
// a system message block for easy reference.
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

// DisplayServers renders and displays a formatted list of all configured MCP
// (Model Context Protocol) servers. Each server is numbered and shown in a
// system message block for easy reference.
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

// IsSlashCommand determines whether the provided input string is a slash command
// by checking if it starts with a forward slash (/). Returns true for commands
// like "/help", "/tools", etc.
func (c *CLI) IsSlashCommand(input string) bool {
	return strings.HasPrefix(input, "/")
}

// GetToolApproval asks the user for permission to execute the tool with the given
// arguments. Returns true if the user approves.
func (c *CLI) GetToolApproval(toolName, toolArgs string) (bool, error) {
	input := NewToolApprovalInput(toolName, toolArgs, c.width)
	p := tea.NewProgram(input)
	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}

	if finalInput, ok := finalModel.(*ToolApprovalInput); ok {
		return finalInput.approved, nil
	}
	return false, fmt.Errorf("GetToolApproval: unexpected error type")
}

// SlashCommandResult encapsulates the outcome of processing a slash command,
// indicating whether the command was recognized and handled, and whether the
// conversation history should be cleared as a result of the command.
type SlashCommandResult struct {
	Handled      bool
	ClearHistory bool
}

// HandleSlashCommand processes and executes slash commands, returning a result
// that indicates whether the command was handled and any side effects. The servers
// and tools parameters provide context for commands that display available resources.
// Supported commands include /help, /tools, /servers, /clear, /usage, /reset-usage, and /quit.
func (c *CLI) HandleSlashCommand(input string, servers []string, tools []string) SlashCommandResult {
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
		fmt.Println("\n  Goodbye!")
		os.Exit(0)
		return SlashCommandResult{Handled: true}
	default:
		return SlashCommandResult{Handled: false}
	}
}

// ClearMessages removes all messages from the display container and refreshes
// the screen. This is typically used when starting a new conversation or
// clearing the chat history.
func (c *CLI) ClearMessages() {
	c.messageContainer.Clear()
	c.displayContainer()
}

// displayContainer renders and displays the message container
func (c *CLI) displayContainer() {

	// Add left padding to the entire container
	content := c.messageContainer.Render()

	// Check if we're displaying a user message
	// User messages should not have additional left padding since they're right-aligned
	// This only applies in non-compact mode
	paddingLeft := 2
	if !c.compactMode && len(c.messageContainer.messages) > 0 {
		lastMessage := c.messageContainer.messages[len(c.messageContainer.messages)-1]
		if lastMessage.Type == UserMessage {
			paddingLeft = 0
		}
	}

	paddedContent := lipgloss.NewStyle().
		PaddingLeft(paddingLeft).
		Width(c.width). // overwrite (no content) while agent is streaming
		Render(content)

	if c.lastStreamHeight > 0 {
		// Move cursor up by the height of the last streamed message
		fmt.Printf("\033[%dF", c.lastStreamHeight)
	} else if c.usageDisplayed {
		// If we're not overwriting a streaming message but usage was displayed,
		// move up to account for the usage info (2 lines: content + padding)
		fmt.Printf("\033[2F")
		c.usageDisplayed = false
	}

	fmt.Println(paddedContent)

	// clear message history except the "in-progress" message
	if len(c.messageContainer.messages) > 0 {
		// keep the last message, clear the rest (in case of streaming)
		last := c.messageContainer.messages[len(c.messageContainer.messages)-1]
		c.messageContainer.messages = []UIMessage{}
		if last.Streaming {
			// If the last message is still streaming, we keep it
			c.messageContainer.messages = append(c.messageContainer.messages, last)
			c.lastStreamHeight = lipgloss.Height(paddedContent)
		}
	}
}

// UpdateUsage estimates and records token usage based on input and output text.
// This method uses text-based estimation when actual token counts are not available
// from the AI provider's response metadata.
func (c *CLI) UpdateUsage(inputText, outputText string) {
	if c.usageTracker != nil {
		c.usageTracker.EstimateAndUpdateUsage(inputText, outputText)
	}
}

// UpdateUsageFromResponse records token usage using metadata from the AI provider's
// response when available. Falls back to text-based estimation if the metadata is
// missing or appears unreliable. This provides more accurate usage tracking when
// providers supply token count information.
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

// DisplayUsageStats renders and displays comprehensive token usage statistics
// including the last request's token counts and costs, as well as session totals.
// Shows a message if usage tracking is not available for the current model.
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

// ResetUsageStats clears all accumulated usage statistics, resetting token counts
// and costs to zero. Displays a confirmation message after resetting or an info
// message if usage tracking is not available.
func (c *CLI) ResetUsageStats() {
	if c.usageTracker == nil {
		c.DisplayInfo("Usage tracking is not available for this model.")
		return
	}

	c.usageTracker.Reset()
	c.DisplayInfo("Usage statistics have been reset.")
}

// DisplayUsageAfterResponse renders and displays token usage information immediately
// following an AI response. This provides real-time feedback about the cost and
// token consumption of each interaction.
func (c *CLI) DisplayUsageAfterResponse() {
	if c.usageTracker == nil {
		return
	}

	usageInfo := c.usageTracker.RenderUsageInfo()
	if usageInfo != "" {
		paddedUsage := lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingTop(1).
			Render(usageInfo)
		fmt.Print(paddedUsage)
		c.usageDisplayed = true
	}
}

// updateSize updates the CLI size based on terminal dimensions
func (c *CLI) updateSize() {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		c.width = 80  // Fallback width
		c.height = 24 // Fallback height
		return
	}

	// Add left and right padding (4 characters total: 2 on each side)
	paddingTotal := 4
	c.width = width - paddingTotal
	c.height = height

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
