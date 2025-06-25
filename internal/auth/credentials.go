package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CredentialStore holds all stored credentials
type CredentialStore struct {
	Anthropic *AnthropicCredentials `json:"anthropic,omitempty"`
}

// AnthropicCredentials holds Anthropic API credentials
type AnthropicCredentials struct {
	Type         string    `json:"type"`                    // "oauth" or "api_key"
	APIKey       string    `json:"api_key,omitempty"`       // For API key auth
	AccessToken  string    `json:"access_token,omitempty"`  // For OAuth
	RefreshToken string    `json:"refresh_token,omitempty"` // For OAuth
	ExpiresAt    int64     `json:"expires_at,omitempty"`    // For OAuth
	CreatedAt    time.Time `json:"created_at"`
}

// IsExpired checks if the OAuth token is expired
func (c *AnthropicCredentials) IsExpired() bool {
	if c.Type != "oauth" || c.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() >= c.ExpiresAt
}

// NeedsRefresh checks if the OAuth token needs refresh (5 minutes before expiry)
func (c *AnthropicCredentials) NeedsRefresh() bool {
	if c.Type != "oauth" || c.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() >= (c.ExpiresAt - 300) // 5 minutes buffer
}

// CredentialManager handles credential storage and retrieval
type CredentialManager struct {
	credentialsPath string
}

// NewCredentialManager creates a new credential manager
func NewCredentialManager() (*CredentialManager, error) {
	credentialsPath, err := getCredentialsPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine credentials path: %w", err)
	}

	return &CredentialManager{
		credentialsPath: credentialsPath,
	}, nil
}

// getCredentialsPath returns the path to the credentials file
func getCredentialsPath() (string, error) {
	// Try XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, ".mcphost", "credentials.json"), nil
	}

	// Fall back to ~/.config/.mcphost/credentials.json
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".config", ".mcphost", "credentials.json"), nil
}

// LoadCredentials loads credentials from the file
func (cm *CredentialManager) LoadCredentials() (*CredentialStore, error) {
	// If file doesn't exist, return empty store
	if _, err := os.Stat(cm.credentialsPath); os.IsNotExist(err) {
		return &CredentialStore{}, nil
	}

	data, err := os.ReadFile(cm.credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var store CredentialStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}

	return &store, nil
}

// SaveCredentials saves credentials to the file
func (cm *CredentialManager) SaveCredentials(store *CredentialStore) error {
	// Ensure directory exists
	dir := filepath.Dir(cm.credentialsPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Write with restrictive permissions (read/write for owner only)
	if err := os.WriteFile(cm.credentialsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// SetAnthropicCredentials stores Anthropic API credentials (for API key auth)
func (cm *CredentialManager) SetAnthropicCredentials(apiKey string) error {
	if err := validateAnthropicAPIKey(apiKey); err != nil {
		return err
	}

	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}

	store.Anthropic = &AnthropicCredentials{
		Type:      "api_key",
		APIKey:    apiKey,
		CreatedAt: time.Now(),
	}

	return cm.SaveCredentials(store)
}

// GetAnthropicCredentials retrieves Anthropic API credentials
func (cm *CredentialManager) GetAnthropicCredentials() (*AnthropicCredentials, error) {
	store, err := cm.LoadCredentials()
	if err != nil {
		return nil, err
	}

	return store.Anthropic, nil
}

// RemoveAnthropicCredentials removes stored Anthropic credentials
func (cm *CredentialManager) RemoveAnthropicCredentials() error {
	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}

	store.Anthropic = nil

	// If store is empty, remove the file entirely
	if store.Anthropic == nil {
		if err := os.Remove(cm.credentialsPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove credentials file: %w", err)
		}
		return nil
	}

	return cm.SaveCredentials(store)
}

// HasAnthropicCredentials checks if Anthropic credentials are stored
func (cm *CredentialManager) HasAnthropicCredentials() (bool, error) {
	creds, err := cm.GetAnthropicCredentials()
	if err != nil {
		return false, err
	}
	if creds == nil {
		return false, nil
	}

	// Check based on credential type
	switch creds.Type {
	case "oauth":
		return creds.AccessToken != "", nil
	case "api_key":
		return creds.APIKey != "", nil
	default:
		return false, nil
	}
}

// GetCredentialsPath returns the path to the credentials file
func (cm *CredentialManager) GetCredentialsPath() string {
	return cm.credentialsPath
}

// validateAnthropicAPIKey validates the format of an Anthropic API key
func validateAnthropicAPIKey(apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// Anthropic API keys typically start with "sk-ant-" and are quite long
	if !strings.HasPrefix(apiKey, "sk-ant-") {
		return fmt.Errorf("invalid Anthropic API key format (should start with 'sk-ant-')")
	}

	if len(apiKey) < 20 {
		return fmt.Errorf("API key appears to be too short")
	}

	return nil
}

// GetAnthropicAPIKey is a convenience function that checks stored credentials first,
// then falls back to environment variables and flags
func GetAnthropicAPIKey(flagValue string) (string, string, error) {
	// 1. Check flag value first (highest priority)
	if flagValue != "" {
		return flagValue, "command-line flag", nil
	}

	// 2. Check stored credentials
	cm, err := NewCredentialManager()
	if err == nil {
		if creds, err := cm.GetAnthropicCredentials(); err == nil && creds != nil {
			if creds.Type == "oauth" && creds.AccessToken != "" {
				// For OAuth, get a valid access token (may refresh if needed)
				token, err := cm.GetValidAccessToken()
				if err != nil {
					return "", "", fmt.Errorf("failed to get valid OAuth token: %w", err)
				}
				return token, "stored OAuth credentials", nil
			} else if creds.Type == "api_key" && creds.APIKey != "" {
				return creds.APIKey, "stored API key", nil
			}
		}
	}

	// 3. Fall back to environment variable
	if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
		return envKey, "ANTHROPIC_API_KEY environment variable", nil
	}

	return "", "", fmt.Errorf("no Anthropic API key found. Use 'mcphost auth login anthropic', set ANTHROPIC_API_KEY environment variable, or use --provider-api-key flag")
}
