package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/log"

	"github.com/charmbracelet/glamour"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcphost/pkg/llm"
	"github.com/ollama/ollama/api"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func convertAnthropicContentToContentBlock(content []llm.AnthropicContent) []ContentBlock {
	blocks := make([]ContentBlock, len(content))
	for i, block := range content {
		blocks[i] = ContentBlock{
			Type:      block.Type,
			Text:      block.Text,
			ID:        block.ID,
			ToolUseID: block.ToolUseID,
			Name:      block.Name,
			Input:     block.Input,
			Content:   block.Content,
		}
	}
	return blocks
}

var (
	renderer *glamour.TermRenderer

	configFile    string
	messageWindow int
)

const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
	maxRetries     = 5 // Will reach close to max backoff
)

var rootCmd = &cobra.Command{
	Use:   "mcphost",
	Short: "Chat with Claude 3.5 Sonnet or Ollama models",
	Long: `MCPHost is a CLI tool that allows you to interact with Claude 3.5 Sonnet or Ollama models.
It supports various tools through MCP servers and provides streaming responses.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPHost()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file (default is $HOME/mcp.json)")
	rootCmd.PersistentFlags().
		IntVar(&messageWindow, "message-window", 10, "number of messages to keep in context")
}

func pruneMessages[T MessageParam | api.Message](messages []T) []T {
	if len(messages) <= messageWindow {
		return messages
	}

	// Keep only the most recent messages based on window size
	messages = messages[len(messages)-messageWindow:]

	switch any(messages[0]).(type) {
	case MessageParam:
		// Handle Anthropic messages
		toolUseIds := make(map[string]bool)
		toolResultIds := make(map[string]bool)

		// First pass: collect all tool use and result IDs
		for _, msg := range messages {
			m := any(msg).(MessageParam)
			for _, block := range m.Content {
				if block.Type == "tool_use" {
					toolUseIds[block.ID] = true
				} else if block.Type == "tool_result" {
					toolResultIds[block.ToolUseID] = true
				}
			}
		}

		// Second pass: filter out orphaned tool calls/results
		var prunedMessages []T
		for _, msg := range messages {
			m := any(msg).(MessageParam)
			var prunedBlocks []ContentBlock
			for _, block := range m.Content {
				keep := true
				if block.Type == "tool_use" {
					keep = toolResultIds[block.ID]
				} else if block.Type == "tool_result" {
					keep = toolUseIds[block.ToolUseID]
				}
				if keep {
					prunedBlocks = append(prunedBlocks, block)
				}
			}
			// Only include messages that have content or are not assistant messages
			if (len(prunedBlocks) > 0 && m.Role == "assistant") || m.Role != "assistant" {
				hasTextBlock := false
				for _, block := range m.Content {
					if block.Type == "text" {
						hasTextBlock = true
						break
					}
				}
				if len(prunedBlocks) > 0 || hasTextBlock {
					m.Content = prunedBlocks
					prunedMessages = append(prunedMessages, any(m).(T))
				}
			}
		}
		return prunedMessages

	case api.Message:
		// Handle Ollama messages
		var prunedMessages []T
		for i, msg := range messages {
			m := any(msg).(api.Message)

			// If this message has tool calls, ensure we keep the next message (tool response)
			if len(m.ToolCalls) > 0 {
				if i+1 < len(messages) {
					next := any(messages[i+1]).(api.Message)
					if next.Role == "tool" {
						prunedMessages = append(prunedMessages, msg)
						prunedMessages = append(prunedMessages, messages[i+1])
						continue
					}
				}
				// If no matching tool response, skip this message
				continue
			}

			// Skip tool responses that don't have a preceding tool call
			if m.Role == "tool" {
				if i > 0 {
					prev := any(messages[i-1]).(api.Message)
					if len(prev.ToolCalls) > 0 {
						continue // Already handled in the tool call case
					}
				}
				continue // Skip orphaned tool response
			}

			// Keep all other messages
			prunedMessages = append(prunedMessages, msg)
		}
		return prunedMessages
	}

	return messages
}

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80 // Fallback width
	}
	return width - 20
}

func handleHistoryCommand(messages interface{}) {
	displayMessageHistory(messages)
}

func updateRenderer() error {
	width := getTerminalWidth()
	var err error
	renderer, err = glamour.NewTermRenderer(
		glamour.WithStandardStyle(styles.TokyoNightStyle),
		glamour.WithWordWrap(width),
	)
	return err
}

func runPrompt(
	provider llm.Provider,
	mcpClients map[string]*mcpclient.StdioMCPClient,
	tools []llm.Tool,
	prompt string,
	messages *[]MessageParam,
) error {
	// Display the user's prompt if it's not empty (i.e., not a tool response)
	if prompt != "" {
		fmt.Printf("\n%s\n", promptStyle.Render("You: "+prompt))
		*messages = append(
			*messages,
			MessageParam{
				Role: "user",
				Content: []ContentBlock{{
					Type: "text",
					Text: prompt,
				}},
			},
		)
	}

	var message llm.Message
	var err error
	backoff := initialBackoff
	retries := 0

	// Convert MessageParam to llm.Message for provider
	llmMessages := make([]llm.Message, len(*messages))
	for i, msg := range *messages {
		anthropicContent := make([]llm.AnthropicContent, len(msg.Content))
		for j, block := range msg.Content {
			anthropicContent[j] = llm.AnthropicContent{
				Type:      block.Type,
				Text:      block.Text,
				ID:        block.ID,
				ToolUseID: block.ToolUseID,
				Name:      block.Name,
				Input:     block.Input,
				Content:   block.Content,
			}
		}
		
		llmMessages[i] = &llm.AnthropicMessage{
			Msg: llm.AnthropicAPIMessage{
				Role:    msg.Role,
				Content: anthropicContent,
			},
		}
	}

	for {
		action := func() {
			message, err = provider.CreateMessage(
				context.Background(),
				prompt,
				llmMessages,
				tools,
			)
		}
		_ = spinner.New().Title("Thinking...").Action(action).Run()

		if err != nil {
			// Check if it's an overloaded error
			if strings.Contains(err.Error(), "overloaded_error") {
				if retries >= maxRetries {
					return fmt.Errorf(
						"claude is currently overloaded. please wait a few minutes and try again",
					)
				}

				log.Warn("Claude is overloaded, backing off...",
					"attempt", retries+1,
					"backoff", backoff.String())

				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				retries++
				continue
			}
			// If it's not an overloaded error, return the error immediately
			return err
		}
		// If we got here, the request succeeded
		break
	}

	if str, err := renderer.Render("\nClaude: "); err == nil {
		fmt.Print(str)
	}

	toolResults := []ContentBlock{}

	// Type assert to get the concrete AnthropicMessage type
	anthropicMsg, ok := message.(*llm.AnthropicMessage)
	if !ok {
		return fmt.Errorf("unexpected message type: %T", message)
	}

	for _, block := range anthropicMsg.Msg.Content {
		switch block.Type {
		case "text":
			if err := updateRenderer(); err != nil {
				return fmt.Errorf("error updating renderer: %v", err)
			}
			str, err := renderer.Render(block.Text + "\n")
			if err != nil {
				log.Error("Failed to render response", "error", err)
				fmt.Print(block.Text + "\n")
				continue
			}
			fmt.Print(str)

		case "tool_use":
			log.Info("🔧 Using tool", "name", block.Name)

			parts := strings.Split(block.Name, "__")
			if len(parts) != 2 {
				fmt.Printf("Error: Invalid tool name format: %s\n", block.Name)
				continue
			}

			serverName, toolName := parts[0], parts[1]
			mcpClient, ok := mcpClients[serverName]
			if !ok {
				fmt.Printf("Error: Server not found: %s\n", serverName)
				continue
			}

			var toolArgs map[string]interface{}
			if err := json.Unmarshal(block.Input, &toolArgs); err != nil {
				fmt.Printf("Error parsing tool arguments: %v\n", err)
				continue
			}

			var toolResultPtr *mcp.CallToolResult
			action := func() {
				ctx, cancel := context.WithTimeout(
					context.Background(),
					10*time.Second,
				)
				defer cancel()

				req := mcp.CallToolRequest{}
				req.Params.Name = toolName
				req.Params.Arguments = toolArgs
				toolResultPtr, err = mcpClient.CallTool(
					ctx,
					req,
				)
			}
			_ = spinner.New().
				Title(fmt.Sprintf("Running tool %s...", toolName)).
				Action(action).
				Run()

			if err != nil {
				errMsg := fmt.Sprintf(
					"Error calling tool %s: %v",
					toolName,
					err,
				)
				fmt.Printf("\n%s\n", errorStyle.Render(errMsg))

				// Add error message as tool result
				toolResults = append(toolResults, ContentBlock{
					Type:      "tool_result",
					ToolUseID: block.ID,
					Content: []ContentBlock{{
						Type: "text",
						Text: errMsg,
					}},
				})
				continue
			}

			toolResult := *toolResultPtr
			// Add the tool result directly to messages array as JSON string
			resultJSON, err := json.Marshal(toolResult.Content)
			if err != nil {
				errMsg := fmt.Sprintf("Error marshaling tool result: %v", err)
				fmt.Printf("\n%s\n", errorStyle.Render(errMsg))
				continue
			}

			toolResults = append(toolResults, ContentBlock{
				Type:      "tool_result",
				ToolUseID: block.ID,
				Content: []ContentBlock{{
					Type: "text",
					Text: string(resultJSON),
				}},
			})
		}
	}

	*messages = append(*messages, MessageParam{
		Role:    message.GetRole(),
		Content: convertAnthropicContentToContentBlock(anthropicMsg.Msg.Content),
	})

	if len(toolResults) > 0 {
		*messages = append(*messages, MessageParam{
			Role:    "user",
			Content: toolResults,
		})
		// Make another call to get Claude's response to the tool results
		return runPrompt(provider, mcpClients, tools, "", messages)
	}

	inputTokens, outputTokens := message.GetUsage()
	log.Info("Usage statistics",
		"input_tokens", inputTokens,
		"output_tokens", outputTokens,
		"total_tokens", inputTokens+outputTokens)

	fmt.Println() // Add spacing
	return nil
}

func runMCPHost() error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	mcpConfig, err := loadMCPConfig()
	if err != nil {
		return fmt.Errorf("error loading MCP config: %v", err)
	}

	mcpClients, err := createMCPClients(mcpConfig)
	if err != nil {
		return fmt.Errorf("error creating MCP clients: %v", err)
	}

	defer func() {
		log.Info("Shutting down MCP servers...")
		for name, client := range mcpClients {
			if err := client.Close(); err != nil {
				log.Error("Failed to close server", "name", name, "error", err)
			} else {
				log.Info("Server closed", "name", name)
			}
		}
	}()

	for name := range mcpClients {
		log.Info("Server connected", "name", name)
	}

	provider := llm.NewAnthropicProvider(apiKey)

	var allTools []llm.Tool
	for serverName, mcpClient := range mcpClients {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		toolsResult, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
		cancel()

		if err != nil {
			log.Error(
				"Error fetching tools",
				"server",
				serverName,
				"error",
				err,
			)
			continue
		}

		serverTools := mcpToolsToAnthropicTools(serverName, toolsResult.Tools)
		allTools = append(allTools, serverTools...)
		log.Info(
			"Tools loaded",
			"server",
			serverName,
			"count",
			len(toolsResult.Tools),
		)
	}

	if err := updateRenderer(); err != nil {
		return fmt.Errorf("error initializing renderer: %v", err)
	}

	messages := make([]MessageParam, 0)

	// Main interaction loop
	for {
		width := getTerminalWidth()
		var prompt string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewText().
					Key("prompt").
					Title("Enter your prompt (Type /help for commands, Ctrl+C to quit)").
					Value(&prompt),
			),
		).WithWidth(width).WithTheme(huh.ThemeCharm())

		err := form.Run()
		if err != nil {
			// Check if it's a user abort (Ctrl+C)
			if err.Error() == "user aborted" {
				fmt.Println("\nGoodbye!")
				return nil // Exit cleanly
			}
			return err // Return other errors normally
		}

		prompt = form.GetString("prompt")
		if prompt == "" {
			continue
		}

		// Handle slash commands
		handled, err := handleSlashCommand(
			prompt,
			mcpConfig,
			mcpClients,
			messages,
		)
		if err != nil {
			return err
		}
		if handled {
			continue
		}

		if len(messages) > 0 {
			messages = pruneMessages(messages)
		}
		err = runPrompt(provider, mcpClients, allTools, prompt, &messages)
		if err != nil {
			return err
		}
	}
}
