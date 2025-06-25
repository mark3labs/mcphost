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
	messageContainer *MessageContainer
	usageTracker     *UsageTracker
	width            int
	height           int
}

// NewCLI creates a new CLI instance with message container
func NewCLI(debug bool) (*CLI, error) {
	cli := &CLI{}
	cli.updateSize()
	cli.messageRenderer = NewMessageRenderer(cli.width, debug)
	cli.messageContainer = NewMessageContainer(cli.width, cli.height-4) // Reserve space for input and help

	return cli, nil
}

// SetUsageTracker sets the usage tracker for the CLI
func (c *CLI) SetUsageTracker(tracker *UsageTracker) {
	c.usageTracker = tracker
	if c.usageTracker != nil {
		c.usageTracker.SetWidth(c.width)
	}
}

// GetPrompt gets user input using the huh library with divider and padding
func (c *CLI) GetPrompt() (string, error) {
	// Display usage info if available
	if c.usageTracker != nil {
		usageInfo := c.usageTracker.RenderUsageInfo()
		if usageInfo != "" {
			paddedUsage := lipgloss.NewStyle().
				PaddingLeft(2).
				Render(usageInfo)
			fmt.Print(paddedUsage)
		}
	}

	// Create an enhanced divider with gradient effect
	theme := GetTheme()
	dividerStyle := lipgloss.NewStyle().
		Width(c.width).
		BorderTop(true).
		BorderStyle(lipgloss.Border{
			Top: "━",
		}).
		BorderForeground(theme.Border).
		MarginTop(1).
		MarginBottom(1).
		PaddingLeft(2)

	// Render the enhanced input section
	fmt.Print(dividerStyle.Render(""))

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

// DisplayUserMessage displays the user's message using the new renderer
func (c *CLI) DisplayUserMessage(message string) {
	msg := c.messageRenderer.RenderUserMessage(message, time.Now())
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayAssistantMessage displays the assistant's message using the new renderer
func (c *CLI) DisplayAssistantMessage(message string) error {
	return c.DisplayAssistantMessageWithModel(message, "")
}

// DisplayAssistantMessageWithModel displays the assistant's message with model info
func (c *CLI) DisplayAssistantMessageWithModel(message, modelName string) error {
	msg := c.messageRenderer.RenderAssistantMessage(message, time.Now(), modelName)
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
	return nil
}

// DisplayToolCallMessage displays a tool call in progress
func (c *CLI) DisplayToolCallMessage(toolName, toolArgs string) {
	msg := c.messageRenderer.RenderToolCallMessage(toolName, toolArgs, time.Now())

	// Always display immediately - spinner management is handled externally
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayToolMessage displays a tool call message
func (c *CLI) DisplayToolMessage(toolName, toolArgs, toolResult string, isError bool) {
	msg := c.messageRenderer.RenderToolMessage(toolName, toolArgs, toolResult, isError)

	// Always display immediately - spinner management is handled externally
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayStreamingMessage displays streaming content
func (c *CLI) DisplayStreamingMessage(reader *schema.StreamReader[*schema.Message]) error {
	// For streaming, we'll collect the content and then display it
	var content strings.Builder

	for {
		msg, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream receive error: %v", err)
		}
		content.WriteString(msg.Content)
	}

	return c.DisplayAssistantMessage(content.String())
}

// DisplayError displays an error message using the message component
func (c *CLI) DisplayError(err error) {
	msg := c.messageRenderer.RenderErrorMessage(err.Error(), time.Now())
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayInfo displays an informational message using the system message component
func (c *CLI) DisplayInfo(message string) {
	msg := c.messageRenderer.RenderSystemMessage(message, time.Now())
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayCancellation displays a cancellation message
func (c *CLI) DisplayCancellation() {
	msg := c.messageRenderer.RenderSystemMessage("Generation cancelled by user (ESC pressed)", time.Now())
	c.messageContainer.AddMessage(msg)
	c.displayContainer()
}

// DisplayDebugConfig displays configuration settings in debug mode using tool response block styling
func (c *CLI) DisplayDebugConfig(config map[string]any) {
	msg := c.messageRenderer.RenderDebugConfigMessage(config, time.Now())
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
	historyContainer := NewMessageContainer(c.width, c.height-4)

	for _, msg := range messages {
		switch msg.Role {
		case schema.User:
			uiMsg := c.messageRenderer.RenderUserMessage(msg.Content, time.Now())
			historyContainer.AddMessage(uiMsg)
		case schema.Assistant:
			uiMsg := c.messageRenderer.RenderAssistantMessage(msg.Content, time.Now(), "")
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

// HandleSlashCommand handles slash commands and returns true if handled
func (c *CLI) HandleSlashCommand(input string, servers []string, tools []string, history []*schema.Message) bool {
	switch input {
	case "/help":
		c.DisplayHelp()
		return true
	case "/tools":
		c.DisplayTools(tools)
		return true
	case "/servers":
		c.DisplayServers(servers)
		return true
	case "/history":
		c.DisplayHistory(history)
		return true
	case "/clear":
		c.ClearMessages()
		return true
	case "/usage":
		c.DisplayUsageStats()
		return true
	case "/reset-usage":
		c.ResetUsageStats()
		return true
	case "/quit":
		fmt.Println("\nGoodbye!")
		os.Exit(0)
		return true
	default:
		return false
	}
}

// ClearMessages clears all messages from the container
func (c *CLI) ClearMessages() {
	c.messageContainer.Clear()
	c.displayContainer()
}

// displayContainer renders and displays the message container
func (c *CLI) displayContainer() {
	// Clear screen and display messages
	fmt.Print("\033[2J\033[H") // Clear screen and move cursor to top

	// Add left padding to the entire container
	content := c.messageContainer.Render()
	paddedContent := lipgloss.NewStyle().
		PaddingLeft(2).
		Render(content)

	fmt.Print(paddedContent)
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

		// Handle cache tokens if available (some providers support this)
		cacheReadTokens := 0
		cacheWriteTokens := 0

		c.usageTracker.UpdateUsage(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
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

	msg := c.messageRenderer.RenderSystemMessage(content.String(), time.Now())
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
	if c.messageContainer != nil {
		c.messageContainer.SetSize(c.width, c.height-4)
	}
	if c.usageTracker != nil {
		c.usageTracker.SetWidth(c.width)
	}
}
