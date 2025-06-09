package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPServerConfig represents configuration for an MCP server
type MCPServerConfig struct {
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	URL     string   `json:"url,omitempty"`
	Headers []string `json:"headers,omitempty"`
}

// Config represents the application configuration
type Config struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// SystemPromptConfig represents system prompt configuration
type SystemPromptConfig struct {
	SystemPrompt string `json:"systemPrompt"`
}

// LoadMCPConfig loads MCP configuration from file
func LoadMCPConfig(configFile string) (*Config, error) {
	if configFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("error getting home directory: %v", err)
		}
		configFile = filepath.Join(homeDir, ".mcp.json")
	}

	// Create default config if file doesn't exist
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		defaultConfig := &Config{
			MCPServers: make(map[string]MCPServerConfig),
		}
		return defaultConfig, nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	return &config, nil
}

// LoadSystemPrompt loads system prompt from file
func LoadSystemPrompt(filePath string) (string, error) {
	if filePath == "" {
		return "", nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading system prompt file: %v", err)
	}

	var config SystemPromptConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("error parsing system prompt file: %v", err)
	}

	return config.SystemPrompt, nil
}