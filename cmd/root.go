package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcphost/internal/agent"
	"github.com/mark3labs/mcphost/internal/auth"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/session"
	"github.com/mark3labs/mcphost/internal/tokens"
	"github.com/mark3labs/mcphost/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile       string
	systemPromptFile string
	modelFlag        string
	providerURL      string
	providerAPIKey   string
	debugMode        bool
	promptFlag       string
	quietFlag        bool
	noExitFlag       bool
	maxSteps         int
	scriptMCPConfig  *config.Config // Used to override config in script mode

	// Session management
	saveSessionPath string
	loadSessionPath string

	// Model generation parameters
	maxTokens     int
	temperature   float32
	topP          float32
	topK          int32
	stopSequences []string

	// Ollama-specific parameters
	numGPU  int32
	mainGPU int32
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
  
  # Session management
  mcphost --save-session ./my-session.json -p "Hello"
  mcphost --load-session ./my-session.json -p "Continue our conversation"
  mcphost --load-session ./session.json --save-session ./session.json -p "Next message"
  
  # Script mode
  mcphost script myscript.sh`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPHost(context.Background())
	},
}

func Execute(v string) {
	rootCmd.Version = v
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func initConfig() {
	if configFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(configFile)

		// Try to read the specified config file
		if err := viper.ReadInConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading config file '%s': %v\n", configFile, err)
			os.Exit(1)
		}
	} else {
		// Ensure a config file exists (create default if none found)
		if err := config.EnsureConfigExists(); err != nil {
			// If we can't create config, continue silently (non-fatal)
			fmt.Fprintf(os.Stderr, "Warning: Could not create default config file: %v\n", err)
		}

		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding home directory: %v\n", err)
			os.Exit(1)
		}

		// Search config in home directory with name ".mcphost" (without extension)
		viper.AddConfigPath(home)
		viper.SetConfigName(".mcphost")
		viper.SetConfigType("yaml")

		// Also try JSON format
		if err := viper.ReadInConfig(); err != nil {
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

	// Set environment variable prefix
	viper.SetEnvPrefix("MCPHOST")
	viper.AutomaticEnv()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file (default is $HOME/.mcp.json)")
	rootCmd.PersistentFlags().
		StringVar(&systemPromptFile, "system-prompt", "", "system prompt text or path to text file")

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
		BoolVar(&noExitFlag, "no-exit", false, "prevent non-interactive mode from exiting, show input prompt instead")
	rootCmd.PersistentFlags().
		IntVar(&maxSteps, "max-steps", 0, "maximum number of agent steps (0 for unlimited)")

	// Session management flags
	rootCmd.PersistentFlags().
		StringVar(&saveSessionPath, "save-session", "", "save session to file after each message")
	rootCmd.PersistentFlags().
		StringVar(&loadSessionPath, "load-session", "", "load session from file at startup")

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&providerURL, "provider-url", "", "base URL for the provider API (applies to OpenAI, Anthropic, Ollama, and Google)")
	flags.StringVar(&providerAPIKey, "provider-api-key", "", "API key for the provider (applies to OpenAI, Anthropic, and Google)")

	// Model generation parameters
	flags.IntVar(&maxTokens, "max-tokens", 4096, "maximum number of tokens in the response")
	flags.Float32Var(&temperature, "temperature", 0.7, "controls randomness in responses (0.0-1.0)")
	flags.Float32Var(&topP, "top-p", 0.95, "controls diversity via nucleus sampling (0.0-1.0)")
	flags.Int32Var(&topK, "top-k", 40, "controls diversity by limiting top K tokens to sample from")
	flags.StringSliceVar(&stopSequences, "stop-sequences", nil, "custom stop sequences (comma-separated)")

	// Ollama-specific parameters
	flags.Int32Var(&numGPU, "num-gpu-layers", -1, "number of model layers to offload to GPU for Ollama models (-1 for auto-detect)")
	flags.MarkHidden("num-gpu-layers") // Advanced option, hidden from help
	flags.Int32Var(&mainGPU, "main-gpu", 0, "main GPU device to use for Ollama models")

	// Bind flags to viper for config file support
	viper.BindPFlag("system-prompt", rootCmd.PersistentFlags().Lookup("system-prompt"))
	viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("prompt", rootCmd.PersistentFlags().Lookup("prompt"))
	viper.BindPFlag("max-steps", rootCmd.PersistentFlags().Lookup("max-steps"))
	viper.BindPFlag("provider-url", rootCmd.PersistentFlags().Lookup("provider-url"))
	viper.BindPFlag("provider-api-key", rootCmd.PersistentFlags().Lookup("provider-api-key"))
	viper.BindPFlag("max-tokens", rootCmd.PersistentFlags().Lookup("max-tokens"))
	viper.BindPFlag("temperature", rootCmd.PersistentFlags().Lookup("temperature"))
	viper.BindPFlag("top-p", rootCmd.PersistentFlags().Lookup("top-p"))
	viper.BindPFlag("top-k", rootCmd.PersistentFlags().Lookup("top-k"))
	viper.BindPFlag("stop-sequences", rootCmd.PersistentFlags().Lookup("stop-sequences"))
	viper.BindPFlag("num-gpu-layers", rootCmd.PersistentFlags().Lookup("num-gpu-layers"))
	viper.BindPFlag("main-gpu", rootCmd.PersistentFlags().Lookup("main-gpu"))

	// Defaults are already set in flag definitions, no need to duplicate in viper

	// Add subcommands
	rootCmd.AddCommand(authCmd)
}

func runMCPHost(ctx context.Context) error {
	return runNormalMode(ctx)
}

func runNormalMode(ctx context.Context) error {
	// Initialize token counters
	tokens.InitializeTokenCounters()

	// Validate flag combinations
	if quietFlag && promptFlag == "" {
		return fmt.Errorf("--quiet flag can only be used with --prompt/-p")
	}
	if noExitFlag && promptFlag == "" {
		return fmt.Errorf("--no-exit flag can only be used with --prompt/-p")
	}

	// Set up logging
	if debugMode {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// Load MCP configuration
	var mcpConfig *config.Config
	var err error

	if scriptMCPConfig != nil {
		// Use script-provided config
		mcpConfig = scriptMCPConfig
	} else {
		// Get MCP config from the global viper instance (already loaded by initConfig)
		mcpConfig = &config.Config{
			MCPServers: make(map[string]config.MCPServerConfig),
		}
		if err := viper.Unmarshal(mcpConfig); err != nil {
			return fmt.Errorf("failed to unmarshal MCP config: %v", err)
		}

		// Validate the config
		if err := mcpConfig.Validate(); err != nil {
			return fmt.Errorf("invalid MCP config: %v", err)
		}
	}

	// Update debug mode from viper
	if viper.GetBool("debug") && !debugMode {
		debugMode = viper.GetBool("debug")
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	systemPrompt, err := config.LoadSystemPrompt(viper.GetString("system-prompt"))
	if err != nil {
		return fmt.Errorf("failed to load system prompt: %v", err)
	}

	// Create model configuration
	temperature := float32(viper.GetFloat64("temperature"))
	topP := float32(viper.GetFloat64("top-p"))
	topK := int32(viper.GetInt("top-k"))
	numGPU := int32(viper.GetInt("num-gpu-layers"))
	mainGPU := int32(viper.GetInt("main-gpu"))

	modelConfig := &models.ProviderConfig{
		ModelString:    viper.GetString("model"),
		SystemPrompt:   systemPrompt,
		ProviderAPIKey: viper.GetString("provider-api-key"),
		ProviderURL:    viper.GetString("provider-url"),
		MaxTokens:      viper.GetInt("max-tokens"),
		Temperature:    &temperature,
		TopP:           &topP,
		TopK:           &topK,
		StopSequences:  viper.GetStringSlice("stop-sequences"),
		NumGPU:         &numGPU,
		MainGPU:        &mainGPU,
	}

	// Create agent configuration
	agentConfig := &agent.AgentConfig{
		ModelConfig:  modelConfig,
		MCPConfig:    mcpConfig,
		SystemPrompt: systemPrompt,
		MaxSteps:     viper.GetInt("max-steps"), // Pass 0 for infinite, agent will handle it
	}

	// Create the agent with spinner for Ollama models
	var mcpAgent *agent.Agent

	if strings.HasPrefix(viper.GetString("model"), "ollama:") && !quietFlag {
		// Create a temporary CLI for the spinner
		tempCli, tempErr := ui.NewCLI(viper.GetBool("debug"))
		if tempErr == nil {
			err = tempCli.ShowSpinner("Loading Ollama model...", func() error {
				var agentErr error
				mcpAgent, agentErr = agent.NewAgent(ctx, agentConfig)
				return agentErr
			})
		} else {
			// Fallback without spinner
			mcpAgent, err = agent.NewAgent(ctx, agentConfig)
		}
	} else {
		// No spinner for other providers
		mcpAgent, err = agent.NewAgent(ctx, agentConfig)
	}

	if err != nil {
		return fmt.Errorf("failed to create agent: %v", err)
	}
	defer mcpAgent.Close()

	// Get model name for display
	modelString := viper.GetString("model")
	parts := strings.SplitN(modelString, ":", 2)
	modelName := "Unknown"
	if len(parts) == 2 {
		modelName = parts[1]
	}

	// Get tools
	tools := mcpAgent.GetTools()

	// Create CLI interface (skip if quiet mode)
	var cli *ui.CLI
	if !quietFlag {
		cli, err = ui.NewCLI(viper.GetBool("debug"))
		if err != nil {
			return fmt.Errorf("failed to create CLI: %v", err)
		}

		// Set up usage tracking for supported providers
		if len(parts) == 2 {
			provider := parts[0]
			modelID := parts[1]

			// Skip usage tracking for ollama as it's not in models.dev
			if provider != "ollama" {
				registry := models.GetGlobalRegistry()
				if modelInfo, err := registry.ValidateModel(provider, modelID); err == nil {
					// Check if OAuth credentials are being used for Anthropic models
					isOAuth := false
					if provider == "anthropic" {
						_, source, err := auth.GetAnthropicAPIKey(viper.GetString("provider-api-key"))
						if err == nil && strings.HasPrefix(source, "stored OAuth") {
							isOAuth = true
						}
					}

					usageTracker := ui.NewUsageTracker(modelInfo, provider, 80, isOAuth) // Will be updated with actual width
					cli.SetUsageTracker(usageTracker)
				}
			}
		}

		// Log successful initialization
		if len(parts) == 2 {
			cli.DisplayInfo(fmt.Sprintf("Model loaded: %s (%s)", parts[0], parts[1]))
		}

		// Display loading message if available (e.g., GPU fallback info)
		if loadingMessage := mcpAgent.GetLoadingMessage(); loadingMessage != "" {
			cli.DisplayInfo(loadingMessage)
		}

		cli.DisplayInfo(fmt.Sprintf("Loaded %d tools from MCP servers", len(tools)))
		// Display debug configuration if debug mode is enabled
		if viper.GetBool("debug") {
			debugConfig := map[string]any{
				"model":         viper.GetString("model"),
				"max-steps":     viper.GetInt("max-steps"),
				"max-tokens":    viper.GetInt("max-tokens"),
				"temperature":   viper.GetFloat64("temperature"),
				"top-p":         viper.GetFloat64("top-p"),
				"top-k":         viper.GetInt("top-k"),
				"provider-url":  viper.GetString("provider-url"),
				"system-prompt": viper.GetString("system-prompt"),
			}

			// Add Ollama-specific parameters if using Ollama
			if strings.HasPrefix(viper.GetString("model"), "ollama:") {
				debugConfig["num-gpu-layers"] = viper.GetInt("num-gpu-layers")
				debugConfig["main-gpu"] = viper.GetInt("main-gpu")
			}

			// Only include non-empty stop sequences
			stopSequences := viper.GetStringSlice("stop-sequences")
			if len(stopSequences) > 0 {
				debugConfig["stop-sequences"] = stopSequences
			}

			// Only include API keys if they're set (but don't show the actual values for security)
			if viper.GetString("provider-api-key") != "" {
				debugConfig["provider-api-key"] = "[SET]"
			}

			cli.DisplayDebugConfig(debugConfig)
		}
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
	var sessionManager *session.Manager

	// Load existing session if specified
	if loadSessionPath != "" {
		loadedSession, err := session.LoadFromFile(loadSessionPath)
		if err != nil {
			return fmt.Errorf("failed to load session: %v", err)
		}

		// Convert session messages to schema messages
		for _, msg := range loadedSession.Messages {
			messages = append(messages, msg.ConvertToSchemaMessage())
		}

		// If we're also saving, use the loaded session with the session manager
		if saveSessionPath != "" {
			sessionManager = session.NewManagerWithSession(loadedSession, saveSessionPath)
		}

		if !quietFlag && cli != nil {
			// Create a map of tool call IDs to tool calls for quick lookup
			toolCallMap := make(map[string]session.ToolCall)
			for _, sessionMsg := range loadedSession.Messages {
				if sessionMsg.Role == "assistant" && len(sessionMsg.ToolCalls) > 0 {
					for _, tc := range sessionMsg.ToolCalls {
						toolCallMap[tc.ID] = tc
					}
				}
			}

			// Display all previous messages as they would have appeared
			for _, sessionMsg := range loadedSession.Messages {
				if sessionMsg.Role == "user" {
					cli.DisplayUserMessage(sessionMsg.Content)
				} else if sessionMsg.Role == "assistant" {
					// Display tool calls if present
					if len(sessionMsg.ToolCalls) > 0 {
						for _, tc := range sessionMsg.ToolCalls {
							// Convert arguments to string
							var argsStr string
							if argBytes, err := json.Marshal(tc.Arguments); err == nil {
								argsStr = string(argBytes)
							}

							// Display tool call
							cli.DisplayToolCallMessage(tc.Name, argsStr)
						}
					}

					// Display assistant response (only if there's content)
					if sessionMsg.Content != "" {
						cli.DisplayAssistantMessage(sessionMsg.Content)
					}
				} else if sessionMsg.Role == "tool" {
					// Display tool result
					if sessionMsg.ToolCallID != "" {
						if toolCall, exists := toolCallMap[sessionMsg.ToolCallID]; exists {
							// Convert arguments to string
							var argsStr string
							if argBytes, err := json.Marshal(toolCall.Arguments); err == nil {
								argsStr = string(argBytes)
							}

							// Parse tool result content - it might be JSON-encoded MCP content
							resultContent := sessionMsg.Content

							// Try to parse as MCP content structure
							var mcpContent struct {
								Content []struct {
									Type string `json:"type"`
									Text string `json:"text"`
								} `json:"content"`
							}

							// First try to unmarshal as-is
							if err := json.Unmarshal([]byte(sessionMsg.Content), &mcpContent); err == nil {
								// Extract text from MCP content structure
								if len(mcpContent.Content) > 0 && mcpContent.Content[0].Type == "text" {
									resultContent = mcpContent.Content[0].Text
								}
							} else {
								// If that fails, try unquoting first (in case it's double-encoded)
								var unquoted string
								if err := json.Unmarshal([]byte(sessionMsg.Content), &unquoted); err == nil {
									if err := json.Unmarshal([]byte(unquoted), &mcpContent); err == nil {
										if len(mcpContent.Content) > 0 && mcpContent.Content[0].Type == "text" {
											resultContent = mcpContent.Content[0].Text
										}
									}
								}
							}

							// Display tool result (assuming no error for saved results)
							cli.DisplayToolMessage(toolCall.Name, argsStr, resultContent, false)
						}
					}
				}
			}
		}
	} else if saveSessionPath != "" {
		// Only saving, create new session manager
		sessionManager = session.NewManager(saveSessionPath)

		// Set metadata
		sessionManager.SetMetadata(session.Metadata{
			MCPHostVersion: "dev", // TODO: Get actual version
			Provider:       parts[0],
			Model:          modelName,
		})
	}

	// Check if running in non-interactive mode
	if promptFlag != "" {
		return runNonInteractiveMode(ctx, mcpAgent, cli, promptFlag, modelName, messages, quietFlag, noExitFlag, mcpConfig, sessionManager)
	}

	// Quiet mode is not allowed in interactive mode
	if quietFlag {
		return fmt.Errorf("--quiet flag can only be used with --prompt/-p")
	}

	return runInteractiveMode(ctx, mcpAgent, cli, serverNames, toolNames, modelName, messages, sessionManager)
}

// AgenticLoopConfig configures the behavior of the unified agentic loop
type AgenticLoopConfig struct {
	// Mode configuration
	IsInteractive    bool   // true for interactive mode, false for non-interactive
	InitialPrompt    string // initial prompt for non-interactive mode
	ContinueAfterRun bool   // true to continue to interactive mode after initial run (--no-exit)

	// UI configuration
	Quiet bool // suppress all output except final response

	// Context data
	ServerNames    []string         // for slash commands
	ToolNames      []string         // for slash commands
	ModelName      string           // for display
	MCPConfig      *config.Config   // for continuing to interactive mode
	SessionManager *session.Manager // for session persistence
}

// runAgenticLoop handles all execution modes with a single unified loop
func runAgenticLoop(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, messages []*schema.Message, config AgenticLoopConfig) error {
	// Handle initial prompt for non-interactive modes
	if !config.IsInteractive && config.InitialPrompt != "" {
		// Display user message (skip if quiet)
		if !config.Quiet && cli != nil {
			cli.DisplayUserMessage(config.InitialPrompt)
		}

		// Create temporary messages with user input for processing (don't add to history yet)
		tempMessages := append(messages, schema.UserMessage(config.InitialPrompt))

		// Process the initial prompt with tool calls
		response, conversationMessages, err := runAgenticStep(ctx, mcpAgent, cli, tempMessages, config)
		if err != nil {
			// Check if this was a user cancellation
			if err.Error() == "generation cancelled by user" && cli != nil {
				cli.DisplayCancellation()
				// On cancellation, continue to interactive mode (like --no-exit)
				// Don't add the cancelled message to history
				config.IsInteractive = true
			} else {
				return err
			}
		} else {
			// Only add to history after successful completion
			userMsg := schema.UserMessage(config.InitialPrompt)
			messages = append(messages, userMsg)
			messages = append(messages, response)

			// Save to session if session manager is available
			if config.SessionManager != nil {
				// Simple approach: save the entire conversation history
				// This includes the user message + all generated messages
				allMessages := append([]*schema.Message{userMsg}, conversationMessages...)

				// Clear the session and save the complete history
				if err := config.SessionManager.ReplaceAllMessages(allMessages); err != nil {
					// Log error but don't fail the operation
					if cli != nil && !config.Quiet {
						cli.DisplayError(fmt.Errorf("failed to save conversation to session: %v", err))
					}
				}
			}

			// If not continuing to interactive mode, exit here
			if !config.ContinueAfterRun {
				return nil
			}

			// Update config for interactive mode continuation
			config.IsInteractive = true
		}
	}

	// Interactive loop (or continuation after non-interactive)
	if config.IsInteractive {
		return runInteractiveLoop(ctx, mcpAgent, cli, messages, config)
	}

	return nil
}

// runAgenticStep processes a single step of the agentic loop (handles tool calls)
func runAgenticStep(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, messages []*schema.Message, config AgenticLoopConfig) (*schema.Message, []*schema.Message, error) {
	var currentSpinner *ui.Spinner

	// Start initial spinner (skip if quiet)
	if !config.Quiet && cli != nil {
		currentSpinner = ui.NewSpinner("Thinking...")
		currentSpinner.Start()
	}

	result, err := mcpAgent.GenerateWithLoop(ctx, messages,
		// Tool call handler - called when a tool is about to be executed
		func(toolName, toolArgs string) {
			if !config.Quiet && cli != nil {
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
			if !config.Quiet && cli != nil {
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
			if !config.Quiet && cli != nil {
				// Parse tool result content - it might be JSON-encoded MCP content
				resultContent := result

				// Try to parse as MCP content structure
				var mcpContent struct {
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				}

				// First try to unmarshal as-is
				if err := json.Unmarshal([]byte(result), &mcpContent); err == nil {
					// Extract text from MCP content structure
					if len(mcpContent.Content) > 0 && mcpContent.Content[0].Type == "text" {
						resultContent = mcpContent.Content[0].Text
					}
				} else {
					// If that fails, try unquoting first (in case it's double-encoded)
					var unquoted string
					if err := json.Unmarshal([]byte(result), &unquoted); err == nil {
						if err := json.Unmarshal([]byte(unquoted), &mcpContent); err == nil {
							if len(mcpContent.Content) > 0 && mcpContent.Content[0].Type == "text" {
								resultContent = mcpContent.Content[0].Text
							}
						}
					}
				}

				cli.DisplayToolMessage(toolName, toolArgs, resultContent, isError)
				// Start spinner again for next LLM call
				currentSpinner = ui.NewSpinner("Thinking...")
				currentSpinner.Start()
			}
		},
		// Response handler - called when the LLM generates a response
		func(content string) {
			if !config.Quiet && cli != nil {
				// Stop spinner when we get the final response
				if currentSpinner != nil {
					currentSpinner.Stop()
					currentSpinner = nil
				}
			}
		},
		// Tool call content handler - called when content accompanies tool calls
		func(content string) {
			if !config.Quiet && cli != nil {
				// Stop spinner before displaying content
				if currentSpinner != nil {
					currentSpinner.Stop()
					currentSpinner = nil
				}
				cli.DisplayAssistantMessageWithModel(content, config.ModelName)
				// Start spinner again for tool calls
				currentSpinner = ui.NewSpinner("Thinking...")
				currentSpinner.Start()
			}
		},
	)

	// Make sure spinner is stopped if still running
	if !config.Quiet && cli != nil && currentSpinner != nil {
		currentSpinner.Stop()
	}

	if err != nil {
		if !config.Quiet && cli != nil {
			cli.DisplayError(fmt.Errorf("agent error: %v", err))
		}
		return nil, nil, err
	}

	// Get the final response and conversation messages
	response := result.FinalResponse
	conversationMessages := result.ConversationMessages

	// Display assistant response with model name (skip if quiet)
	if !config.Quiet && cli != nil {
		if err := cli.DisplayAssistantMessageWithModel(response.Content, config.ModelName); err != nil {
			cli.DisplayError(fmt.Errorf("display error: %v", err))
			return nil, nil, err
		}

		// Update usage tracking with the last user message and response
		if len(messages) > 0 {
			lastUserMessage := ""
			// Find the last user message
			for i := len(messages) - 1; i >= 0; i-- {
				if messages[i].Role == schema.User {
					lastUserMessage = messages[i].Content
					break
				}
			}
			cli.UpdateUsageFromResponse(response, lastUserMessage)
		}
	} else if config.Quiet {
		// In quiet mode, only output the final response content to stdout
		fmt.Print(response.Content)
	}

	// Return the final response and all conversation messages
	return response, conversationMessages, nil
}

// runInteractiveLoop handles the interactive portion of the agentic loop
func runInteractiveLoop(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, messages []*schema.Message, config AgenticLoopConfig) error {
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
			result := cli.HandleSlashCommand(prompt, config.ServerNames, config.ToolNames, messages)
			if result.Handled {
				// If the command was to clear history, clear the messages slice and session
				if result.ClearHistory {
					messages = messages[:0] // Clear the slice
					// Also clear session if available
					if config.SessionManager != nil {
						if err := config.SessionManager.ReplaceAllMessages([]*schema.Message{}); err != nil {
							cli.DisplayError(fmt.Errorf("failed to clear session: %v", err))
						}
					}
				}
				continue
			}
			cli.DisplayError(fmt.Errorf("unknown command: %s", prompt))
			continue
		}

		// Display user message
		cli.DisplayUserMessage(prompt)

		// Create temporary messages with user input for processing
		tempMessages := append(messages, schema.UserMessage(prompt))

		// Process the user input with tool calls
		response, conversationMessages, err := runAgenticStep(ctx, mcpAgent, cli, tempMessages, config)
		if err != nil {
			// Check if this was a user cancellation
			if err.Error() == "generation cancelled by user" {
				cli.DisplayCancellation()
			} else {
				cli.DisplayError(fmt.Errorf("agent error: %v", err))
			}
			continue
		}

		// Only add to history after successful completion
		userMsg := schema.UserMessage(prompt)
		messages = append(messages, userMsg)
		messages = append(messages, response)

		// Save to session if session manager is available
		if config.SessionManager != nil {
			if err := config.SessionManager.AddMessage(userMsg); err != nil {
				// Log error but don't fail the operation
				cli.DisplayError(fmt.Errorf("failed to save user message to session: %v", err))
			}

			// Save all conversation messages (includes tool calls and results)
			// Find the messages that were generated during this conversation
			if len(conversationMessages) > len(tempMessages) {
				// Extract only the new messages generated during this step
				newMessages := conversationMessages[len(tempMessages):]
				if err := config.SessionManager.AddMessages(newMessages); err != nil {
					// Log error but don't fail the operation
					cli.DisplayError(fmt.Errorf("failed to save conversation messages to session: %v", err))
				}
			} else {
				// No tool calls, just save the final response
				if err := config.SessionManager.AddMessage(response); err != nil {
					// Log error but don't fail the operation
					cli.DisplayError(fmt.Errorf("failed to save assistant message to session: %v", err))
				}
			}
		}
	}
}

// runNonInteractiveMode handles the non-interactive mode execution
func runNonInteractiveMode(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, prompt, modelName string, messages []*schema.Message, quiet, noExit bool, mcpConfig *config.Config, sessionManager *session.Manager) error {
	// Prepare data for slash commands (needed if continuing to interactive mode)
	var serverNames []string
	for name := range mcpConfig.MCPServers {
		serverNames = append(serverNames, name)
	}

	tools := mcpAgent.GetTools()
	var toolNames []string
	for _, tool := range tools {
		if info, err := tool.Info(ctx); err == nil {
			toolNames = append(toolNames, info.Name)
		}
	}

	// Configure and run unified agentic loop
	config := AgenticLoopConfig{
		IsInteractive:    false,
		InitialPrompt:    prompt,
		ContinueAfterRun: noExit,
		Quiet:            quiet,
		ServerNames:      serverNames,
		ToolNames:        toolNames,
		ModelName:        modelName,
		MCPConfig:        mcpConfig,
		SessionManager:   sessionManager,
	}

	return runAgenticLoop(ctx, mcpAgent, cli, messages, config)
}

// runInteractiveMode handles the interactive mode execution
func runInteractiveMode(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, serverNames, toolNames []string, modelName string, messages []*schema.Message, sessionManager *session.Manager) error {
	// Configure and run unified agentic loop
	config := AgenticLoopConfig{
		IsInteractive:    true,
		InitialPrompt:    "",
		ContinueAfterRun: false,
		Quiet:            false,
		ServerNames:      serverNames,
		ToolNames:        toolNames,
		ModelName:        modelName,
		MCPConfig:        nil, // Not needed for pure interactive mode
		SessionManager:   sessionManager,
	}

	return runAgenticLoop(ctx, mcpAgent, cli, messages, config)
}
