package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/ollama/ollama/api"
	"google.golang.org/genai"

	"github.com/mark3labs/mcphost/internal/auth"
	"github.com/mark3labs/mcphost/internal/models/gemini"
)

const (
	// ClaudeCodePrompt is the required system prompt for OAuth authentication
	ClaudeCodePrompt = "You are Claude Code, Anthropic's official CLI for Claude."
)

// resolveModelAlias resolves model aliases to their full names using the registry
func resolveModelAlias(provider, modelName string) string {
	registry := GetGlobalRegistry()
	
	// Common alias patterns for Anthropic models - using Claude 4 as the latest/default
	aliasMap := map[string]string{
		// Claude 4 models (latest and most capable)
		"claude-opus-latest":       "claude-opus-4-20250514",
		"claude-sonnet-latest":     "claude-sonnet-4-20250514",
		"claude-4-opus-latest":     "claude-opus-4-20250514",
		"claude-4-sonnet-latest":   "claude-sonnet-4-20250514",
		
		// Claude 3.x models for backward compatibility
		"claude-3-5-haiku-latest":  "claude-3-5-haiku-20241022",
		"claude-3-5-sonnet-latest": "claude-3-5-sonnet-20241022", 
		"claude-3-7-sonnet-latest": "claude-3-7-sonnet-20250219",
		"claude-3-opus-latest":     "claude-3-opus-20240229",
	}
	
	// Check if it's a known alias
	if resolved, exists := aliasMap[modelName]; exists {
		// Verify the resolved model exists in the registry
		if _, err := registry.ValidateModel(provider, resolved); err == nil {
			return resolved
		}
	}
	
	// Return original if no alias found or resolved model doesn't exist
	return modelName
}

// ProviderConfig holds configuration for creating LLM providers
type ProviderConfig struct {
	ModelString    string
	SystemPrompt   string
	ProviderAPIKey string // API key for OpenAI and Anthropic
	ProviderURL    string // Base URL for OpenAI, Anthropic, and Ollama

	// Model generation parameters
	MaxTokens     int
	Temperature   *float32
	TopP          *float32
	TopK          *int32
	StopSequences []string

	// Ollama-specific parameters
	NumGPU  *int32
	MainGPU *int32
}

