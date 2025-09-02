package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcphost/internal/agent"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/hooks"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/session"
	"github.com/mark3labs/mcphost/internal/tokens"
	"github.com/mark3labs/mcphost/internal/tools"
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
	streamFlag       bool           // Enable streaming output
	compactMode      bool           // Enable compact output mode
	scriptMCPConfig  *config.Config // Used to override config in script mode

	// Session management
	saveSessionPath string
	loadSessionPath string
	sessionPath     string

	// Model generation parameters
	maxTokens     int
	temperature   float32
	topP          float32
	topK          int32
	stopSequences []string

	// Ollama-specific parameters
	numGPU  int32
	mainGPU int32

	// Hooks control
	noHooks bool

	// TLS configuration
	tlsSkipVerify bool
)

// agentUIAdapter adapts agent.Agent to ui.AgentInterface
type agentUIAdapter struct {
	agent *agent.Agent
}

func (a *agentUIAdapter) GetLoadingMessage() string {
	return a.agent.GetLoadingMessage()
}

func (a *agentUIAdapter) GetTools() []any {
	tools := a.agent.GetTools()
	result := make([]any, len(tools))
	for i, tool := range tools {
		result[i] = tool
	}
	return result
}

func (a *agentUIAdapter) GetLoadedServerNames() []string {
	return a.agent.GetLoadedServerNames()
}

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
  mcphost --session ./session.json -p "Next message"
  
  # Script mode
  mcphost script myscript.sh`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPHost(context.Background())
	},
}

// GetRootCommand returns the root command with the version set
func GetRootCommand(v string) *cobra.Command {
	rootCmd.Version = v
	return rootCmd
}

func InitConfig() {
	if configFile != "" {
		// Use config file from the flag
		if err := LoadConfigWithEnvSubstitution(configFile); err != nil {
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

		// Set up viper config search paths and names
		// Current directory has higher priority than home directory
		viper.AddConfigPath(".")  // Current directory (searched first)
		viper.AddConfigPath(home) // Home directory (searched second)

		// Try to find and load config file using viper's search mechanism
		configLoaded := false
		configNames := []string{".mcphost", ".mcp"} // Try .mcphost first, then legacy .mcp

		for _, name := range configNames {
			viper.SetConfigName(name)

			// Try to read the config file
			if err := viper.ReadInConfig(); err == nil {
				// Config file found, now reload it with env substitution
				configPath := viper.ConfigFileUsed()
				if err := LoadConfigWithEnvSubstitution(configPath); err != nil {
					// Only exit on environment variable substitution errors
					if strings.Contains(err.Error(), "environment variable substitution failed") {
						fmt.Fprintf(os.Stderr, "Error reading config file '%s': %v\n", configPath, err)
						os.Exit(1)
					}
					// For other errors, continue trying other config files
					continue
				}
				configLoaded = true
				break
			}
		}

		// If no config file was loaded, continue without error (optional config)
		if !configLoaded && debugMode {
			fmt.Fprintf(os.Stderr, "No config file found in current directory or home directory\n")
		}
	}

	// Set environment variable prefix
	viper.SetEnvPrefix("MCPHOST")
	viper.AutomaticEnv()

	// Load hooks configuration unless disabled
	if !viper.GetBool("no-hooks") {
		hooksConfig, err := hooks.LoadHooksConfig()
		if err != nil {
			// Hooks are optional, so just log a warning
			if debugMode {
				fmt.Fprintf(os.Stderr, "Warning: Failed to load hooks configuration: %v\n", err)
			}
		} else {
			viper.Set("hooks", hooksConfig)
		}
	}

}

// LoadConfigWithEnvSubstitution loads a config file with environment variable substitution
func LoadConfigWithEnvSubstitution(configPath string) error {
	// Read raw config file content
	rawContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// Apply environment variable substitution
	substituter := &config.EnvSubstituter{}
	processedContent, err := substituter.SubstituteEnvVars(string(rawContent))
	if err != nil {
		return fmt.Errorf("config env substitution failed: %v", err)
	}

	// Determine config type from file extension
	configType := "yaml"
	if strings.HasSuffix(configPath, ".json") {
		configType = "json"
	}

	config.SetConfigPath(configPath)

	// Use viper to parse the processed content
	viper.SetConfigType(configType)
	return viper.ReadConfig(strings.NewReader(processedContent))
}

func configToUiTheme(theme config.Theme) ui.Theme {
	return ui.Theme{
		Primary:     lipgloss.AdaptiveColor(theme.Primary),
		Secondary:   lipgloss.AdaptiveColor(theme.Secondary),
		Success:     lipgloss.AdaptiveColor(theme.Success),
		Warning:     lipgloss.AdaptiveColor(theme.Warning),
		Error:       lipgloss.AdaptiveColor(theme.Error),
		Info:        lipgloss.AdaptiveColor(theme.Info),
		Text:        lipgloss.AdaptiveColor(theme.Text),
		Muted:       lipgloss.AdaptiveColor(theme.Muted),
		VeryMuted:   lipgloss.AdaptiveColor(theme.VeryMuted),
		Background:  lipgloss.AdaptiveColor(theme.Background),
		Border:      lipgloss.AdaptiveColor(theme.Border),
		MutedBorder: lipgloss.AdaptiveColor(theme.MutedBorder),
		System:      lipgloss.AdaptiveColor(theme.System),
		Tool:        lipgloss.AdaptiveColor(theme.Tool),
		Accent:      lipgloss.AdaptiveColor(theme.Accent),
		Highlight:   lipgloss.AdaptiveColor(theme.Highlight),
	}
}

func init() {
	cobra.OnInitialize(InitConfig)
	var theme config.Theme
	err := config.FilepathOr("theme", &theme)
	if err == nil && viper.InConfig("theme") {
		uiTheme := configToUiTheme(theme)
		ui.SetTheme(uiTheme)
	}

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
	rootCmd.PersistentFlags().
		BoolVar(&streamFlag, "stream", true, "enable streaming output for faster response display")
	rootCmd.PersistentFlags().
		BoolVar(&compactMode, "compact", false, "enable compact output mode without fancy styling")
	rootCmd.PersistentFlags().
		BoolVar(&noHooks, "no-hooks", false, "disable all hooks execution")

	// Session management flags
	rootCmd.PersistentFlags().
		StringVar(&saveSessionPath, "save-session", "", "save session to file after each message")
	rootCmd.PersistentFlags().
		StringVar(&loadSessionPath, "load-session", "", "load session from file at startup")
	rootCmd.PersistentFlags().
		StringVarP(&sessionPath, "session", "s", "", "session file to load and update")

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&providerURL, "provider-url", "", "base URL for the provider API (applies to OpenAI, Anthropic, Ollama, and Google)")
	flags.StringVar(&providerAPIKey, "provider-api-key", "", "API key for the provider (applies to OpenAI, Anthropic, and Google)")
	flags.BoolVar(&tlsSkipVerify, "tls-skip-verify", false, "skip TLS certificate verification (WARNING: insecure, use only for self-signed certificates)")

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
	viper.BindPFlag("stream", rootCmd.PersistentFlags().Lookup("stream"))
	viper.BindPFlag("compact", rootCmd.PersistentFlags().Lookup("compact"))
	viper.BindPFlag("no-hooks", rootCmd.PersistentFlags().Lookup("no-hooks"))
	viper.BindPFlag("provider-url", rootCmd.PersistentFlags().Lookup("provider-url"))
	viper.BindPFlag("provider-api-key", rootCmd.PersistentFlags().Lookup("provider-api-key"))
	viper.BindPFlag("max-tokens", rootCmd.PersistentFlags().Lookup("max-tokens"))
	viper.BindPFlag("temperature", rootCmd.PersistentFlags().Lookup("temperature"))
	viper.BindPFlag("top-p", rootCmd.PersistentFlags().Lookup("top-p"))
	viper.BindPFlag("top-k", rootCmd.PersistentFlags().Lookup("top-k"))
	viper.BindPFlag("stop-sequences", rootCmd.PersistentFlags().Lookup("stop-sequences"))
	viper.BindPFlag("num-gpu-layers", rootCmd.PersistentFlags().Lookup("num-gpu-layers"))
	viper.BindPFlag("main-gpu", rootCmd.PersistentFlags().Lookup("main-gpu"))
	viper.BindPFlag("tls-skip-verify", rootCmd.PersistentFlags().Lookup("tls-skip-verify"))

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
		// Use the new config loader
		mcpConfig, err = config.LoadAndValidateConfig()
		if err != nil {
			return fmt.Errorf("failed to load MCP config: %v", err)
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
		TLSSkipVerify:  viper.GetBool("tls-skip-verify"),
	}

	// Create spinner function for agent creation
	var spinnerFunc agent.SpinnerFunc
	if !quietFlag {
		spinnerFunc = func(message string, fn func() error) error {
			tempCli, tempErr := ui.NewCLI(viper.GetBool("debug"), viper.GetBool("compact"))
			if tempErr == nil {
				return tempCli.ShowSpinner(message, fn)
			}
			// Fallback without spinner
			return fn()
		}
	}

	// Create the agent using the factory
	// Use a buffered debug logger to capture messages during initialization
	var bufferedLogger *tools.BufferedDebugLogger
	var debugLogger tools.DebugLogger
	if viper.GetBool("debug") {
		bufferedLogger = tools.NewBufferedDebugLogger(true)
		debugLogger = bufferedLogger
	}

	mcpAgent, err := agent.CreateAgent(ctx, &agent.AgentCreationOptions{ModelConfig: modelConfig,
		MCPConfig:        mcpConfig,
		SystemPrompt:     systemPrompt,
		MaxSteps:         viper.GetInt("max-steps"),
		StreamingEnabled: viper.GetBool("stream"),
		ShowSpinner:      true,
		Quiet:            quietFlag,
		SpinnerFunc:      spinnerFunc,
		DebugLogger:      debugLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to create agent: %v", err)
	}
	defer mcpAgent.Close()

	// Initialize hook executor if hooks are configured
	// Get model name for display
	modelString := viper.GetString("model")
	parts := strings.SplitN(modelString, ":", 2)
	modelName := "Unknown"
	if len(parts) == 2 {
		modelName = parts[1]
	}

	var hookExecutor *hooks.Executor
	if hooksConfig := viper.Get("hooks"); hooksConfig != nil {
		if hc, ok := hooksConfig.(*hooks.HookConfig); ok {
			// Generate a session ID for this run
			sessionID := fmt.Sprintf("mcphost-%d", time.Now().Unix())
			transcriptPath := "" // We could add transcript logging later
			hookExecutor = hooks.NewExecutor(hc, sessionID, transcriptPath)

			// Set model and interactive mode
			hookExecutor.SetModel(modelString)
			hookExecutor.SetInteractive(promptFlag == "") // Interactive if no prompt flag
		}
	}

	// Create an adapter for the agent to match the UI interface
	agentAdapter := &agentUIAdapter{agent: mcpAgent}

	// Create CLI interface using the factory
	cli, err := ui.SetupCLI(&ui.CLISetupOptions{
		Agent:          agentAdapter,
		ModelString:    modelString,
		Debug:          viper.GetBool("debug"),
		Compact:        viper.GetBool("compact"),
		Quiet:          quietFlag,
		ShowDebug:      false, // Will be handled separately below
		ProviderAPIKey: viper.GetString("provider-api-key"),
	})
	if err != nil {
		return fmt.Errorf("failed to setup CLI: %v", err)
	}

	// Display buffered debug messages if any
	if bufferedLogger != nil && cli != nil {
		messages := bufferedLogger.GetMessages()
		if len(messages) > 0 {
			// Combine all messages into a single debug output
			combinedMessage := strings.Join(messages, "\n  ")
			cli.DisplayDebugMessage(combinedMessage)
		}
	}

	// Display debug configuration if debug mode is enabled
	if !quietFlag && cli != nil && viper.GetBool("debug") {
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

		// Add TLS skip verify if enabled
		if viper.GetBool("tls-skip-verify") {
			debugConfig["tls-skip-verify"] = true
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

		// Add MCP server configuration for debugging
		if len(mcpConfig.MCPServers) > 0 {
			mcpServers := make(map[string]any)
			loadedServers := mcpAgent.GetLoadedServerNames()
			loadedServerSet := make(map[string]bool)
			for _, name := range loadedServers {
				loadedServerSet[name] = true
			}

			for name, server := range mcpConfig.MCPServers {
				serverInfo := map[string]any{
					"type":   server.Type,
					"status": "failed", // Default to failed
				}

				// Mark as loaded if it's in the loaded servers list
				if loadedServerSet[name] {
					serverInfo["status"] = "loaded"
				}

				if len(server.Command) > 0 {
					serverInfo["command"] = server.Command
				}
				if len(server.Environment) > 0 {
					// Mask sensitive environment variables
					maskedEnv := make(map[string]string)
					for k, v := range server.Environment {
						if strings.Contains(strings.ToLower(k), "token") ||
							strings.Contains(strings.ToLower(k), "key") ||
							strings.Contains(strings.ToLower(k), "secret") {
							maskedEnv[k] = "[MASKED]"
						} else {
							maskedEnv[k] = v
						}
					}
					serverInfo["environment"] = maskedEnv
				}
				if server.URL != "" {
					serverInfo["url"] = server.URL
				}
				if server.Name != "" {
					serverInfo["name"] = server.Name
				}
				mcpServers[name] = serverInfo
			}
			debugConfig["mcpServers"] = mcpServers
		}
		cli.DisplayDebugConfig(debugConfig)
	}

	// Prepare data for slash commands
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

	// Main interaction logic
	var messages []*schema.Message
	var sessionManager *session.Manager
	if sessionPath != "" {
		_, err := os.Stat(sessionPath)
		if os.IsNotExist(err) {
			content := []byte("{}")
			if err := os.WriteFile(sessionPath, content, 0664); err != nil {
				panic(err)
			}
		}
		loadSessionPath = sessionPath
		saveSessionPath = sessionPath
	}

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
		return runNonInteractiveMode(ctx, mcpAgent, cli, promptFlag, modelName, messages, quietFlag, noExitFlag, mcpConfig, sessionManager, hookExecutor)
	}

	// Quiet mode is not allowed in interactive mode
	if quietFlag {
		return fmt.Errorf("--quiet flag can only be used with --prompt/-p")
	}

	return runInteractiveMode(ctx, mcpAgent, cli, serverNames, toolNames, modelName, messages, sessionManager, hookExecutor)
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

// addMessagesToHistory adds messages to the conversation history and saves to session if available
func addMessagesToHistory(messages *[]*schema.Message, sessionManager *session.Manager, cli *ui.CLI, newMessages ...*schema.Message) {
	// Add to local history
	*messages = append(*messages, newMessages...)

	// Save to session if session manager is available
	if sessionManager != nil {
		// Use ReplaceAllMessages to ensure session matches local history exactly
		if err := sessionManager.ReplaceAllMessages(*messages); err != nil {
			// Log error but don't fail the operation
			if cli != nil {
				cli.DisplayError(fmt.Errorf("failed to save messages to session: %v", err))
			}
		}
	}
}

// replaceMessagesHistory replaces the conversation history and saves to session if available
func replaceMessagesHistory(messages *[]*schema.Message, sessionManager *session.Manager, cli *ui.CLI, newMessages []*schema.Message) {
	// Replace local history
	*messages = newMessages

	// Save to session if session manager is available
	if sessionManager != nil {
		// Use ReplaceAllMessages to ensure session matches local history exactly
		if err := sessionManager.ReplaceAllMessages(*messages); err != nil {
			// Log error but don't fail the operation
			if cli != nil {
				cli.DisplayError(fmt.Errorf("failed to save messages to session: %v", err))
			}
		}
	}
}

// runAgenticLoop handles all execution modes with a single unified loop
func runAgenticLoop(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, messages []*schema.Message, config AgenticLoopConfig, hookExecutor *hooks.Executor) error {
	// Handle initial prompt for non-interactive modes
	if !config.IsInteractive && config.InitialPrompt != "" {
		// Execute UserPromptSubmit hooks for non-interactive mode
		if hookExecutor != nil {
			input := &hooks.UserPromptSubmitInput{
				CommonInput: hookExecutor.PopulateCommonFields(hooks.UserPromptSubmit),
				Prompt:      config.InitialPrompt,
			}

			hookOutput, err := hookExecutor.ExecuteHooks(ctx, hooks.UserPromptSubmit, input)
			if err != nil {
				// Log error but don't fail
				if debugMode {
					fmt.Fprintf(os.Stderr, "UserPromptSubmit hook execution error: %v\n", err)
				}
			}

			// Check if hook blocked the prompt
			if hookOutput != nil && hookOutput.Decision == "block" {
				return fmt.Errorf("prompt blocked by hook: %s", hookOutput.Reason)
			}
		}

		// Display user message (skip if quiet)
		if !config.Quiet && cli != nil {
			cli.DisplayUserMessage(config.InitialPrompt)
		}

		// Create temporary messages with user input for processing (don't add to history yet)
		tempMessages := append(messages, schema.UserMessage(config.InitialPrompt))

		// Process the initial prompt with tool calls
		_, conversationMessages, err := runAgenticStep(ctx, mcpAgent, cli, tempMessages, config, hookExecutor)
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
			// conversationMessages already includes the user message, tool calls, and final response
			replaceMessagesHistory(&messages, config.SessionManager, cli, conversationMessages)

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
		return runInteractiveLoop(ctx, mcpAgent, cli, messages, config, hookExecutor)
	}

	return nil
}

// runAgenticStep processes a single step of the agentic loop (handles tool calls)
func runAgenticStep(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, messages []*schema.Message, config AgenticLoopConfig, hookExecutor *hooks.Executor) (*schema.Message, []*schema.Message, error) {
	var currentSpinner *ui.Spinner

	// Start initial spinner (skip if quiet)
	if !config.Quiet && cli != nil {
		currentSpinner = ui.NewSpinner("Thinking...")
		currentSpinner.Start()
	}

	// Create streaming callback for real-time display
	var streamingCallback agent.StreamingResponseHandler
	var responseWasStreamed bool
	var lastDisplayedContent string
	var streamingContent strings.Builder
	var streamingStarted bool
	if cli != nil && !config.Quiet {
		streamingCallback = func(chunk string) {
			// Stop spinner before first chunk if still running
			if currentSpinner != nil {
				currentSpinner.Stop()
				currentSpinner = nil
			}
			// Mark that this response is being streamed
			responseWasStreamed = true

			// Start streaming message on first chunk
			if !streamingStarted {
				cli.StartStreamingMessage(config.ModelName)
				streamingStarted = true
				streamingContent.Reset() // Reset content for new streaming session
			}

			// Accumulate content and update message
			streamingContent.WriteString(chunk)
			cli.UpdateStreamingMessage(streamingContent.String())
		}
	}

	// Reset streaming state before agent execution
	responseWasStreamed = false
	streamingStarted = false
	streamingContent.Reset()

	// Variables to store tool information for hooks
	var currentToolName string
	var currentToolArgs string
	var toolIsBlocked bool
	var blockReason string

	result, err := mcpAgent.GenerateWithLoopAndStreaming(ctx, messages,
		// Tool call handler - called when a tool is about to be executed
		func(toolName, toolArgs string) {
			// Store tool info for use in execution handler
			currentToolName = toolName
			currentToolArgs = toolArgs

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
			if isStarting {
				// Execute PreToolUse hooks
				if hookExecutor != nil {
					input := &hooks.PreToolUseInput{
						CommonInput: hookExecutor.PopulateCommonFields(hooks.PreToolUse),
						ToolName:    currentToolName,
						ToolInput:   json.RawMessage(currentToolArgs),
					}

					hookOutput, err := hookExecutor.ExecuteHooks(ctx, hooks.PreToolUse, input)
					if err != nil {
						// Log error but don't fail the tool execution
						if debugMode {
							fmt.Fprintf(os.Stderr, "Hook execution error: %v\n", err)
						}
					}

					// Check if hook blocked the execution
					if hookOutput != nil && hookOutput.Decision == "block" {
						toolIsBlocked = true
						blockReason = hookOutput.Reason
						if blockReason == "" {
							blockReason = "Tool execution blocked by security policy"
						}
						if !config.Quiet && cli != nil {
							cli.DisplayInfo(fmt.Sprintf("Tool execution blocked by hook: %s", blockReason))
						}
					}
				}

				if !config.Quiet && cli != nil {
					// Start spinner for tool execution
					currentSpinner = ui.NewSpinner(fmt.Sprintf("Executing %s...", toolName))
					currentSpinner.Start()
				}
			} else {
				// Stop spinner when tool execution completes
				if !config.Quiet && cli != nil && currentSpinner != nil {
					currentSpinner.Stop()
					currentSpinner = nil
				}
			}
		},
		// Tool result handler - called when a tool execution completes
		func(toolName, toolArgs, result string, isError bool) {
			// Check if this tool was blocked
			if toolIsBlocked {
				// Reset the flag for next tool
				toolIsBlocked = false

				// Override the result with a block message
				blockedResult := fmt.Sprintf(`{"error": "Tool execution blocked", "message": "%s"}`, blockReason)
				result = blockedResult
				isError = true

				// Display the blocked message
				if !config.Quiet && cli != nil {
					cli.DisplayToolMessage(toolName, toolArgs, fmt.Sprintf("Tool execution blocked: %s", blockReason), true)
				}

				// Reset block reason
				blockReason = ""
				return
			}

			// Execute PostToolUse hooks
			var postToolHookOutput *hooks.HookOutput
			if hookExecutor != nil && result != "" {
				input := &hooks.PostToolUseInput{
					CommonInput:  hookExecutor.PopulateCommonFields(hooks.PostToolUse),
					ToolName:     currentToolName,
					ToolInput:    json.RawMessage(currentToolArgs),
					ToolResponse: json.RawMessage(result),
				}

				hookOutput, err := hookExecutor.ExecuteHooks(ctx, hooks.PostToolUse, input)
				if err != nil {
					// Log error but don't fail
					if debugMode {
						fmt.Fprintf(os.Stderr, "PostToolUse hook execution error: %v\n", err)
					}
				}
				postToolHookOutput = hookOutput
			}

			// Check if hook wants to suppress output
			if postToolHookOutput != nil && postToolHookOutput.SuppressOutput {
				// Skip displaying tool result to user
				// Note: Result still goes to LLM unless ModifyOutput is used
				return
			}

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
				// Reset streaming state for next LLM call
				responseWasStreamed = false
				streamingStarted = false
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
			if !config.Quiet && cli != nil && !responseWasStreamed {
				// Only display if content wasn't already streamed
				// Stop spinner before displaying content
				if currentSpinner != nil {
					currentSpinner.Stop()
					currentSpinner = nil
				}
				cli.DisplayAssistantMessageWithModel(content, config.ModelName)
				lastDisplayedContent = content
				// Start spinner again for tool calls
				currentSpinner = ui.NewSpinner("Thinking...")
				currentSpinner.Start()
			} else if responseWasStreamed {
				// Content was already streamed, just track it and manage spinner
				lastDisplayedContent = content
				if currentSpinner != nil {
					currentSpinner.Stop()
					currentSpinner = nil
				}
				// Start spinner again for tool calls
				currentSpinner = ui.NewSpinner("Thinking...")
				currentSpinner.Start()
			}
		},
		streamingCallback, // Add streaming callback as the last parameter
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

	// Extract the last user message for usage tracking (do this once)
	lastUserMessage := ""
	if len(messages) > 0 {
		// Find the last user message
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == schema.User {
				lastUserMessage = messages[i].Content
				break
			}
		}
	}

	// Update usage tracking for ALL responses (streaming and non-streaming)
	if !config.Quiet && cli != nil {
		cli.UpdateUsageFromResponse(response, lastUserMessage)
	}

	// Display assistant response with model name
	// Skip if: quiet mode, same content already displayed, or if streaming completed the full response
	streamedFullResponse := responseWasStreamed && streamingContent.String() == response.Content
	if !config.Quiet && cli != nil && response.Content != lastDisplayedContent && response.Content != "" && !streamedFullResponse {
		if err := cli.DisplayAssistantMessageWithModel(response.Content, config.ModelName); err != nil {
			cli.DisplayError(fmt.Errorf("display error: %v", err))
			return nil, nil, err
		}
	} else if config.Quiet {
		// In quiet mode, only output the final response content to stdout
		fmt.Print(response.Content)
	}

	// Display usage information immediately after the response (for both streaming and non-streaming)
	if !config.Quiet && cli != nil {
		cli.DisplayUsageAfterResponse()
	}

	// Execute Stop hook after agent has finished responding
	executeStopHook(hookExecutor, response, "completed", config.ModelName)

	// Return the final response and all conversation messages
	return response, conversationMessages, nil
}

// executeStopHook executes the Stop hook if a hook executor is available
func executeStopHook(hookExecutor *hooks.Executor, response *schema.Message, stopReason string, modelName string) {
	if hookExecutor != nil {
		// Prepare metadata
		var meta json.RawMessage
		if response != nil {
			metaData := map[string]interface{}{
				"model":          modelName,
				"role":           string(response.Role),
				"has_tool_calls": len(response.ToolCalls) > 0,
			}
			if metaBytes, err := json.Marshal(metaData); err == nil {
				meta = json.RawMessage(metaBytes)
			}
		}

		responseContent := ""
		if response != nil {
			responseContent = response.Content
		}

		input := &hooks.StopInput{
			CommonInput:    hookExecutor.PopulateCommonFields(hooks.Stop),
			StopHookActive: true,
			Response:       responseContent,
			StopReason:     stopReason,
			Meta:           meta,
		}

		// Execute Stop hook (ignore errors as we're exiting anyway)
		hookExecutor.ExecuteHooks(context.Background(), hooks.Stop, input)
	}
}

// runInteractiveLoop handles the interactive portion of the agentic loop
func runInteractiveLoop(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, messages []*schema.Message, config AgenticLoopConfig, hookExecutor *hooks.Executor) error {
	for {
		// Get user input
		prompt, err := cli.GetPrompt()
		if err == io.EOF {
			fmt.Println("\n  Goodbye!")
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to get prompt: %v", err)
		}

		if prompt == "" {
			continue
		}

		// Execute UserPromptSubmit hooks
		if hookExecutor != nil {
			input := &hooks.UserPromptSubmitInput{
				CommonInput: hookExecutor.PopulateCommonFields(hooks.UserPromptSubmit),
				Prompt:      prompt,
			}

			hookOutput, err := hookExecutor.ExecuteHooks(ctx, hooks.UserPromptSubmit, input)
			if err != nil {
				// Log error but don't fail
				if debugMode {
					fmt.Fprintf(os.Stderr, "UserPromptSubmit hook execution error: %v\n", err)
				}
			}

			// Check if hook blocked the prompt
			if hookOutput != nil && hookOutput.Decision == "block" {
				if cli != nil {
					cli.DisplayInfo(fmt.Sprintf("Prompt blocked: %s", hookOutput.Reason))
				}
				continue // Skip this prompt
			}

			// Check if hook wants to stop the session
			if hookOutput != nil && hookOutput.Continue != nil && !*hookOutput.Continue {
				if hookOutput.StopReason != "" {
					cli.DisplayInfo(fmt.Sprintf("Session ended by hook: %s", hookOutput.StopReason))
				}
				return nil // Exit interactive loop gracefully
			}
		}
		// Handle slash commands
		if cli.IsSlashCommand(prompt) {
			result := cli.HandleSlashCommand(prompt, config.ServerNames, config.ToolNames)
			if result.Handled {
				// If the command was to clear history, clear the messages slice and session
				if result.ClearHistory {
					messages = messages[:0] // Clear the slice
					// Use unified function to clear session as well
					addMessagesToHistory(&messages, config.SessionManager, cli)
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
		_, conversationMessages, err := runAgenticStep(ctx, mcpAgent, cli, tempMessages, config, hookExecutor)
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
		// conversationMessages already includes the user message, tool calls, and final response
		addMessagesToHistory(&messages, config.SessionManager, cli, conversationMessages...)
	}
}

// runNonInteractiveMode handles the non-interactive mode execution
func runNonInteractiveMode(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, prompt, modelName string, messages []*schema.Message, quiet, noExit bool, mcpConfig *config.Config, sessionManager *session.Manager, hookExecutor *hooks.Executor) error {
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

	return runAgenticLoop(ctx, mcpAgent, cli, messages, config, hookExecutor)
}

// runInteractiveMode handles the interactive mode execution
func runInteractiveMode(ctx context.Context, mcpAgent *agent.Agent, cli *ui.CLI, serverNames, toolNames []string, modelName string, messages []*schema.Message, sessionManager *session.Manager, hookExecutor *hooks.Executor) error {
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

	return runAgenticLoop(ctx, mcpAgent, cli, messages, config, hookExecutor)
}
