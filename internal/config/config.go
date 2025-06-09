package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// MCPServerConfig represents configuration for an MCP server
type MCPServerConfig struct {
	Command      string   `json:"command,omitempty"`
	Args         []string `json:"args,omitempty"`
	URL          string   `json:"url,omitempty"`
	Headers      []string `json:"headers,omitempty"`
	AllowedTools []string `json:"allowedTools,omitempty"`
	ExcludedTools []string `json:"excludedTools,omitempty"`
}

// Config represents the application configuration
type Config struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// Validate validates the configuration
func (c *Config) Validate() error {
	for serverName, serverConfig := range c.MCPServers {
		if len(serverConfig.AllowedTools) > 0 && len(serverConfig.ExcludedTools) > 0 {
			return fmt.Errorf("server %s: allowedTools and excludedTools are mutually exclusive", serverName)
		}
	}
	return nil
}

// SystemPromptConfig represents system prompt configuration
type SystemPromptConfig struct {
	SystemPrompt string `json:"systemPrompt"`
}

// LoadMCPConfig loads MCP configuration from file
func LoadMCPConfig(configFile string) (*Config, error) {
	v := viper.New()
	
	if configFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("error getting home directory: %v", err)
		}
		
		// Set config name and search paths
		v.SetConfigName(".mcp")
		v.AddConfigPath(homeDir)
		
		// Set config type precedence: yaml first, then json
		v.SetConfigType("yaml")
		
		// Try to read config file
		if err := v.ReadInConfig(); err != nil {
			// If yaml not found, try json
			v.SetConfigType("json")
			if err := v.ReadInConfig(); err != nil {
				// If neither found, return default config
				if _, ok := err.(viper.ConfigFileNotFoundError); ok {
					return &Config{
						MCPServers: make(map[string]MCPServerConfig),
					}, nil
				}
				return nil, fmt.Errorf("error reading config file: %v", err)
			}
		}
	} else {
		// Use specified config file
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("error reading config file: %v", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Validate that allowedTools and excludedTools are mutually exclusive
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadSystemPrompt loads system prompt from file
func LoadSystemPrompt(filePath string) (string, error) {
	if filePath == "" {
		return "", nil
	}

	v := viper.New()
	v.SetConfigFile(filePath)
	
	if err := v.ReadInConfig(); err != nil {
		return "", fmt.Errorf("error reading system prompt file: %v", err)
	}

	systemPrompt := v.GetString("systemPrompt")
	if systemPrompt == "" {
		return "", fmt.Errorf("systemPrompt field not found in config file")
	}

	return systemPrompt, nil
}