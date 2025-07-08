package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/cloudwego/eino/schema"
	"golang.org/x/term"
)

var (
	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)

// CLI handles the command line interface with improved message rendering
type CLI struct {
	messageRenderer  *MessageRenderer
	compactRenderer  *CompactRenderer // Add compact renderer
	messageContainer *MessageContainer
	usageTracker     *UsageTracker
	width            int
	height           int
	compactMode      bool   // Add compact mode flag
	modelName        string // Store current model name
	lastStreamHeight int    // track how far back we need to move the cursor to overwrite streaming messages
	usageDisplayed   bool   // track if usage info was displayed after last assistant message
}

// NewCLI creates a new CLI instance with message container
func NewCLI(debug bool, compact bool) (*CLI, error) {
	cli := &CLI{
		compactMode: compact,
	}
	cli.updateSize()
	cli.messageRenderer = NewMessageRenderer(cli.width, debug)
	cli.compactRenderer = NewCompactRenderer(cli.width, debug)
	cli.messageContainer = NewMessageContainer(cli.width, cli.height-4, compact) // Pass compact mode

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

// GetPrompt gets user input using the huh library with divider and padding
func (c *CLI) GetPrompt() (string, error) {
	// Usage info is now displayed immediately after responses via DisplayUsageAfterResponse()
	// No need to display it here to avoid duplication

	c.messageContainer.messages = nil // clear previous messages (they should have been printed already)
	c.lastStreamHeight = 0            // Reset last stream height for new prompt

	// No divider needed - removed for cleaner appearance

	var prompt string
	err := huh.NewForm(huh.NewGroup(huh.NewText().
		Title("Enter your prompt (Type /help for commands, Ctrl+C to quit, ESC to cancel generation)").
		Value(&prompt).
		CharLimit(5000)),
	).WithWidth(c.width).
		WithTheme(huh.ThemeCharm()).
		Run()

	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", io.EOF // Signal clean exit
		}
		return "", err
	}

	return prompt, nil
}

// ShowSpinner displays a spinner with the given message and executes the action
func (c *CLI) ShowSpinner(message string, action func() error) error {
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
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
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
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
	return nil
}

// DisplayToolCallMessage displays a tool call in progress
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

// DisplayToolMessage displays a tool call message
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

// StartStreamingMessage starts a streaming assistant message
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

// UpdateStreamingMessage updates the streaming message with new content
func (c *CLI) UpdateStreamingMessage(content string) {
	// Update the last message (which should be the streaming assistant message)
	c.messageContainer.UpdateLastMessage(content)
	c.displayContainer()
}

// DisplayError displays an error message using the appropriate renderer
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

// DisplayInfo displays an informational message using the appropriate renderer
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

// DisplayCancellation displays a cancellation message
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

// DisplayDebugConfig displays configuration settings using the appropriate renderer
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

	fmt.Println("\nConversation History:")
	fmt.Println(historyContainer.Render())
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
		fmt.Println("\nGoodbye!")
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

// DisplayUsageAfterResponse displays usage information immediately after a response
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
