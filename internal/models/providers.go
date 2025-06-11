package models

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/ollama/ollama/api"
	"google.golang.org/genai"

	"github.com/mark3labs/mcphost/internal/models/gemini"
)

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
}

// CreateProvider creates an eino ToolCallingChatModel based on the provider configuration
func CreateProvider(ctx context.Context, config *ProviderConfig) (model.ToolCallingChatModel, error) {
	parts := strings.SplitN(config.ModelString, ":", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid model format. Expected provider:model, got %s", config.ModelString)
	}

	provider := parts[0]
	modelName := parts[1]

	switch provider {
	case "anthropic":
		return createAnthropicProvider(ctx, config, modelName)
	case "openai":
		return createOpenAIProvider(ctx, config, modelName)
	case "google":
		return createGoogleProvider(ctx, config, modelName)
	case "ollama":
		return createOllamaProvider(ctx, config, modelName)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func createAnthropicProvider(ctx context.Context, config *ProviderConfig, modelName string) (model.ToolCallingChatModel, error) {
	apiKey := config.ProviderAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Anthropic API key not provided. Use --provider-api-key flag or ANTHROPIC_API_KEY environment variable")
	}

	maxTokens := config.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // Default value
	}

	claudeConfig := &claude.Config{
		APIKey:    apiKey,
		Model:     modelName,
		MaxTokens: maxTokens,
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

	if config.MaxTokens > 0 {
		openaiConfig.MaxTokens = &config.MaxTokens
	}

	if config.Temperature != nil {
		openaiConfig.Temperature = config.Temperature
	}

	if config.TopP != nil {
		openaiConfig.TopP = config.TopP
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

	ollamaConfig.Options = options

	return ollama.NewChatModel(ctx, ollamaConfig)
}
