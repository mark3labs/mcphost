package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// MergeConfigs merges script frontmatter config with base config, allowing scripts
// to override MCP server configurations. The script config takes precedence over
// the base config for any fields that are specified.
func MergeConfigs(baseConfig *Config, scriptConfig *Config) *Config {
	merged := *baseConfig // Copy base config

	// Override MCP servers if script provides them
	if len(scriptConfig.MCPServers) > 0 {
		merged.MCPServers = scriptConfig.MCPServers
	}

	// Add other merge logic as needed for future config fields
	return &merged
}

// LoadAndValidateConfig loads configuration from viper, fixes environment variable
// casing issues, and validates the configuration. Returns an error if loading or
// validation fails.
func LoadAndValidateConfig() (*Config, error) {
	config := &Config{
		MCPServers: make(map[string]MCPServerConfig),
	}
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	// Fix environment variable case sensitivity issue
	// Viper lowercases all keys, but we need to preserve the original case for environment variables
	fixEnvironmentCase(config)

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	return config, nil
}

// fixEnvironmentCase fixes the case of environment variable keys that were lowercased by Viper
func fixEnvironmentCase(config *Config) {
	// Get the raw config data from viper
	rawConfig := viper.AllSettings()

	// Check if we have mcpServers in the raw config
	if mcpServersRaw, ok := rawConfig["mcpservers"]; ok {
		if mcpServersMap, ok := mcpServersRaw.(map[string]interface{}); ok {
			// Iterate through each server
			for serverName, serverDataRaw := range mcpServersMap {
				if serverData, ok := serverDataRaw.(map[string]interface{}); ok {
					// Check if this server has an environment field
					if _, hasEnv := serverData["environment"]; hasEnv {
						// Get the server config from our parsed config
						if serverConfig, exists := config.MCPServers[serverName]; exists {
							// Create a new environment map with proper casing
							newEnv := make(map[string]string)

							// For each environment variable, check if it should be uppercase
							for key, value := range serverConfig.Environment {
								// Convert to uppercase if it looks like an environment variable
								// (contains underscore or is all uppercase in typical usage)
								upperKey := strings.ToUpper(key)
								if strings.Contains(key, "_") || key == strings.ToLower(upperKey) {
									newEnv[upperKey] = value
								} else {
									newEnv[key] = value
								}
							}

							// Update the server config with the fixed environment map
							serverConfig.Environment = newEnv
							config.MCPServers[serverName] = serverConfig
						}
					}
				}
			}
		}
	}
}
