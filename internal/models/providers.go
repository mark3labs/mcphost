package models

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// ProviderConfig holds configuration for creating LLM providers
type ProviderConfig struct {
	ModelString      string
	SystemPrompt     string
	AnthropicAPIKey  string
	AnthropicBaseURL string
	OpenAIAPIKey     string
	OpenAIBaseURL    string
	GoogleAPIKey     string
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
	apiKey := config.AnthropicAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Anthropic API key not provided. Use --anthropic-api-key flag or ANTHROPIC_API_KEY environment variable")
	}

	claudeConfig := &claude.Config{
		APIKey:    apiKey,
		Model:     modelName,
		MaxTokens: 4096,
	}

	if config.AnthropicBaseURL != "" {
		claudeConfig.BaseURL = &config.AnthropicBaseURL
	}

	return claude.NewChatModel(ctx, claudeConfig)
}

func createOpenAIProvider(ctx context.Context, config *ProviderConfig, modelName string) (model.ToolCallingChatModel, error) {
	apiKey := config.OpenAIAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not provided. Use --openai-api-key flag or OPENAI_API_KEY environment variable")
	}

	openaiConfig := &openai.ChatModelConfig{
		APIKey: apiKey,
		Model:  modelName,
	}

	if config.OpenAIBaseURL != "" {
		openaiConfig.BaseURL = config.OpenAIBaseURL
	}

	return openai.NewChatModel(ctx, openaiConfig)
}

func createGoogleProvider(ctx context.Context, config *ProviderConfig, modelName string) (model.ToolCallingChatModel, error) {
	apiKey := config.GoogleAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Google API key not provided. Use --google-api-key flag or GOOGLE_API_KEY/GEMINI_API_KEY environment variable")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Google client: %v", err)
	}

	geminiConfig := &gemini.Config{
		Client: client,
		Model:  modelName,
	}

	return gemini.NewChatModel(ctx, geminiConfig)
}

func createOllamaProvider(ctx context.Context, config *ProviderConfig, modelName string) (model.ToolCallingChatModel, error) {
	ollamaConfig := &ollama.ChatModelConfig{
		BaseURL: "http://localhost:11434", // Default Ollama URL
		Model:   modelName,
	}

	// Check for custom Ollama host
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		ollamaConfig.BaseURL = host
	}

	return ollama.NewChatModel(ctx, ollamaConfig)
}