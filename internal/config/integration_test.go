package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestConfigLoadingWithEnvSubstitution(t *testing.T) {
	// Create a temporary config file with environment variables
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	configContent := `
mcpServers:
  github:
    type: local
    command: ["docker", "run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN=${env://GITHUB_TOKEN}", "ghcr.io/github/github-mcp-server"]
    environment:
      DEBUG: "${env://DEBUG:-false}"
      LOG_LEVEL: "${env://LOG_LEVEL:-info}"
  
  database:
    type: local
    command: ["python", "db-server.py"]
    environment:
      DATABASE_URL: "${env://DATABASE_URL:-sqlite:///tmp/default.db}"
      API_KEY: "${env://DB_API_KEY}"

model: "${env://MODEL:-anthropic:claude-sonnet-4-20250514}"
provider-api-key: "${env://OPENAI_API_KEY:-}"
debug: ${env://DEBUG:-false}
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set up environment variables
	os.Setenv("GITHUB_TOKEN", "ghp_test_token")
	os.Setenv("DEBUG", "true")
	os.Setenv("DB_API_KEY", "secret_key")
	defer func() {
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("DEBUG")
		os.Unsetenv("DB_API_KEY")
	}()

	// Test the loadConfigWithEnvSubstitution function
	viper.Reset() // Clear any existing config
	err = loadConfigWithEnvSubstitution(configPath)
	if err != nil {
		t.Fatalf("Failed to load config with env substitution: %v", err)
	}

	// Verify that environment variables were substituted correctly
	var config Config
	err = viper.Unmarshal(&config)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Check GitHub server config
	githubServer, exists := config.MCPServers["github"]
	if !exists {
		t.Fatal("GitHub server not found in config")
	}

	// Check that GITHUB_TOKEN was substituted in command
	expectedCommand := []string{"docker", "run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN=ghp_test_token", "ghcr.io/github/github-mcp-server"}
	if len(githubServer.Command) != len(expectedCommand) {
		t.Errorf("Expected command length %d, got %d", len(expectedCommand), len(githubServer.Command))
	}
	for i, expected := range expectedCommand {
		if i < len(githubServer.Command) && githubServer.Command[i] != expected {
			t.Errorf("Expected command[%d] = %s, got %s", i, expected, githubServer.Command[i])
		}
	}

	// Check environment variables (viper converts keys to lowercase)
	if githubServer.Environment["debug"] != "true" {
		t.Errorf("Expected debug=true, got %s", githubServer.Environment["debug"])
	}
	if githubServer.Environment["log_level"] != "info" {
		t.Errorf("Expected log_level=info, got %s", githubServer.Environment["log_level"])
	}

	// Check database server config
	dbServer, exists := config.MCPServers["database"]
	if !exists {
		t.Fatal("Database server not found in config")
	}

	if dbServer.Environment["database_url"] != "sqlite:///tmp/default.db" {
		t.Errorf("Expected database_url=sqlite:///tmp/default.db, got %s", dbServer.Environment["database_url"])
	}
	if dbServer.Environment["api_key"] != "secret_key" {
		t.Errorf("Expected api_key=secret_key, got %s", dbServer.Environment["api_key"])
	}

	// Check global config values
	if config.Model != "anthropic:claude-sonnet-4-20250514" {
		t.Errorf("Expected model=anthropic:claude-sonnet-4-20250514, got %s", config.Model)
	}
	if !config.Debug {
		t.Error("Expected debug=true")
	}
}

func TestConfigLoadingWithMissingRequiredEnvVar(t *testing.T) {
	// Create a temporary config file with a required environment variable
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	configContent := `
mcpServers:
  github:
    type: local
    command: ["gh", "api"]
    environment:
      GITHUB_TOKEN: "${env://REQUIRED_GITHUB_TOKEN}"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Make sure the environment variable is not set
	os.Unsetenv("REQUIRED_GITHUB_TOKEN")

	// Test that loading fails with a clear error
	viper.Reset()
	err = loadConfigWithEnvSubstitution(configPath)
	if err == nil {
		t.Fatal("Expected error for missing required environment variable")
	}

	if !strings.Contains(err.Error(), "required environment variable REQUIRED_GITHUB_TOKEN not set") {
		t.Errorf("Expected error about missing REQUIRED_GITHUB_TOKEN, got: %v", err)
	}
}

func TestJSONConfigWithEnvSubstitution(t *testing.T) {
	// Test JSON format as well
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	configContent := `{
  "mcpServers": {
    "test": {
      "type": "local",
      "command": ["echo", "${env://TEST_VALUE:-hello}"],
      "environment": {
        "VAR1": "${env://VAR1:-default1}",
        "VAR2": "${env://VAR2}"
      }
    }
  },
  "model": "${env://MODEL:-anthropic:claude-sonnet-4-20250514}"
}`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set one environment variable, leave others to use defaults
	os.Setenv("VAR2", "custom_value")
	defer os.Unsetenv("VAR2")

	viper.Reset()
	err = loadConfigWithEnvSubstitution(configPath)
	if err != nil {
		t.Fatalf("Failed to load JSON config with env substitution: %v", err)
	}

	var config Config
	err = viper.Unmarshal(&config)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	testServer := config.MCPServers["test"]
	expectedCommand := []string{"echo", "hello"}
	if len(testServer.Command) != len(expectedCommand) || testServer.Command[1] != "hello" {
		t.Errorf("Expected command %v, got %v", expectedCommand, testServer.Command)
	}

	if testServer.Environment["var1"] != "default1" {
		t.Errorf("Expected var1=default1, got %s", testServer.Environment["var1"])
	}
	if testServer.Environment["var2"] != "custom_value" {
		t.Errorf("Expected var2=custom_value, got %s", testServer.Environment["var2"])
	}
}

// Helper function to simulate the loadConfigWithEnvSubstitution function
// This is needed because the function is in cmd/root.go and we can't easily test it directly
func loadConfigWithEnvSubstitution(configPath string) error {
	// Read raw config file content
	rawContent, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Apply environment variable substitution
	substituter := &EnvSubstituter{}
	processedContent, err := substituter.SubstituteEnvVars(string(rawContent))
	if err != nil {
		return err
	}

	// Determine config type from file extension
	configType := "yaml"
	if strings.HasSuffix(configPath, ".json") {
		configType = "json"
	}

	// Use viper to parse the processed content
	viper.SetConfigType(configType)
	return viper.ReadConfig(strings.NewReader(processedContent))
}