// CreateProvider creates an eino ToolCallingChatModel based on the provider configuration
func CreateProvider(ctx context.Context, config *ProviderConfig) (model.ToolCallingChatModel, error) {
	parts := strings.SplitN(config.ModelString, ":", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid model format. Expected provider:model, got %s", config.ModelString)
	}

	provider := parts[0]
	modelName := parts[1]

	// Resolve model aliases before validation (for OAuth compatibility)
	if provider == "anthropic" {
		modelName = resolveModelAlias(provider, modelName)
	}

	// Get the global registry for validation
	registry := GetGlobalRegistry()

	// Validate the model exists (skip for ollama as it's not in models.dev, and skip when using custom provider URL)
	if provider != "ollama" && config.ProviderURL == "" {
		modelInfo, err := registry.ValidateModel(provider, modelName)
		if err != nil {
			// Provide helpful suggestions
			suggestions := registry.SuggestModels(provider, modelName)
			if len(suggestions) > 0 {
				return nil, fmt.Errorf("%v. Did you mean one of: %s", err, strings.Join(suggestions, ", "))
			}
			return nil, err
		}

		// Validate environment variables
		if err := registry.ValidateEnvironment(provider, config.ProviderAPIKey); err != nil {
			return nil, err
		}

		// Validate configuration parameters against model capabilities
		if err := validateModelConfig(config, modelInfo); err != nil {
			return nil, err
		}
	}

	switch provider {
	case "anthropic":
		return createAnthropicProvider(ctx, config, modelName)
	case "openai":
		return createOpenAIProvider(ctx, config, modelName)
	case "google":
		return createGoogleProvider(ctx, config, modelName)
	case "ollama":
		return createOllamaProvider(ctx, config, modelName)
	case "azure":
		return createAzureOpenAIProvider(ctx, config, modelName)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// validateModelConfig validates configuration parameters against model capabilities
func validateModelConfig(config *ProviderConfig, modelInfo *ModelInfo) error {
	// Omit temperature if not supported by the model
	if config.Temperature != nil && !modelInfo.Temperature {
		config.Temperature = nil
	}

	// Warn about context limits if MaxTokens is set too high
	if config.MaxTokens > modelInfo.Limit.Output {
		return fmt.Errorf("max_tokens (%d) exceeds model's output limit (%d) for %s",
			config.MaxTokens, modelInfo.Limit.Output, modelInfo.ID)
	}

	return nil
}

func createAzureOpenAIProvider(ctx context.Context, config *ProviderConfig, modelName string) (model.ToolCallingChatModel, error) {
	apiKey := config.ProviderAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("AZURE_OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Azure OpenAI API key not provided. Use --provider-api-key flag or AZURE_OPENAI_API_KEY environment variable")
	}

	azureConfig := &openai.ChatModelConfig{
		APIKey:     apiKey,
		Model:      modelName,
		ByAzure:    true,                 // Indicate this is an Azure OpenAI model
		APIVersion: "2025-01-01-preview", // Default Azure OpenAI API version
	}

	if config.ProviderURL != "" {
		azureConfig.BaseURL = config.ProviderURL
	} else {
		azureConfig.BaseURL = os.Getenv("AZURE_OPENAI_BASE_URL")
	}
	if azureConfig.BaseURL == "" {
		return nil, fmt.Errorf("Azure OpenAI Base URL not provided. Use --provider-url flag or AZURE_OPENAI_BASE_URL environment variable")
	}

	if config.MaxTokens > 0 {
		azureConfig.MaxTokens = &config.MaxTokens
	}

	if config.Temperature != nil {
		azureConfig.Temperature = config.Temperature
	}

	if config.TopP != nil {
		azureConfig.TopP = config.TopP
	}

	if len(config.StopSequences) > 0 {
		azureConfig.Stop = config.StopSequences
	}

	return openai.NewChatModel(ctx, azureConfig)
}

func createAnthropicProvider(ctx context.Context, config *ProviderConfig, modelName string) (model.ToolCallingChatModel, error) {
	apiKey, source, err := auth.GetAnthropicAPIKey(config.ProviderAPIKey)
	if err != nil {
		return nil, err
	}

	// Log the source of the API key in debug mode (without revealing the key)
	if os.Getenv("DEBUG") != "" || os.Getenv("MCPHOST_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "Using Anthropic API key from: %s\n", source)
	}

	// Model alias resolution is handled in CreateProvider

	maxTokens := config.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // Default value
	}

	claudeConfig := &claude.Config{
		Model:     modelName,
		MaxTokens: maxTokens,
	}

	// Handle OAuth vs API key authentication
	if strings.HasPrefix(source, "stored OAuth") {
		// For OAuth tokens, we need to use Authorization: Bearer header
		// Create a custom HTTP client that adds the proper headers
		claudeConfig.HTTPClient = createOAuthHTTPClient(apiKey)
		// Set a dummy API key to prevent the library from failing validation
		claudeConfig.APIKey = "oauth-placeholder"
	} else {
		// For API keys, use the standard x-api-key header
		claudeConfig.APIKey = apiKey
	}

	if config.ProviderURL != "" {
		claudeConfig.BaseURL = &config.ProviderURL
	}

	if config.Temperature != nil {
		claudeConfig.Temperature = config.Temperature
	}

	if config.TopP != nil {
		claudeConfig.TopP = config.TopP
	}

	if config.TopK != nil {
		claudeConfig.TopK = config.TopK
	}

	if len(config.StopSequences) > 0 {
		claudeConfig.StopSequences = config.StopSequences
	}

	return claude.NewChatModel(ctx, claudeConfig)
}

func createOpenAIProvider(ctx context.Context, config *ProviderConfig, modelName string) (model.ToolCallingChatModel, error) {
	apiKey := config.ProviderAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not provided. Use --provider-api-key flag or OPENAI_API_KEY environment variable")
	}

	openaiConfig := &openai.ChatModelConfig{
		APIKey: apiKey,
		Model:  modelName,
	}

	if config.ProviderURL != "" {
		openaiConfig.BaseURL = config.ProviderURL
	}

	// Check if this is a reasoning model to handle beta limitations (skip validation if using custom URL)
	registry := GetGlobalRegistry()
	isReasoningModel := false
	if config.ProviderURL == "" {
		if modelInfo, err := registry.ValidateModel("openai", modelName); err == nil && modelInfo.Reasoning {
			isReasoningModel = true
		}
	}

	if config.MaxTokens > 0 {
		if isReasoningModel {
			// For reasoning models, use MaxCompletionTokens instead of MaxTokens
			if openaiConfig.ExtraFields == nil {
				openaiConfig.ExtraFields = make(map[string]any)
			}
			openaiConfig.ExtraFields["max_completion_tokens"] = config.MaxTokens
		} else {
			// For non-reasoning models, use MaxTokens as usual
			openaiConfig.MaxTokens = &config.MaxTokens
		}
	}

	// For reasoning models, skip temperature and top_p due to beta limitations
	if !isReasoningModel {
		if config.Temperature != nil {
			openaiConfig.Temperature = config.Temperature
		}

		if config.TopP != nil {
			openaiConfig.TopP = config.TopP
		}
	}

	if len(config.StopSequences) > 0 {
		openaiConfig.Stop = config.StopSequences
	}

	return openai.NewChatModel(ctx, openaiConfig)
}

