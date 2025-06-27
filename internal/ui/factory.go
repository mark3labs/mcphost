package ui

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcphost/internal/auth"
	"github.com/mark3labs/mcphost/internal/models"
)

// AgentInterface defines the interface we need from agent to avoid import cycles
type AgentInterface interface {
	GetLoadingMessage() string
	GetTools() []any                // Using any to avoid importing tool types
	GetLoadedServerNames() []string // Add this method for debug config
}

// CLISetupOptions contains options for setting up CLI
type CLISetupOptions struct {
	Agent          AgentInterface
	ModelString    string
	Debug          bool
	Compact        bool
	Quiet          bool
	ShowDebug      bool   // Whether to show debug config
	ProviderAPIKey string // For OAuth detection
}

// parseModelName extracts provider and model name from model string
func parseModelName(modelString string) (provider, model string) {
	parts := strings.SplitN(modelString, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "unknown", "unknown"
}

// SetupCLI creates and configures CLI with standard info display
func SetupCLI(opts *CLISetupOptions) (*CLI, error) {
	if opts.Quiet {
		return nil, nil // No CLI in quiet mode
	}

	cli, err := NewCLI(opts.Debug, opts.Compact)
	if err != nil {
		return nil, fmt.Errorf("failed to create CLI: %v", err)
	}

	// Parse model string for display and usage tracking
	provider, model := parseModelName(opts.ModelString)

	// Set the model name for consistent display
	if model != "unknown" {
		cli.SetModelName(model)
	}

	// Set up usage tracking for supported providers
	if provider != "unknown" && model != "unknown" {
		// Skip usage tracking for ollama as it's not in models.dev
		if provider != "ollama" {
			registry := models.GetGlobalRegistry()
			if modelInfo, err := registry.ValidateModel(provider, model); err == nil {
				// Check if OAuth credentials are being used for Anthropic models
				isOAuth := false
				if provider == "anthropic" {
					_, source, err := auth.GetAnthropicAPIKey(opts.ProviderAPIKey)
					if err == nil && strings.HasPrefix(source, "stored OAuth") {
						isOAuth = true
					}
				}

				usageTracker := NewUsageTracker(modelInfo, provider, 80, isOAuth) // Will be updated with actual width
				cli.SetUsageTracker(usageTracker)
			}
		}
	}

	// Display model info
	if provider != "unknown" && model != "unknown" {
		cli.DisplayInfo(fmt.Sprintf("Model loaded: %s (%s)", provider, model))
	}

	// Display loading message if available (e.g., GPU fallback info)
	if loadingMessage := opts.Agent.GetLoadingMessage(); loadingMessage != "" {
		cli.DisplayInfo(loadingMessage)
	}

	// Display tool count
	tools := opts.Agent.GetTools()
	cli.DisplayInfo(fmt.Sprintf("Loaded %d tools from MCP servers", len(tools)))

	return cli, nil
}
