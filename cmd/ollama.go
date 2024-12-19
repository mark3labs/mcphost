package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcphost/pkg/llm"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/log"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	api "github.com/ollama/ollama/api"
	"github.com/spf13/cobra"
)

var (
	modelName string
	ollamaCmd = &cobra.Command{
		Use:   "ollama",
		Short: "Chat using an Ollama model",
		Long:  `Use a local Ollama model for chat with MCP tool support`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOllama()
		},
	}
)

func modelSupportsTools(client *api.Client, modelName string) bool {
	resp, err := client.Show(context.Background(), &api.ShowRequest{
		Model: modelName,
	})

	if err != nil {
		log.Error("Failed to get model info", "error", err)
		return false
	}

	// Check if model details indicate function calling support
	// This looks for function calling capability in model details
	if resp.Modelfile != "" {
		if strings.Contains(resp.Modelfile, "<tools>") {
			return true
		}
	}

	return false
}

func runLLMPrompt(
	provider llm.Provider,
	mcpClients map[string]*mcpclient.StdioMCPClient,
	tools []llm.Tool,
	prompt string,
	messages []llm.Message,
) error {
	if prompt != "" {
		fmt.Printf("\n%s\n", promptStyle.Render("You: "+prompt))
	}

	var err error
	var response llm.Message

	action := func() {
		response, err = provider.CreateMessage(
			context.Background(),
			prompt,
			messages,
			tools,
		)
	}

	_ = spinner.New().Title("Thinking...").Action(action).Run()
	if err != nil {
		return err
	}

	fmt.Print(responseStyle.Render("\nAssistant: "))
	if err := updateRenderer(); err != nil {
		return fmt.Errorf("error updating renderer: %v", err)
	}

	rendered, err := renderer.Render(response.GetContent() + "\n")
	if err != nil {
		log.Error("Failed to render response", "error", err)
		fmt.Print(response.GetContent() + "\n")
	} else {
		fmt.Print(rendered)
	}

	messages = append(messages, response)

	// Handle tool calls
	for _, toolCall := range response.GetToolCalls() {
		log.Info("🔧 Using tool", "name", toolCall.GetName())

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

		var toolResult *mcp.CallToolResult
		action := func() {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				10*time.Second,
			)
			defer cancel()

			req := mcp.CallToolRequest{}
			req.Params.Name = toolName
			req.Params.Arguments = toolCall.GetArguments()
			toolResult, err = mcpClient.CallTool(ctx, req)
		}

		_ = spinner.New().
			Title(fmt.Sprintf("Running tool %s...", toolName)).
			Action(action).
			Run()

		if err != nil {
			fmt.Printf("\n%s\n", errorStyle.Render(
				fmt.Sprintf("Error calling tool %s: %v", toolName, err),
			))
			continue
		}

		// Create a tool response message
		toolResponseMsg := &llm.OllamaMessage{
			Message: api.Message{
				Role:    "tool",
				Content: fmt.Sprintf("%v", toolResult.Content),
			},
		}
		messages = append(messages, toolResponseMsg)

		// Make another call to get the model's response to the tool result
		return runLLMPrompt(provider, mcpClients, tools, "", messages)
	}

	fmt.Println() // Add spacing
	return nil
}

func init() {
	ollamaCmd.Flags().
		StringVar(&modelName, "model", "", "Ollama model to use (required)")
	_ = ollamaCmd.MarkFlagRequired("model")
	rootCmd.AddCommand(ollamaCmd)
}

func mcpToolsToLLMTools(serverName string, mcpTools []mcp.Tool) []llm.Tool {
	llmTools := make([]llm.Tool, len(mcpTools))
	for i, tool := range mcpTools {
		namespacedName := fmt.Sprintf("%s__%s", serverName, tool.Name)
		llmTools[i] = llm.Tool{
			Name:        namespacedName,
			Description: tool.Description,
			InputSchema: llm.Schema{
				Type:       tool.InputSchema.Type,
				Properties: tool.InputSchema.Properties,
				Required:   tool.InputSchema.Required,
			},
		}
	}
	return llmTools
}

