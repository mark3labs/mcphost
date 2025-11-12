package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/tools"
)

// SpinnerFunc is a function type for showing spinners during agent creation.
// It executes the provided function while displaying a spinner with the given message.
type SpinnerFunc func(message string, fn func() error) error

// AgentCreationOptions contains options for creating an agent.
// It extends AgentConfig with UI-related options for showing progress during creation.
type AgentCreationOptions struct {
	// ModelConfig specifies the LLM provider and model to use
	ModelConfig *models.ProviderConfig
	// MCPConfig contains MCP server configurations
	MCPConfig *config.Config
	// SystemPrompt is the initial system message for the agent
	SystemPrompt string
	// MaxSteps limits the number of tool calls (0 for unlimited)
	MaxSteps int
	// StreamingEnabled controls whether responses are streamed
	StreamingEnabled bool
	// ShowSpinner indicates whether to show a spinner for Ollama models during loading
	ShowSpinner bool // For Ollama models
	// Quiet suppresses the spinner even if ShowSpinner is true
	Quiet bool // Skip spinner if quiet
	// SpinnerFunc is the function to show spinner, provided by the caller
	SpinnerFunc SpinnerFunc // Function to show spinner (provided by caller)
	// DebugLogger is an optional logger for debugging MCP communications
	DebugLogger tools.DebugLogger // Optional debug logger
}

// CreateAgent creates an agent with optional spinner for Ollama models.
// It shows a loading spinner for Ollama models if ShowSpinner is true and not in quiet mode.
// Returns the created agent or an error if creation fails.
func CreateAgent(ctx context.Context, opts *AgentCreationOptions) (*Agent, error) {
	agentConfig := &AgentConfig{
		ModelConfig:      opts.ModelConfig,
		MCPConfig:        opts.MCPConfig,
		SystemPrompt:     opts.SystemPrompt,
		MaxSteps:         opts.MaxSteps,
		StreamingEnabled: opts.StreamingEnabled,
		DebugLogger:      opts.DebugLogger,
	}

	var agent *Agent
	var err error

	// Show spinner for Ollama models if requested and not quiet
	if opts.ShowSpinner && strings.HasPrefix(opts.ModelConfig.ModelString, "ollama:") && !opts.Quiet && opts.SpinnerFunc != nil {
		err = opts.SpinnerFunc("Loading Ollama model...", func() error {
			agent, err = NewAgent(ctx, agentConfig)
			return err
		})
	} else {
		agent, err = NewAgent(ctx, agentConfig)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %v", err)
	}

	return agent, nil
}

// ParseModelName extracts provider and model name from a model string.
// Model strings are formatted as "provider:model" (e.g., "anthropic:claude-3-5-sonnet-20241022").
// If the string doesn't contain a colon, returns "unknown" for both provider and model.
func ParseModelName(modelString string) (provider, model string) {
	parts := strings.SplitN(modelString, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "unknown", "unknown"
}
