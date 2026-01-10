//go:generate go run generate_models.go

package models

import (
	"fmt"
	"os"
	"strings"
)

// ModelsRegistry provides validation and information about models.
// It maintains a registry of all supported LLM providers and their models,
// including capabilities, pricing, and configuration requirements.
// The registry data is generated from models.dev and provides a single
// source of truth for model validation and discovery.
type ModelsRegistry struct {
	// providers maps provider IDs to their information and available models
	providers map[string]ProviderInfo
}

// NewModelsRegistry creates a new models registry with static data.
// The registry is populated with model information generated from models.dev,
// providing comprehensive metadata about available models across all supported providers.
//
// Returns:
//   - *ModelsRegistry: A new registry instance populated with current model data
func NewModelsRegistry() *ModelsRegistry {
	return &ModelsRegistry{
		providers: GetModelsData(),
	}
}

// ValidateModel validates if a model exists and returns detailed information.
// It checks whether a specific model is available for a given provider and
// returns comprehensive information about the model's capabilities and limits.
//
// Parameters:
//   - provider: The provider ID (e.g., "anthropic", "openai", "google")
//   - modelID: The specific model ID (e.g., "claude-3-sonnet-20240620", "gpt-4")
//
// Returns:
//   - *ModelInfo: Detailed information about the model including pricing, limits, and capabilities
//   - error: Returns an error if the provider is unsupported or model is not found
func (r *ModelsRegistry) ValidateModel(provider, modelID string) (*ModelInfo, error) {
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	modelInfo, exists := providerInfo.Models[modelID]
	if !exists {
		return nil, fmt.Errorf("model %s not found for provider %s", modelID, provider)
	}

	return &modelInfo, nil
}

// GetRequiredEnvVars returns the required environment variables for a provider.
// These are the environment variable names that should contain API keys or
// other authentication credentials for the specified provider.
//
// Parameters:
//   - provider: The provider ID (e.g., "anthropic", "openai", "google")
//
// Returns:
//   - []string: List of environment variable names the provider checks for credentials
//   - error: Returns an error if the provider is unsupported
//
// Example:
//
//	For "anthropic", returns ["ANTHROPIC_API_KEY"]
//	For "google", returns ["GOOGLE_API_KEY", "GEMINI_API_KEY", "GOOGLE_GENERATIVE_AI_API_KEY"]
func (r *ModelsRegistry) GetRequiredEnvVars(provider string) ([]string, error) {
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return providerInfo.Env, nil
}

// ValidateEnvironment checks if required environment variables are set.
// It verifies that at least one of the provider's required environment variables
// contains an API key, unless an API key is explicitly provided via configuration.
//
// Parameters:
//   - provider: The provider ID to validate environment for
//   - apiKey: An API key provided via configuration (if empty, checks environment variables)
//
// Returns:
//   - error: Returns nil if validation passes, or an error describing missing credentials
func (r *ModelsRegistry) ValidateEnvironment(provider string, apiKey string) error {
	envVars, err := r.GetRequiredEnvVars(provider)
	if err != nil {
		return err
	}

	// If API key is provided via config, we don't need to check env vars
	if apiKey != "" {
		return nil
	}

	// Add alternative environment variable names for google-vertex-anthropic
	// These match the env vars checked by eino-claude and other tools
	if provider == "google-vertex-anthropic" {
		envVars = append(envVars,
			"ANTHROPIC_VERTEX_PROJECT_ID",
			"GOOGLE_CLOUD_PROJECT",
			"GCLOUD_PROJECT",
			"CLOUDSDK_CORE_PROJECT",
			"ANTHROPIC_VERTEX_REGION",
			"CLOUD_ML_REGION",
		)
	}

	// Check if at least one environment variable is set
	var foundVar bool
	for _, envVar := range envVars {
		if os.Getenv(envVar) != "" {
			foundVar = true
			break
		}
	}

	if !foundVar {
		return fmt.Errorf("missing required environment variables for %s: %s (at least one required)",
			provider, strings.Join(envVars, ", "))
	}

	return nil
}

// SuggestModels returns similar model names when an invalid model is provided.
// It helps users discover the correct model ID by finding models that partially
// match the provided input, useful for correcting typos or finding alternatives.
//
// Parameters:
//   - provider: The provider ID to search within
//   - invalidModel: The invalid or misspelled model name to find suggestions for
//
// Returns:
//   - []string: A list of up to 5 suggested model IDs that partially match the input
func (r *ModelsRegistry) SuggestModels(provider, invalidModel string) []string {
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil
	}

	var suggestions []string
	invalidLower := strings.ToLower(invalidModel)

	// Look for models that contain parts of the invalid model name
	for modelID, modelInfo := range providerInfo.Models {
		modelIDLower := strings.ToLower(modelID)
		modelNameLower := strings.ToLower(modelInfo.Name)

		// Check if the invalid model is a substring of existing models
		if strings.Contains(modelIDLower, invalidLower) ||
			strings.Contains(modelNameLower, invalidLower) ||
			strings.Contains(invalidLower, strings.ToLower(strings.Split(modelID, "-")[0])) {
			suggestions = append(suggestions, modelID)
		}
	}

	// Limit suggestions to avoid overwhelming output
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}

	return suggestions
}

// GetSupportedProviders returns a list of all supported providers.
// This includes all providers that have models registered in the system,
// such as "anthropic", "openai", "google", "alibaba", etc.
//
// Returns:
//   - []string: A list of all provider IDs available in the registry
func (r *ModelsRegistry) GetSupportedProviders() []string {
	var providers []string
	for providerID := range r.providers {
		providers = append(providers, providerID)
	}
	return providers
}

// GetModelsForProvider returns all models for a specific provider.
// This is useful for listing available models when a user wants to see
// all options for a particular provider.
//
// Parameters:
//   - provider: The provider ID to get models for
//
// Returns:
//   - map[string]ModelInfo: A map of model IDs to their detailed information
//   - error: Returns an error if the provider is unsupported
func (r *ModelsRegistry) GetModelsForProvider(provider string) (map[string]ModelInfo, error) {
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return providerInfo.Models, nil
}

// Global registry instance
var globalRegistry = NewModelsRegistry()

// GetGlobalRegistry returns the global models registry instance.
// This provides a singleton registry that can be accessed throughout
// the application for model validation and information retrieval.
// The registry is initialized once with data from models.dev.
//
// Returns:
//   - *ModelsRegistry: The global registry instance
func GetGlobalRegistry() *ModelsRegistry {
	return globalRegistry
}
