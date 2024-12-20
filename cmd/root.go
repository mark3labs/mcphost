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
	"github.com/mark3labs/mcphost/pkg/history"
	"github.com/mark3labs/mcphost/pkg/llm"
	"github.com/mark3labs/mcphost/pkg/llm/anthropic"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	renderer      *glamour.TermRenderer
	configFile    string
	messageWindow int
	modelFlag     string // New flag for model selection
)

// Message types
type simpleMessage struct {
	role           string
	content        string
	toolID         string
	isToolResponse bool
	toolCalls      []history.ContentBlock
}

func (m *simpleMessage) GetRole() string    { return m.role }
func (m *simpleMessage) GetContent() string { return m.content }
func (m *simpleMessage) GetToolCalls() []llm.ToolCall {
	var calls []llm.ToolCall
	for _, block := range m.toolCalls {
		if block.Type == "tool_use" {
			calls = append(calls, &toolCall{
				id:   block.ID,
				name: block.Name,
				args: block.Input,
			})
		}
	}
	return calls
}
func (m *simpleMessage) IsToolResponse() bool      { return m.isToolResponse }
func (m *simpleMessage) GetToolResponseID() string { return m.toolID }
func (m *simpleMessage) GetUsage() (int, int)      { return 0, 0 }

type toolCall struct {
	id   string
	name string
	args json.RawMessage
}

func (t *toolCall) GetID() string   { return t.id }
func (t *toolCall) GetName() string { return t.name }
func (t *toolCall) GetArguments() map[string]interface{} {
	var args map[string]interface{}
	if err := json.Unmarshal(t.args, &args); err != nil {
		return make(map[string]interface{})
	}
	return args
}

const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
	maxRetries     = 5 // Will reach close to max backoff
)

var rootCmd = &cobra.Command{
	Use:   "mcphost",
	Short: "Chat with AI models through a unified interface",
	Long: `MCPHost is a CLI tool that allows you to interact with various AI models
through a unified interface. It supports various tools through MCP servers
and provides streaming responses.

Available models can be specified using the --model flag:
- Anthropic Claude (default): anthropic:claude-3-5-sonnet-latest
- Ollama models: ollama:modelname

Example:
  mcphost -m ollama:qwen2.5:3b`,
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
	rootCmd.PersistentFlags().
		StringVarP(&modelFlag, "model", "m", "anthropic:claude-3-5-sonnet-latest",
			"model to use (format: provider:model, e.g. anthropic:claude-3-5-sonnet-latest or ollama:qwen2.5:3b)")
}

// Add new function to create provider
func createProvider(modelString string) (llm.Provider, error) {
	parts := strings.Split(modelString, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf(
			"invalid model format. Expected provider:model, got %s",
			modelString,
		)
	}

	provider := parts[0]
	model := strings.Join(parts[1:], ":")

	switch provider {
	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf(
				"ANTHROPIC_API_KEY environment variable not set",
			)
		}
		return anthropic.NewProvider(apiKey), nil

	case "ollama":
		return llm.NewOllamaProvider(model)

	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func pruneMessages(messages []history.HistoryMessage) []history.HistoryMessage {
	if len(messages) <= messageWindow {
		return messages
	}

	// Keep only the most recent messages based on window size
	messages = messages[len(messages)-messageWindow:]

	// Handle messages
	toolUseIds := make(map[string]bool)
	toolResultIds := make(map[string]bool)

	// First pass: collect all tool use and result IDs
	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.Type == "tool_use" {
				toolUseIds[block.ID] = true
			} else if block.Type == "tool_result" {
				toolResultIds[block.ToolUseID] = true
			}
		}
	}

	// Second pass: filter out orphaned tool calls/results
	var prunedMessages []history.HistoryMessage
	for _, msg := range messages {
		var prunedBlocks []history.ContentBlock
		for _, block := range msg.Content {
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
		if (len(prunedBlocks) > 0 && msg.Role == "assistant") ||
			msg.Role != "assistant" {
			hasTextBlock := false
			for _, block := range msg.Content {
				if block.Type == "text" {
					hasTextBlock = true
					break
				}
			}
			if len(prunedBlocks) > 0 || hasTextBlock {
				msg.Content = prunedBlocks
				prunedMessages = append(prunedMessages, msg)
			}
		}
	}
	return prunedMessages
}

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80 // Fallback width
	}
	return width - 20
}

