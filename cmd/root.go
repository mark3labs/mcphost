package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcphost/internal/agent"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/ui"
	"github.com/spf13/cobra"
)

var (
	configFile       string
	systemPromptFile string
	messageWindow    int
	modelFlag        string
	openaiBaseURL    string
	anthropicBaseURL string
	openaiAPIKey     string
	anthropicAPIKey  string
	googleAPIKey     string
	debugMode        bool
)

var rootCmd = &cobra.Command{
	Use:   "mcphost",
	Short: "Chat with AI models through a unified interface",
	Long: `MCPHost is a CLI tool that allows you to interact with various AI models
through a unified interface. It supports various tools through MCP servers
and provides streaming responses.

Available models can be specified using the --model flag:
- Anthropic Claude (default): anthropic:claude-sonnet-4-20250514
- OpenAI: openai:gpt-4
- Ollama models: ollama:modelname
- Google: google:modelname

Example:
  mcphost -m ollama:qwen2.5:3b
  mcphost -m openai:gpt-4
  mcphost -m google:gemini-2.0-flash`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPHost(context.Background())
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file (default is $HOME/.mcp.json)")
	rootCmd.PersistentFlags().
		StringVar(&systemPromptFile, "system-prompt", "", "system prompt json file")
	rootCmd.PersistentFlags().
		IntVar(&messageWindow, "message-window", 10, "number of messages to keep in context")
	rootCmd.PersistentFlags().
		StringVarP(&modelFlag, "model", "m", "anthropic:claude-sonnet-4-20250514",
			"model to use (format: provider:model)")
	rootCmd.PersistentFlags().
		BoolVar(&debugMode, "debug", false, "enable debug logging")

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&openaiBaseURL, "openai-url", "", "base URL for OpenAI API")
	flags.StringVar(&anthropicBaseURL, "anthropic-url", "", "base URL for Anthropic API")
	flags.StringVar(&openaiAPIKey, "openai-api-key", "", "OpenAI API key")
	flags.StringVar(&anthropicAPIKey, "anthropic-api-key", "", "Anthropic API key")
	flags.StringVar(&googleAPIKey, "google-api-key", "", "Google (Gemini) API key")
}

func runMCPHost(ctx context.Context) error {
	// Set up logging
	if debugMode {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// Load configuration
	mcpConfig, err := config.LoadMCPConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load MCP config: %v", err)
	}

	systemPrompt, err := config.LoadSystemPrompt(systemPromptFile)
	if err != nil {
		return fmt.Errorf("failed to load system prompt: %v", err)
	}

	// Create model configuration
	modelConfig := &models.ProviderConfig{
		ModelString:      modelFlag,
		SystemPrompt:     systemPrompt,
		AnthropicAPIKey:  anthropicAPIKey,
		AnthropicBaseURL: anthropicBaseURL,
		OpenAIAPIKey:     openaiAPIKey,
		OpenAIBaseURL:    openaiBaseURL,
		GoogleAPIKey:     googleAPIKey,
	}

	// Create agent configuration
	agentConfig := &agent.Config{
		ModelConfig:   modelConfig,
		MCPConfig:     mcpConfig,
		SystemPrompt:  systemPrompt,
		MaxSteps:      20,
		MessageWindow: messageWindow,
	}

	// Create the agent
	mcpAgent, err := agent.NewMCPAgent(ctx, agentConfig)
	if err != nil {
		return fmt.Errorf("failed to create agent: %v", err)
	}
	defer mcpAgent.Close()

	// Create CLI interface
	cli, err := ui.NewCLI()
	if err != nil {
		return fmt.Errorf("failed to create CLI: %v", err)
	}

	// Log successful initialization
	parts := strings.SplitN(modelFlag, ":", 2)
	modelName := "Unknown"
	if len(parts) == 2 {
		modelName = parts[1]
		cli.DisplayInfo(fmt.Sprintf("Model loaded: %s (%s)", parts[0], parts[1]))
	}

	tools := mcpAgent.GetTools()
	cli.DisplayInfo(fmt.Sprintf("Loaded %d tools from MCP servers", len(tools)))

	// Prepare data for slash commands
	var serverNames []string
	for name := range mcpConfig.MCPServers {
		serverNames = append(serverNames, name)
	}

	var toolNames []string
	for _, tool := range tools {
		if info, err := tool.Info(ctx); err == nil {
			toolNames = append(toolNames, info.Name)
		}
	}

	// Main interaction loop
	var messages []*schema.Message
	for {
		// Get user input
		prompt, err := cli.GetPrompt()
		if err == io.EOF {
			fmt.Println("\nGoodbye!")
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to get prompt: %v", err)
		}

		if prompt == "" {
			continue
		}

		// Handle slash commands
		if cli.IsSlashCommand(prompt) {
			if cli.HandleSlashCommand(prompt, serverNames, toolNames, messages) {
				continue
			}
			cli.DisplayError(fmt.Errorf("unknown command: %s", prompt))
			continue
		}

		// Display user message
		cli.DisplayUserMessage(prompt)

		// Add user message to history
		messages = append(messages, schema.UserMessage(prompt))

		// Prune messages if needed
		if len(messages) > messageWindow {
			messages = messages[len(messages)-messageWindow:]
		}

		// Get agent response with spinner and callbacks
		var response *schema.Message
		err = cli.ShowSpinner("Thinking...", func() error {
			var spinnerErr error
			// Create callback handler to capture tool calls in real-time
			callbackHandler := cli.CreateCallbackHandler()
			
			// Use the agent with callbacks to capture tool calls as they happen
			response, spinnerErr = mcpAgent.Generate(ctx, messages, compose.WithCallbacks(callbackHandler))
			return spinnerErr
		})
		if err != nil {
			cli.DisplayError(fmt.Errorf("agent error: %v", err))
			continue
		}

		// Display assistant response with model name
		if err := cli.DisplayAssistantMessageWithModel(response.Content, modelName); err != nil {
			cli.DisplayError(fmt.Errorf("display error: %v", err))
		}

		// Add assistant response to history
		messages = append(messages, response)
	}
}