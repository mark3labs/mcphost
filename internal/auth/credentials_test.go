package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialManager(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "mcphost-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a credential manager with a test path
	cm := &CredentialManager{
		credentialsPath: filepath.Join(tempDir, "credentials.json"),
	}

	// Test initial state - no credentials
	hasAuth, err := cm.HasAnthropicCredentials()
	if err != nil {
		t.Fatalf("HasAnthropicCredentials failed: %v", err)
	}
	if hasAuth {
		t.Error("Expected no credentials initially")
	}

	// Test setting credentials
	testAPIKey := "sk-ant-test-key-12345678901234567890"
	err = cm.SetAnthropicCredentials(testAPIKey)
	if err != nil {
		t.Fatalf("SetAnthropicCredentials failed: %v", err)
	}

	// Test that credentials are now present
	hasAuth, err = cm.HasAnthropicCredentials()
	if err != nil {
		t.Fatalf("HasAnthropicCredentials failed: %v", err)
	}
	if !hasAuth {
		t.Error("Expected credentials to be present")
	}

	// Test retrieving credentials
	creds, err := cm.GetAnthropicCredentials()
	if err != nil {
		t.Fatalf("GetAnthropicCredentials failed: %v", err)
	}
	if creds == nil {
		t.Fatal("Expected credentials to be returned")
	}
	if creds.APIKey != testAPIKey {
		t.Errorf("Expected API key %s, got %s", testAPIKey, creds.APIKey)
	}
	if creds.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	// Test removing credentials
	err = cm.RemoveAnthropicCredentials()
	if err != nil {
		t.Fatalf("RemoveAnthropicCredentials failed: %v", err)
	}

	// Test that credentials are gone
	hasAuth, err = cm.HasAnthropicCredentials()
	if err != nil {
		t.Fatalf("HasAnthropicCredentials failed: %v", err)
	}
	if hasAuth {
		t.Error("Expected no credentials after removal")
	}

	// Test that file is removed when empty
	if _, err := os.Stat(cm.credentialsPath); !os.IsNotExist(err) {
		t.Error("Expected credentials file to be removed when empty")
	}
}

func TestValidateAnthropicAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid key",
			apiKey:  "sk-ant-test-key-12345678901234567890",
			wantErr: false,
		},
		{
			name:    "empty key",
			apiKey:  "",
			wantErr: true,
		},
		{
			name:    "wrong prefix",
			apiKey:  "sk-test-key-12345678901234567890",
			wantErr: true,
		},
		{
			name:    "too short",
			apiKey:  "sk-ant-short",
			wantErr: true,
		},
		{
			name:    "with whitespace",
			apiKey:  "  sk-ant-test-key-12345678901234567890  ",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAnthropicAPIKey(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAnthropicAPIKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetAnthropicAPIKey(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "mcphost-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save original environment
	originalAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	originalXDGConfig := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Setenv("ANTHROPIC_API_KEY", originalAPIKey)
		os.Setenv("XDG_CONFIG_HOME", originalXDGConfig)
	}()

	// Set up test environment
	os.Setenv("XDG_CONFIG_HOME", tempDir)
	os.Unsetenv("ANTHROPIC_API_KEY")

	// Test 1: Flag value takes precedence
	flagKey := "sk-ant-flag-key-12345678901234567890"
	apiKey, source, err := GetAnthropicAPIKey(flagKey)
	if err != nil {
		t.Fatalf("GetAnthropicAPIKey failed: %v", err)
	}
	if apiKey != flagKey {
		t.Errorf("Expected flag key %s, got %s", flagKey, apiKey)
	}
	if source != "command-line flag" {
		t.Errorf("Expected source 'command-line flag', got %s", source)
	}

	// Test 2: Stored credentials when no flag
	cm, err := NewCredentialManager()
	if err != nil {
		t.Fatalf("NewCredentialManager failed: %v", err)
	}

	storedKey := "sk-ant-stored-key-12345678901234567890"
	err = cm.SetAnthropicCredentials(storedKey)
	if err != nil {
		t.Fatalf("SetAnthropicCredentials failed: %v", err)
	}

	apiKey, source, err = GetAnthropicAPIKey("")
	if err != nil {
		t.Fatalf("GetAnthropicAPIKey failed: %v", err)
	}
	if apiKey != storedKey {
		t.Errorf("Expected stored key %s, got %s", storedKey, apiKey)
	}
	if source != "stored API key" {
		t.Errorf("Expected source 'stored API key', got %s", source)
	}

	// Test 3: Environment variable when no flag or stored credentials
	err = cm.RemoveAnthropicCredentials()
	if err != nil {
		t.Fatalf("RemoveAnthropicCredentials failed: %v", err)
	}

	envKey := "sk-ant-env-key-12345678901234567890"
	os.Setenv("ANTHROPIC_API_KEY", envKey)

	apiKey, source, err = GetAnthropicAPIKey("")
	if err != nil {
		t.Fatalf("GetAnthropicAPIKey failed: %v", err)
	}
	if apiKey != envKey {
		t.Errorf("Expected env key %s, got %s", envKey, apiKey)
	}
	if source != "ANTHROPIC_API_KEY environment variable" {
		t.Errorf("Expected source 'ANTHROPIC_API_KEY environment variable', got %s", source)
	}

	// Test 4: No credentials available
	os.Unsetenv("ANTHROPIC_API_KEY")

	_, _, err = GetAnthropicAPIKey("")
	if err == nil {
		t.Error("Expected error when no credentials available")
	}
}

func TestCredentialStorePersistence(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "mcphost-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	credentialsPath := filepath.Join(tempDir, "credentials.json")

	// Create first manager and store credentials
	cm1 := &CredentialManager{credentialsPath: credentialsPath}
	testAPIKey := "sk-ant-test-key-12345678901234567890"

	err = cm1.SetAnthropicCredentials(testAPIKey)
	if err != nil {
		t.Fatalf("SetAnthropicCredentials failed: %v", err)
	}

	// Create second manager and verify credentials persist
	cm2 := &CredentialManager{credentialsPath: credentialsPath}

	creds, err := cm2.GetAnthropicCredentials()
	if err != nil {
		t.Fatalf("GetAnthropicCredentials failed: %v", err)
	}
	if creds == nil {
		t.Fatal("Expected credentials to persist")
	}
	if creds.APIKey != testAPIKey {
		t.Errorf("Expected API key %s, got %s", testAPIKey, creds.APIKey)
	}

	// Verify file permissions
	info, err := os.Stat(credentialsPath)
	if err != nil {
		t.Fatalf("Failed to stat credentials file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %v", info.Mode().Perm())
	}
}
