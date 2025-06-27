package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// MergeConfigs merges script frontmatter config with base config
func MergeConfigs(baseConfig *Config, scriptConfig *Config) *Config {
	merged := *baseConfig // Copy base config

	// Override MCP servers if script provides them
	if len(scriptConfig.MCPServers) > 0 {
		merged.MCPServers = scriptConfig.MCPServers
	}

	// Add other merge logic as needed for future config fields
	return &merged
}

// LoadAndValidateConfig loads config from viper and validates it
func LoadAndValidateConfig() (*Config, error) {
	config := &Config{
		MCPServers: make(map[string]MCPServerConfig),
	}
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	return config, nil
}