func createGoogleProvider(ctx context.Context, config *ProviderConfig, modelName string) (model.ToolCallingChatModel, error) {
	apiKey := config.ProviderAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Google API key not provided. Use --provider-api-key flag or GOOGLE_API_KEY/GEMINI_API_KEY environment variable")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Google client: %v", err)
	}

	geminiConfig := &gemini.Config{
		Client: client,
		Model:  modelName,
	}

	if config.MaxTokens > 0 {
		geminiConfig.MaxTokens = &config.MaxTokens
	}

	if config.Temperature != nil {
		geminiConfig.Temperature = config.Temperature
	}

	if config.TopP != nil {
		geminiConfig.TopP = config.TopP
	}

	if config.TopK != nil {
		geminiConfig.TopK = config.TopK
	}

	return gemini.NewChatModel(ctx, geminiConfig)
}

func createOllamaProvider(ctx context.Context, config *ProviderConfig, modelName string) (model.ToolCallingChatModel, error) {
	baseURL := "http://localhost:11434" // Default Ollama URL

	// Check for custom Ollama host from environment
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		baseURL = host
	}

	// Override with ProviderURL if provided
	if config.ProviderURL != "" {
		baseURL = config.ProviderURL
	}

	ollamaConfig := &ollama.ChatModelConfig{
		BaseURL: baseURL,
		Model:   modelName,
	}

	// Set up options for Ollama using the api.Options struct
	options := &api.Options{}

	if config.Temperature != nil {
		options.Temperature = *config.Temperature
	}

	if config.TopP != nil {
		options.TopP = *config.TopP
	}

	if config.TopK != nil {
		options.TopK = int(*config.TopK)
	}

	if len(config.StopSequences) > 0 {
		options.Stop = config.StopSequences
	}

	if config.MaxTokens > 0 {
		options.NumPredict = config.MaxTokens
	}

	// Set GPU configuration for Ollama
	if config.NumGPU != nil {
		options.NumGPU = int(*config.NumGPU)
	}

	if config.MainGPU != nil {
		options.MainGPU = int(*config.MainGPU)
	}

	ollamaConfig.Options = options

	return ollama.NewChatModel(ctx, ollamaConfig)
}

// createOAuthHTTPClient creates an HTTP client that adds OAuth headers for Anthropic API
func createOAuthHTTPClient(accessToken string) *http.Client {
	return &http.Client{
		Transport: &oauthTransport{
			accessToken: accessToken,
			base:        http.DefaultTransport,
		},
	}
}

// oauthTransport is an HTTP transport that adds OAuth headers
type oauthTransport struct {
	accessToken string
	base        http.RoundTripper
}

func (t *oauthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	newReq := req.Clone(req.Context())

	// Remove any existing x-api-key header (from the dummy API key)
	newReq.Header.Del("x-api-key")

	// Add OAuth headers as required by Anthropic's OAuth API
	newReq.Header.Set("Authorization", "Bearer "+t.accessToken)
	newReq.Header.Set("anthropic-beta", "oauth-2025-04-20")
	newReq.Header.Set("anthropic-version", "2023-06-01")

	// Inject Claude Code system prompt for /v1/messages endpoint
	if req.Method == "POST" && strings.Contains(req.URL.Path, "/v1/messages") && req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err == nil {
			modifiedBody, err := t.injectClaudeCodePrompt(body)
			if err == nil {
				newReq.Body = io.NopCloser(bytes.NewReader(modifiedBody))
				newReq.ContentLength = int64(len(modifiedBody))
			}
		}
	}

	// Use the base transport to make the request
	return t.base.RoundTrip(newReq)
}

// injectClaudeCodePrompt modifies the request body to inject Claude Code system prompt
func (t *oauthTransport) injectClaudeCodePrompt(body []byte) ([]byte, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return body, nil // Return original if not JSON
	}

	// Check if request has a system prompt
	systemRaw, hasSystem := data["system"]
	if !hasSystem {
		// No system prompt, inject Claude Code identification
		data["system"] = ClaudeCodePrompt
		return json.Marshal(data)
	}

	switch system := systemRaw.(type) {
	case string:
		// Handle string system prompt
		if system == ClaudeCodePrompt {
			// Already correct, leave as-is
			return body, nil
		}
		// Convert to array with Claude Code first
		data["system"] = []interface{}{
			map[string]interface{}{"type": "text", "text": ClaudeCodePrompt},
			map[string]interface{}{"type": "text", "text": system},
		}

	case []interface{}:
		// Handle array system prompt
		if len(system) > 0 {
			// Check if first element has correct text
			if first, ok := system[0].(map[string]interface{}); ok {
				if text, ok := first["text"].(string); ok && text == ClaudeCodePrompt {
					// Already has Claude Code first, return as-is
					return body, nil
				}
			}
		}
		// Prepend Claude Code identification
		newSystem := []interface{}{
			map[string]interface{}{"type": "text", "text": ClaudeCodePrompt},
		}
		data["system"] = append(newSystem, system...)
	}

	// Re-marshal the modified data
	return json.Marshal(data)
}
