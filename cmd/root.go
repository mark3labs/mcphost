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
	"github.com/mark3labs/mcphost/internal/auth"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
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
	flags.Int32Var(&numGPU, "num-gpu", 1, "number of GPUs to use for Ollama models")
	flags.Int32Var(&mainGPU, "main-gpu", 0, "main GPU to use for Ollama models")

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
	viper.BindPFlag("num-gpu", rootCmd.PersistentFlags().Lookup("num-gpu"))
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
	numGPU := int32(viper.GetInt("num-gpu"))
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

	// Create the agent
	mcpAgent, err := agent.NewAgent(ctx, agentConfig)
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
				debugConfig["num-gpu"] = viper.GetInt("num-gpu")
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

	// Check if running in non-interactive mode
	if promptFlag != "" {
		return runNonInteractiveMode(ctx, mcpAgent, cli, promptFlag, modelName, messages, quietFlag, noExitFlag, mcpConfig)
	}

	// Quiet mode is not allowed in interactive mode
	if quietFlag {
		return fmt.Errorf("--quiet flag can only be used with --prompt/-p")
	}

	return runInteractiveMode(ctx, mcpAgent, cli, serverNames, toolNames, modelName, messages)
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
	ServerNames []string       // for slash commands
	ToolNames   []string       // for slash commands
	ModelName   string         // for display
	MCPConfig   *config.Config // for continuing to interactive mode
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
		response, err := runAgenticStep(ctx, mcpAgent, cli, tempMessages, config)
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
			messages = append(messages, schema.UserMessage(config.InitialPrompt))
			messages = append(messages, response)

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
func runAgenticStep(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, messages []*schema.Message, config AgenticLoopConfig) (*schema.Message, error) {
	var currentSpinner *ui.Spinner

	// Start initial spinner (skip if quiet)
	if !config.Quiet && cli != nil {
		currentSpinner = ui.NewSpinner("Thinking...")
		currentSpinner.Start()
	}

	response, err := mcpAgent.GenerateWithLoop(ctx, messages,
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
				cli.DisplayToolMessage(toolName, toolArgs, result, isError)
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
		return nil, err
	}

	// Display assistant response with model name (skip if quiet)
	if !config.Quiet && cli != nil {
		if err := cli.DisplayAssistantMessageWithModel(response.Content, config.ModelName); err != nil {
			cli.DisplayError(fmt.Errorf("display error: %v", err))
			return nil, err
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

	return response, nil
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
			if cli.HandleSlashCommand(prompt, config.ServerNames, config.ToolNames, messages) {
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
		response, err := runAgenticStep(ctx, mcpAgent, cli, tempMessages, config)
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
		messages = append(messages, schema.UserMessage(prompt))
		messages = append(messages, response)
	}
}

// runNonInteractiveMode handles the non-interactive mode execution
func runNonInteractiveMode(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, prompt, modelName string, messages []*schema.Message, quiet, noExit bool, mcpConfig *config.Config) error {
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
	}

	return runAgenticLoop(ctx, mcpAgent, cli, messages, config)
}

// runInteractiveMode handles the interactive mode execution
func runInteractiveMode(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, serverNames, toolNames []string, modelName string, messages []*schema.Message) error {
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
	}

	return runAgenticLoop(ctx, mcpAgent, cli, messages, config)
}
