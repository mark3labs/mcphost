package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
)

// SpinnerFunc is a function type for showing spinners during agent creation
type SpinnerFunc func(message string, fn func() error) error

// AgentCreationOptions contains options for creating an agent
type AgentCreationOptions struct {
	ModelConfig      *models.ProviderConfig
	MCPConfig        *config.Config
	SystemPrompt     string
	MaxSteps         int
	StreamingEnabled bool
	ShowSpinner      bool        // For Ollama models
	Quiet            bool        // Skip spinner if quiet
	SpinnerFunc      SpinnerFunc // Function to show spinner (provided by caller)
}

// CreateAgent creates an agent with optional spinner for Ollama models
func CreateAgent(ctx context.Context, opts *AgentCreationOptions) (*Agent, error) {
	agentConfig := &AgentConfig{
		ModelConfig:      opts.ModelConfig,
		MCPConfig:        opts.MCPConfig,
		SystemPrompt:     opts.SystemPrompt,
		MaxSteps:         opts.MaxSteps,
		StreamingEnabled: opts.StreamingEnabled,
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

// ParseModelName extracts provider and model name from model string
func ParseModelName(modelString string) (provider, model string) {
	parts := strings.SplitN(modelString, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "unknown", "unknown"
}