func handleHistoryCommand(messages []history.HistoryMessage) {
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

// Method implementations for simpleMessage
func runPrompt(
	provider llm.Provider,
	mcpClients map[string]*mcpclient.StdioMCPClient,
	tools []llm.Tool,
	prompt string,
	messages *[]history.HistoryMessage,
) error {
	// Display the user's prompt if it's not empty (i.e., not a tool response)
	if prompt != "" {
		fmt.Printf("\n%s\n", promptStyle.Render("You: "+prompt))
		*messages = append(
			*messages,
			history.HistoryMessage{
				Role: "user",
				Content: []history.ContentBlock{{
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
	// Messages already implement llm.Message interface
	llmMessages := make([]llm.Message, len(*messages))
	for i := range *messages {
		llmMessages[i] = &(*messages)[i]
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

	toolResults := []history.ContentBlock{}

	var messageContent []history.ContentBlock

	// Handle the message response
	if str, err := renderer.Render("\nAssistant: "); err == nil {
		fmt.Print(str)
	}

	toolResults = []history.ContentBlock{}
	messageContent = []history.ContentBlock{}

	// Add text content
	if message.GetContent() != "" {
		if err := updateRenderer(); err != nil {
			return fmt.Errorf("error updating renderer: %v", err)
		}
		str, err := renderer.Render(message.GetContent() + "\n")
		if err != nil {
			log.Error("Failed to render response", "error", err)
			fmt.Print(message.GetContent() + "\n")
		} else {
			fmt.Print(str)
		}
		messageContent = append(messageContent, history.ContentBlock{
			Type: "text",
			Text: message.GetContent(),
		})
	}

	// Handle tool calls
	for _, toolCall := range message.GetToolCalls() {
		log.Info("🔧 Using tool", "name", toolCall.GetName())

		input, _ := json.Marshal(toolCall.GetArguments())
		messageContent = append(messageContent, history.ContentBlock{
			Type:  "tool_use",
			ID:    toolCall.GetID(),
			Name:  toolCall.GetName(),
			Input: input,
		})

		// Log usage statistics if available
		inputTokens, outputTokens := message.GetUsage()
		if inputTokens > 0 || outputTokens > 0 {
			log.Info("Usage statistics",
				"input_tokens", inputTokens,
				"output_tokens", outputTokens,
				"total_tokens", inputTokens+outputTokens)
		}

		parts := strings.Split(toolCall.GetName(), "__")
		if len(parts) != 2 {
			fmt.Printf(
				"Error: Invalid tool name format: %s\n",
				toolCall.GetName(),
			)
			continue
		}

		serverName, toolName := parts[0], parts[1]
		mcpClient, ok := mcpClients[serverName]
		if !ok {
			fmt.Printf("Error: Server not found: %s\n", serverName)
			continue
		}

		var toolArgs map[string]interface{}
		if err := json.Unmarshal(input, &toolArgs); err != nil {
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
			toolResults = append(toolResults, history.ContentBlock{
				Type:      "tool_result",
				ToolUseID: toolCall.GetID(),
				Content: []history.ContentBlock{{
					Type: "text",
					Text: errMsg,
				}},
			})
			continue
		}

		toolResult := *toolResultPtr
		// Add the tool result directly to messages array
		resultJSON, err := json.Marshal(toolResult.Content)
		if err != nil {
			errMsg := fmt.Sprintf("Error marshaling tool result: %v", err)
			fmt.Printf("\n%s\n", errorStyle.Render(errMsg))
			continue
		}

		// Parse the tool result
		var resultText string
		var parsedResult struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}

		if err := json.Unmarshal(resultJSON, &parsedResult); err != nil {
			// If we can't parse it as expected format, use error message
			errMsg := fmt.Sprintf("Error parsing tool result: %v", err)
			fmt.Printf("\n%s\n", errorStyle.Render(errMsg))
			resultText = errMsg
		} else {
			if parsedResult.Type != "text" {
				errMsg := fmt.Sprintf("Unsupported tool result type: %s", parsedResult.Type)
				log.Warn(errMsg)
				resultText = errMsg
			} else {
				resultText = parsedResult.Text
			}
		}

		toolResults = append(toolResults, history.ContentBlock{
			Type:      "tool_result",
			ToolUseID: toolCall.GetID(),
			Content:   resultText,
		})
		log.Printf("Debug - Created Tool Result Block: %+v", toolResults[len(toolResults)-1])
	}

	*messages = append(*messages, history.HistoryMessage{
		Role:    message.GetRole(),
		Content: messageContent,
	})

	if len(toolResults) > 0 {
		*messages = append(*messages, history.HistoryMessage{
			Role:    "user",
			Content: toolResults,
		})
		// Make another call to get Claude's response to the tool results
		return runPrompt(provider, mcpClients, tools, "", messages)
	}

	fmt.Println() // Add spacing
	return nil
}

func runMCPHost() error {
	// Create the provider based on the model flag
	provider, err := createProvider(modelFlag)
	if err != nil {
		return fmt.Errorf("error creating provider: %v", err)
	}

	// Split the model flag and get just the model name
	modelName := strings.Split(modelFlag, ":")[1]
	log.Info("Model loaded",
		"provider", provider.Name(),
		"model", modelName)

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

	messages := make([]history.HistoryMessage, 0)

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