func mcpToolsToOllamaTools(serverName string, mcpTools []mcp.Tool) []api.Tool {
	ollamaTools := make([]api.Tool, len(mcpTools))

	for i, tool := range mcpTools {
		namespacedName := fmt.Sprintf("%s__%s", serverName, tool.Name)

		ollamaTools[i] = api.Tool{
			Type: "function",
			Function: api.ToolFunction{
				Name:        namespacedName,
				Description: tool.Description,
				Parameters: struct {
					Type       string   `json:"type"`
					Required   []string `json:"required"`
					Properties map[string]struct {
						Type        string   `json:"type"`
						Description string   `json:"description"`
						Enum        []string `json:"enum,omitempty"`
					} `json:"properties"`
				}{
					Type:     tool.InputSchema.Type,
					Required: tool.InputSchema.Required,
					Properties: make(map[string]struct {
						Type        string   `json:"type"`
						Description string   `json:"description"`
						Enum        []string `json:"enum,omitempty"`
					}),
				},
			},
		}

		// Convert properties
		for propName, prop := range tool.InputSchema.Properties {
			propMap, ok := prop.(map[string]interface{})
			if !ok {
				log.Error("Invalid property type", "property", propName)
				continue
			}

			propType, _ := propMap["type"].(string)
			propDesc, _ := propMap["description"].(string)
			propEnumRaw, hasEnum := propMap["enum"]

			var enumVals []string
			if hasEnum {
				if enumSlice, ok := propEnumRaw.([]interface{}); ok {
					enumVals = make([]string, len(enumSlice))
					for i, v := range enumSlice {
						if str, ok := v.(string); ok {
							enumVals[i] = str
						}
					}
				}
			}

			ollamaTools[i].Function.Parameters.Properties[propName] = struct {
				Type        string   `json:"type"`
				Description string   `json:"description"`
				Enum        []string `json:"enum,omitempty"`
			}{
				Type:        propType,
				Description: propDesc,
				Enum:        enumVals,
			}
		}
	}

	return ollamaTools
}

func runOllama() error {
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

	provider, err := llm.NewOllamaProvider(modelName)
	if err != nil {
		return fmt.Errorf("error creating Ollama provider: %v", err)
	}

	if !provider.SupportsTools() {
		fmt.Printf("\n%s\n\n",
			errorStyle.Render(fmt.Sprintf(
				"Warning: Model %s does not support function calling. Tools will be disabled.",
				modelName,
			)),
		)
		mcpClients = nil // Disable tools
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

		serverTools := mcpToolsToLLMTools(serverName, toolsResult.Tools)
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

	// Initialize messages with system prompt
	var messages []llm.Message
	messages = append(messages, &llm.OllamaMessage{
		Message: api.Message{
			Role: "system",
			Content: `You are a helpful AI assistant with access to external tools. Respond directly to questions and requests.
Only use tools when specifically needed to accomplish a task. If you can answer without using tools, do so.
When you do need to use a tool, explain what you're doing first.`,
		},
	})

	if len(messages) > 0 {
		var ollamaMessages []api.Message
		for _, msg := range messages {
			if ollamaMsg, ok := msg.(*llm.OllamaMessage); ok {
				ollamaMessages = append(ollamaMessages, ollamaMsg.Message)
			}
		}
		pruned := pruneMessages(ollamaMessages)
		newMessages := make([]llm.Message, len(pruned))
		for i, msg := range pruned {
			newMessages[i] = &llm.OllamaMessage{Message: msg}
		}
		messages = newMessages
	}

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
		).WithWidth(width)

		err := form.Run()
		if err != nil {
			if err.Error() == "user aborted" {
				fmt.Println("\nGoodbye!")
				return nil
			}
			return err
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
			var ollamaMessages []api.Message
			for _, msg := range messages {
				if ollamaMsg, ok := msg.(*llm.OllamaMessage); ok {
					ollamaMessages = append(ollamaMessages, ollamaMsg.Message)
				}
			}
			pruned := pruneMessages(ollamaMessages)
			newMessages := make([]llm.Message, len(pruned))
			for i, msg := range pruned {
				newMessages[i] = &llm.OllamaMessage{Message: msg}
			}
			messages = newMessages
		}

		err = runLLMPrompt(provider, mcpClients, allTools, prompt, messages)
		if err != nil {
			return err
		}
	}
}
