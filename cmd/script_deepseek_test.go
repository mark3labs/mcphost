package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ramizpolic/mcphost/internal/models"
)

// TestDeepSeekChatScriptMode tests the regression where deepseek-chat model
// works in CLI mode but fails in script mode due to provider-url not being
// properly passed to model validation logic.
func TestDeepSeekChatScriptMode(t *testing.T) {
	// Create a temporary script file that mimics the issue scenario
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "deepseek-script.sh")

	scriptContent := `#!/usr/bin/env -S mcphost script
---
model: "openai:deepseek-chat"
provider-url: "https://api.deepseek.com/v1"
provider-api-key: "${env://DEEPSEEK_API_KEY}"
---
Calculate 3 times 4 equal to?
`

	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	// Set up environment variable
	os.Setenv("DEEPSEEK_API_KEY", "sk-test-key")
	defer os.Unsetenv("DEEPSEEK_API_KEY")

	// Parse the script file
	variables := map[string]string{}
	scriptConfig, err := parseScriptFile(scriptPath, variables)
	if err != nil {
		t.Fatalf("Failed to parse script: %v", err)
	}

	// Verify the script config has the correct values
	if scriptConfig.Model != "openai:deepseek-chat" {
		t.Errorf("Expected model=openai:deepseek-chat, got %s", scriptConfig.Model)
	}
	if scriptConfig.ProviderURL != "https://api.deepseek.com/v1" {
		t.Errorf("Expected provider-url=https://api.deepseek.com/v1, got %s", scriptConfig.ProviderURL)
	}
	if scriptConfig.ProviderAPIKey != "sk-test-key" {
		t.Errorf("Expected provider-api-key=sk-test-key, got %s", scriptConfig.ProviderAPIKey)
	}

	// Now test the actual model creation - this should NOT fail when provider-url is set
	providerConfig := &models.ProviderConfig{
		ModelString:    scriptConfig.Model,
		ProviderAPIKey: scriptConfig.ProviderAPIKey,
		ProviderURL:    scriptConfig.ProviderURL,
		MaxTokens:      scriptConfig.MaxTokens,
		Temperature:    scriptConfig.Temperature,
		TopP:           scriptConfig.TopP,
		TopK:           scriptConfig.TopK,
		StopSequences:  scriptConfig.StopSequences,
	}

	// This should succeed because provider-url is set, which should skip model validation
	ctx := context.Background()
	_, err = models.CreateProvider(ctx, providerConfig)

	// We expect this to fail with a connection error (since we're using a fake API key),
	// NOT with a "model not found" error. The "model not found" error indicates
	// that validation wasn't properly skipped.
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "model deepseek-chat not found for provider openai") {
			t.Errorf("Model validation should be skipped when provider-url is set, but got validation error: %v", err)
		}
		// Other errors (like connection errors) are expected and acceptable for this test
		t.Logf("Expected error (not model validation): %v", err)
	}
}

// TestDeepSeekChatCLIMode tests that the CLI mode works correctly with custom provider URL
func TestDeepSeekChatCLIMode(t *testing.T) {
	// Test the CLI mode behavior - this should work
	providerConfig := &models.ProviderConfig{
		ModelString:    "openai:deepseek-chat",
		ProviderAPIKey: "sk-test-key",
		ProviderURL:    "https://api.deepseek.com/v1", // This should skip validation
		MaxTokens:      0,
	}

	ctx := context.Background()
	_, err := models.CreateProvider(ctx, providerConfig)

	// We expect this to fail with a connection error (since we're using a fake API key),
	// NOT with a "model not found" error
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "model deepseek-chat not found for provider openai") {
			t.Errorf("CLI mode should skip validation when provider-url is set, but got validation error: %v", err)
		}
		// Other errors (like connection errors) are expected and acceptable for this test
		t.Logf("Expected error (not model validation): %v", err)
	}
}

// TestProviderURLValidationSkip tests that model validation is properly skipped
// when a custom provider URL is provided
func TestProviderURLValidationSkip(t *testing.T) {
	testCases := []struct {
		name        string
		model       string
		providerURL string
		shouldSkip  bool
	}{
		{
			name:        "OpenAI with custom URL should skip validation",
			model:       "openai:custom-model",
			providerURL: "https://api.custom.com/v1",
			shouldSkip:  true,
		},
		{
			name:        "OpenAI without custom URL should validate",
			model:       "openai:custom-model",
			providerURL: "",
			shouldSkip:  false,
		},
		{
			name:        "Ollama should always skip validation",
			model:       "ollama:custom-model",
			providerURL: "",
			shouldSkip:  true,
		},
		{
			name:        "Anthropic with custom URL should skip validation",
			model:       "anthropic:custom-model",
			providerURL: "https://api.custom.com/v1",
			shouldSkip:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			providerConfig := &models.ProviderConfig{
				ModelString:    tc.model,
				ProviderAPIKey: "test-key",
				ProviderURL:    tc.providerURL,
			}

			ctx := context.Background()
			_, err := models.CreateProvider(ctx, providerConfig)

			if tc.shouldSkip {
				// Validation should be skipped, so we shouldn't get "model not found" error
				if err != nil && strings.Contains(err.Error(), "not found for provider") {
					t.Errorf("Expected validation to be skipped, but got validation error: %v", err)
				}
			} else {
				// Validation should run, so we should get "model not found" error for custom models
				if err == nil || !strings.Contains(err.Error(), "not found for provider") {
					t.Errorf("Expected validation error for custom model, but got: %v", err)
				}
			}
		})
	}
}
