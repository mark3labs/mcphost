package agent

import (
	"context"
	"testing"

	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
)

func TestNewMCPAgent(t *testing.T) {
	ctx := context.Background()

	// Test with minimal configuration (no API keys, should fail gracefully)
	modelConfig := &models.ProviderConfig{
		ModelString: "anthropic:claude-3-5-sonnet-latest",
	}

	mcpConfig := &config.Config{
		MCPServers: make(map[string]config.MCPServerConfig),
	}

	agentConfig := &Config{
		ModelConfig:   modelConfig,
		MCPConfig:     mcpConfig,
		SystemPrompt:  "You are a helpful assistant.",
		MaxSteps:      10,
		MessageWindow: 5,
	}

	// This should fail due to missing API key, but the structure should be correct
	_, err := NewMCPAgent(ctx, agentConfig)
	if err == nil {
		t.Error("Expected error due to missing API key, but got none")
	}

	// Check that the error is about missing API key
	if err != nil && !contains(err.Error(), "API key") {
		t.Errorf("Expected API key error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}