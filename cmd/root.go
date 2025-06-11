package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcphost/internal/agent"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	promptFlag       string
	quietFlag        bool
	maxSteps         int
	scriptMCPConfig  *config.Config // Used to override config in script mode
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

Examples:
  # Interactive mode
  mcphost -m ollama:qwen2.5:3b
  mcphost -m openai:gpt-4
  mcphost -m google:gemini-2.0-flash
  
  # Non-interactive mode
  mcphost -p "What is the weather like today?"
  mcphost -p "Calculate 15 * 23" --quiet
  
  # Script mode
  mcphost script myscript.sh`,
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
		StringVar(&systemPromptFile, "system-prompt", "", "system prompt text or path to system prompt json file")
	rootCmd.PersistentFlags().
		IntVar(&messageWindow, "message-window", 40, "number of messages to keep in context")
	rootCmd.PersistentFlags().
		StringVarP(&modelFlag, "model", "m", "anthropic:claude-sonnet-4-20250514",
			"model to use (format: provider:model)")
	rootCmd.PersistentFlags().
		BoolVar(&debugMode, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().
		StringVarP(&promptFlag, "prompt", "p", "", "run in non-interactive mode with the given prompt")
	rootCmd.PersistentFlags().
		BoolVar(&quietFlag, "quiet", false, "suppress all output (only works with --prompt)")
	rootCmd.PersistentFlags().
		IntVar(&maxSteps, "max-steps", 0, "maximum number of agent steps (0 for unlimited)")

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&openaiBaseURL, "openai-url", "", "base URL for OpenAI API")
	flags.StringVar(&anthropicBaseURL, "anthropic-url", "", "base URL for Anthropic API")
	flags.StringVar(&openaiAPIKey, "openai-api-key", "", "OpenAI API key")
	flags.StringVar(&anthropicAPIKey, "anthropic-api-key", "", "Anthropic API key")
	flags.StringVar(&googleAPIKey, "google-api-key", "", "Google (Gemini) API key")

	// Bind flags to viper for config file support
	viper.BindPFlag("system-prompt", rootCmd.PersistentFlags().Lookup("system-prompt"))
	viper.BindPFlag("message-window", rootCmd.PersistentFlags().Lookup("message-window"))
	viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("max-steps", rootCmd.PersistentFlags().Lookup("max-steps"))
	viper.BindPFlag("openai-url", rootCmd.PersistentFlags().Lookup("openai-url"))
	viper.BindPFlag("anthropic-url", rootCmd.PersistentFlags().Lookup("anthropic-url"))
	viper.BindPFlag("openai-api-key", rootCmd.PersistentFlags().Lookup("openai-api-key"))
	viper.BindPFlag("anthropic-api-key", rootCmd.PersistentFlags().Lookup("anthropic-api-key"))
	viper.BindPFlag("google-api-key", rootCmd.PersistentFlags().Lookup("google-api-key"))

	// Set defaults in viper (lowest precedence)
	viper.SetDefault("model", "anthropic:claude-sonnet-4-20250514")
	viper.SetDefault("message-window", 40)
	viper.SetDefault("max-steps", 0)
	viper.SetDefault("debug", false)
}

func runMCPHost(ctx context.Context) error {
	return runNormalMode(ctx)
}

func runNormalMode(ctx context.Context) error {
	// Validate flag combinations
	if quietFlag && promptFlag == "" {
		return fmt.Errorf("--quiet flag can only be used with --prompt/-p")
	}

	// Set up logging
	if debugMode {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// Load configuration
	var mcpConfig *config.Config
	var err error

	if scriptMCPConfig != nil {
		// Use script-provided config
		mcpConfig = scriptMCPConfig
	} else {
		// Load normal config
		mcpConfig, err = config.LoadMCPConfig(configFile)
		if err != nil {
			return fmt.Errorf("failed to load MCP config: %v", err)
		}
	}

	// Set up viper to read from the same config file for flag values
	if configFile == "" {
		// Use default config file locations
		homeDir, err := os.UserHomeDir()
		if err == nil {
			viper.SetConfigName(".mcphost")
			viper.AddConfigPath(homeDir)
			viper.SetConfigType("yaml")
			if err := viper.ReadInConfig(); err != nil {
				// Try .mcphost.json
				viper.SetConfigType("json")
				if err := viper.ReadInConfig(); err != nil {
					// Try legacy .mcp files
					viper.SetConfigName(".mcp")
					viper.SetConfigType("yaml")
					if err := viper.ReadInConfig(); err != nil {
						viper.SetConfigType("json")
						viper.ReadInConfig() // Ignore error if no config found
					}
				}
			}
		}
	} else {
		// Use specified config file
		viper.SetConfigFile(configFile)
		viper.ReadInConfig() // Ignore error if file doesn't exist
	}

	// Get final values from viper (respects precedence: flag > config > default)
	finalModel := viper.GetString("model")
	finalSystemPrompt := viper.GetString("system-prompt")
	finalMessageWindow := viper.GetInt("message-window")
	finalDebug := viper.GetBool("debug")
	finalMaxSteps := viper.GetInt("max-steps")
	finalOpenAIURL := viper.GetString("openai-url")
	finalAnthropicURL := viper.GetString("anthropic-url")
	finalOpenAIKey := viper.GetString("openai-api-key")
	finalAnthropicKey := viper.GetString("anthropic-api-key")
	finalGoogleKey := viper.GetString("google-api-key")

	// Update debug mode if it was set in config
	if finalDebug && !debugMode {
		debugMode = finalDebug
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	systemPrompt, err := config.LoadSystemPrompt(finalSystemPrompt)
	if err != nil {
		return fmt.Errorf("failed to load system prompt: %v", err)
	}

	// Create model configuration
	modelConfig := &models.ProviderConfig{
		ModelString:      finalModel,
		SystemPrompt:     systemPrompt,
		AnthropicAPIKey:  finalAnthropicKey,
		AnthropicBaseURL: finalAnthropicURL,
		OpenAIAPIKey:     finalOpenAIKey,
		OpenAIBaseURL:    finalOpenAIURL,
		GoogleAPIKey:     finalGoogleKey,
	}

	// Create agent configuration
	agentMaxSteps := finalMaxSteps
	if agentMaxSteps == 0 {
		agentMaxSteps = 1000 // Set a high limit for "unlimited"
	}

	agentConfig := &agent.AgentConfig{
		ModelConfig:   modelConfig,
		MCPConfig:     mcpConfig,
		SystemPrompt:  systemPrompt,
		MaxSteps:      agentMaxSteps,
		MessageWindow: finalMessageWindow,
	}

	// Create the agent
	mcpAgent, err := agent.NewAgent(ctx, agentConfig)
	if err != nil {
		return fmt.Errorf("failed to create agent: %v", err)
	}
	defer mcpAgent.Close()

	// Get model name for display
	parts := strings.SplitN(finalModel, ":", 2)
	modelName := "Unknown"
	if len(parts) == 2 {
		modelName = parts[1]
	}

	// Get tools
	tools := mcpAgent.GetTools()

	// Create CLI interface (skip if quiet mode)
	var cli *ui.CLI
	if !quietFlag {
		cli, err = ui.NewCLI()
		if err != nil {
			return fmt.Errorf("failed to create CLI: %v", err)
		}

		// Log successful initialization
		if len(parts) == 2 {
			cli.DisplayInfo(fmt.Sprintf("Model loaded: %s (%s)", parts[0], parts[1]))
		}
		cli.DisplayInfo(fmt.Sprintf("Loaded %d tools from MCP servers", len(tools)))
	}

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

	// Main interaction logic
	var messages []*schema.Message

	// Check if running in non-interactive mode
	if promptFlag != "" {
		return runNonInteractiveMode(ctx, mcpAgent, cli, promptFlag, modelName, messages, quietFlag)
	}

	// Quiet mode is not allowed in interactive mode
	if quietFlag {
		return fmt.Errorf("--quiet flag can only be used with --prompt/-p")
	}

	return runInteractiveMode(ctx, mcpAgent, cli, serverNames, toolNames, modelName, messages)
}

// runNonInteractiveMode handles the non-interactive mode execution
func runNonInteractiveMode(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, prompt, modelName string, messages []*schema.Message, quiet bool) error {
	// Display user message (skip if quiet)
	if !quiet && cli != nil {
		cli.DisplayUserMessage(prompt)
	}

	// Add user message to history
	messages = append(messages, schema.UserMessage(prompt))

	// Get agent response with controlled spinner that stops for tool call display
	var response *schema.Message
	var currentSpinner *ui.Spinner

	// Start initial spinner (skip if quiet)
	if !quiet && cli != nil {
		currentSpinner = ui.NewSpinner("Thinking...")
		currentSpinner.Start()
	}

	response, err := mcpAgent.GenerateWithLoop(ctx, messages,
		// Tool call handler - called when a tool is about to be executed
		func(toolName, toolArgs string) {
			if !quiet && cli != nil {
				// Stop spinner before displaying tool call
				if currentSpinner != nil {
					currentSpinner.Stop()
					currentSpinner = nil
				}
				cli.DisplayToolCallMessage(toolName, toolArgs)
			}
		},
		// Tool execution handler - called when tool execution starts/ends
		func(toolName string, isStarting bool) {
			if !quiet && cli != nil {
				if isStarting {
					// Start spinner for tool execution
					currentSpinner = ui.NewSpinner(fmt.Sprintf("Executing %s...", toolName))
					currentSpinner.Start()
				} else {
					// Stop spinner when tool execution completes
					if currentSpinner != nil {
						currentSpinner.Stop()
						currentSpinner = nil
					}
				}
			}
		},
		// Tool result handler - called when a tool execution completes
		func(toolName, toolArgs, result string, isError bool) {
			if !quiet && cli != nil {
				cli.DisplayToolMessage(toolName, toolArgs, result, isError)
				// Start spinner again for next LLM call
				currentSpinner = ui.NewSpinner("Thinking...")
				currentSpinner.Start()
			}
		},
		// Response handler - called when the LLM generates a response
		func(content string) {
			if !quiet && cli != nil {
				// Stop spinner when we get the final response
				if currentSpinner != nil {
					currentSpinner.Stop()
					currentSpinner = nil
				}
			}
		},

		// Tool call content handler - called when content accompanies tool calls
		func(content string) {
			if !quiet && cli != nil {
				// Stop spinner before displaying content
				if currentSpinner != nil {
					currentSpinner.Stop()
					currentSpinner = nil
				}
				cli.DisplayAssistantMessageWithModel(content, modelName)
				// Start spinner again for tool calls
				currentSpinner = ui.NewSpinner("Thinking...")
				currentSpinner.Start()
			}
		},
	)

	// Make sure spinner is stopped if still running
	if !quiet && cli != nil && currentSpinner != nil {
		currentSpinner.Stop()
	}
	if err != nil {
		if !quiet && cli != nil {
			cli.DisplayError(fmt.Errorf("agent error: %v", err))
		}
		return err
	}

	// Display assistant response with model name (skip if quiet)
	if !quiet && cli != nil {
		if err := cli.DisplayAssistantMessageWithModel(response.Content, modelName); err != nil {
			cli.DisplayError(fmt.Errorf("display error: %v", err))
			return err
		}
	} else if quiet {
		// In quiet mode, only output the final response content to stdout
		fmt.Print(response.Content)
	}

	// Exit after displaying the final response
	return nil
}

// runInteractiveMode handles the interactive mode execution
func runInteractiveMode(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, serverNames, toolNames []string, modelName string, messages []*schema.Message) error {

	// Main interaction loop
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

		// Get agent response with controlled spinner that stops for tool call display
		var response *schema.Message
		var currentSpinner *ui.Spinner

		// Start initial spinner
		currentSpinner = ui.NewSpinner("Thinking...")
		currentSpinner.Start()

		response, err = mcpAgent.GenerateWithLoop(ctx, messages,
			// Tool call handler - called when a tool is about to be executed
			func(toolName, toolArgs string) {
				// Stop spinner before displaying tool call
				if currentSpinner != nil {
					currentSpinner.Stop()
					currentSpinner = nil
				}
				cli.DisplayToolCallMessage(toolName, toolArgs)
			},
			// Tool execution handler - called when tool execution starts/ends
			func(toolName string, isStarting bool) {
				if isStarting {
					// Start spinner for tool execution
					currentSpinner = ui.NewSpinner(fmt.Sprintf("Executing %s...", toolName))
					currentSpinner.Start()
				} else {
					// Stop spinner when tool execution completes
					if currentSpinner != nil {
						currentSpinner.Stop()
						currentSpinner = nil
					}
				}
			},
			// Tool result handler - called when a tool execution completes
			func(toolName, toolArgs, result string, isError bool) {
				cli.DisplayToolMessage(toolName, toolArgs, result, isError)
				// Start spinner again for next LLM call
				currentSpinner = ui.NewSpinner("Thinking...")
				currentSpinner.Start()
			},
			// Response handler - called when the LLM generates a response
			func(content string) {
				// Stop spinner when we get the final response
				if currentSpinner != nil {
					currentSpinner.Stop()
					currentSpinner = nil
				}
			},
			// Tool call content handler - called when content accompanies tool calls
			func(content string) {
				// Stop spinner before displaying content
				if currentSpinner != nil {
					currentSpinner.Stop()
					currentSpinner = nil
				}
				cli.DisplayAssistantMessageWithModel(content, modelName)
				// Start spinner again for tool calls
				currentSpinner = ui.NewSpinner("Thinking...")
				currentSpinner.Start()
			},
		)

		// Make sure spinner is stopped if still running
		if currentSpinner != nil {
			currentSpinner.Stop()
		}
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


