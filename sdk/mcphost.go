package sdk

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcphost/cmd"
	"github.com/mark3labs/mcphost/internal/agent"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/session"
	"github.com/spf13/viper"
)

// MCPHost provides programmatic access to mcphost functionality, allowing
// integration of MCP tools and LLM interactions into Go applications. It manages
// agents, sessions, and model configurations.
type MCPHost struct {
	agent       *agent.Agent
	sessionMgr  *session.Manager
	modelString string
}

// Options configures MCPHost creation with optional overrides for model,
// prompts, configuration, and behavior settings. All fields are optional
// and will use CLI defaults if not specified.
type Options struct {
	Model        string // Override model (e.g., "anthropic:claude-3-sonnet")
	SystemPrompt string // Override system prompt
	ConfigFile   string // Override config file path
	MaxSteps     int    // Override max steps (0 = use default)
	Streaming    bool   // Enable streaming (default from config)
	Quiet        bool   // Suppress debug output
}

// New creates an MCPHost instance using the same initialization as the CLI.
// It loads configuration, initializes MCP servers, creates the LLM model, and
// sets up the agent for interaction. Returns an error if initialization fails.
func New(ctx context.Context, opts *Options) (*MCPHost, error) {
	if opts == nil {
		opts = &Options{}
	}

	// Initialize config exactly like CLI does
	cmd.InitConfig()

	// Apply overrides after initialization
	if opts.ConfigFile != "" {
		// Load specific config file
		if err := cmd.LoadConfigWithEnvSubstitution(opts.ConfigFile); err != nil {
			return nil, fmt.Errorf("failed to load config file: %v", err)
		}
	}

	// Override viper settings with options
	if opts.Model != "" {
		viper.Set("model", opts.Model)
	}
	if opts.SystemPrompt != "" {
		viper.Set("system-prompt", opts.SystemPrompt)
	}
	if opts.MaxSteps > 0 {
		viper.Set("max-steps", opts.MaxSteps)
	}
	// Only override streaming if explicitly set
	viper.Set("stream", opts.Streaming)

	// Load MCP configuration using existing function
	mcpConfig, err := config.LoadAndValidateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load MCP config: %v", err)
	}

	// Load system prompt using existing function
	systemPrompt, err := config.LoadSystemPrompt(viper.GetString("system-prompt"))
	if err != nil {
		return nil, fmt.Errorf("failed to load system prompt: %v", err)
	}

	// Create model configuration (same as CLI in root.go:387-406)
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

	// Create agent using existing factory (same as CLI in root.go:431-440)
	a, err := agent.CreateAgent(ctx, &agent.AgentCreationOptions{
		ModelConfig:      modelConfig,
		MCPConfig:        mcpConfig,
		SystemPrompt:     systemPrompt,
		MaxSteps:         viper.GetInt("max-steps"),
		StreamingEnabled: viper.GetBool("stream"),
		ShowSpinner:      false, // No spinner for SDK
		Quiet:            opts.Quiet,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %v", err)
	}

	// Create session manager
	sessionMgr := session.NewManager("")

	return &MCPHost{
		agent:       a,
		sessionMgr:  sessionMgr,
		modelString: viper.GetString("model"),
	}, nil
}

// Prompt sends a message to the agent and returns the response. The agent may
// use tools as needed to generate the response. The conversation history is
// automatically maintained in the session. Returns an error if generation fails.
func (m *MCPHost) Prompt(ctx context.Context, message string) (string, error) {
	// Get messages from session
	messages := m.sessionMgr.GetMessages()

	// Add new user message
	userMsg := schema.UserMessage(message)
	messages = append(messages, userMsg)

	// Call agent (same as CLI does in root.go:902)
	result, err := m.agent.GenerateWithLoop(ctx, messages,
		nil, // onToolCall
		nil, // onToolExecution
		nil, // onToolResult
		nil, // onResponse
		nil, // onToolCallContent
		nil, // onToolApproval
	)
	if err != nil {
		return "", err
	}

	// Update session with all messages from the conversation
	// This preserves the complete history including tool calls
	if err := m.sessionMgr.ReplaceAllMessages(result.ConversationMessages); err != nil {
		return "", fmt.Errorf("failed to update session: %v", err)
	}

	return result.FinalResponse.Content, nil
}

// PromptWithCallbacks sends a message with callbacks for monitoring tool execution
// and streaming responses. The callbacks allow real-time observation of tool calls,
// results, and response generation. Returns the final response or an error.
func (m *MCPHost) PromptWithCallbacks(
	ctx context.Context,
	message string,
	onToolCall func(name, args string),
	onToolResult func(name, args, result string, isError bool),
	onStreaming func(chunk string),
) (string, error) {
	// Get messages from session
	messages := m.sessionMgr.GetMessages()

	// Add new user message
	userMsg := schema.UserMessage(message)
	messages = append(messages, userMsg)

	// Call agent with callbacks
	result, err := m.agent.GenerateWithLoopAndStreaming(ctx, messages,
		onToolCall,
		nil, // onToolExecution
		onToolResult,
		nil, // onResponse
		nil, // onToolCallContent
		onStreaming,
		nil, // onToolApproval
	)
	if err != nil {
		return "", err
	}

	// Update session
	if err := m.sessionMgr.ReplaceAllMessages(result.ConversationMessages); err != nil {
		return "", fmt.Errorf("failed to update session: %v", err)
	}

	return result.FinalResponse.Content, nil
}

// GetSessionManager returns the current session manager for direct access
// to conversation history and session manipulation.
func (m *MCPHost) GetSessionManager() *session.Manager {
	return m.sessionMgr
}

// LoadSession loads a previously saved session from a file, restoring the
// conversation history. Returns an error if the file cannot be loaded or parsed.
func (m *MCPHost) LoadSession(path string) error {
	s, err := session.LoadFromFile(path)
	if err != nil {
		return err
	}
	m.sessionMgr = session.NewManagerWithSession(s, path)
	return nil
}

// SaveSession saves the current session to a file for later restoration.
// Returns an error if the session cannot be written to the specified path.
func (m *MCPHost) SaveSession(path string) error {
	return m.sessionMgr.GetSession().SaveToFile(path)
}

// ClearSession clears the current session history, starting a new conversation
// with an empty message history.
func (m *MCPHost) ClearSession() {
	m.sessionMgr = session.NewManager("")
}

// GetModelString returns the current model string identifier (e.g.,
// "anthropic:claude-3-sonnet" or "openai:gpt-4") being used by the agent.
func (m *MCPHost) GetModelString() string {
	return m.modelString
}

// Close cleans up resources including MCP server connections and model resources.
// Should be called when the MCPHost instance is no longer needed. Returns an
// error if cleanup fails.
func (m *MCPHost) Close() error {
	return m.agent.Close()
}
