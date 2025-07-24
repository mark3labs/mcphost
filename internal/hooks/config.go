package hooks

import (
	"encoding/json"
	"fmt"
	"github.com/mark3labs/mcphost/internal/config"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

// HookConfig represents the complete hooks configuration
type HookConfig struct {
	Hooks map[HookEvent][]HookMatcher `yaml:"hooks" json:"hooks"`
}

// HookMatcher matches specific tools and defines hooks to execute
type HookMatcher struct {
	Matcher string      `yaml:"matcher,omitempty" json:"matcher,omitempty"`
	Merge   string      `yaml:"_merge,omitempty" json:"_merge,omitempty"`
	Hooks   []HookEntry `yaml:"hooks" json:"hooks"`
}

// HookEntry defines a single hook command
type HookEntry struct {
	Type    string `yaml:"type" json:"type"`
	Command string `yaml:"command" json:"command"`
	Timeout int    `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// LoadHooksConfig loads and merges hook configurations from multiple sources
func LoadHooksConfig(customPaths ...string) (*HookConfig, error) {
	// Get config directory following XDG Base Directory specification
	configDir := getConfigDir()

	// Define search paths in order of precedence (lowest to highest)
	searchPaths := []string{
		filepath.Join(configDir, "mcphost", "hooks.json"),
		filepath.Join(configDir, "mcphost", "hooks.yml"),
		".mcphost/hooks.json",
		".mcphost/hooks.yml",
	}

	// Add custom paths with highest precedence
	searchPaths = append(searchPaths, customPaths...)

	merged := &HookConfig{
		Hooks: make(map[HookEvent][]HookMatcher),
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}

		// Apply environment substitution
		envSubstituter := &config.EnvSubstituter{}
		substituted, err := envSubstituter.SubstituteEnvVars(string(content))
		if err != nil {
			return nil, fmt.Errorf("substituting env vars in %s: %w", path, err)
		}

		// Parse configuration
		var cfg HookConfig
		if filepath.Ext(path) == ".json" {
			err = json.Unmarshal([]byte(substituted), &cfg)
		} else {
			err = yaml.Unmarshal([]byte(substituted), &cfg)
		}
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}

		// Merge configurations
		mergeHookConfigs(merged, &cfg)
	}

	return merged, nil
}

// getConfigDir returns the configuration directory following XDG Base Directory specification
func getConfigDir() string {
	// Try XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return xdgConfig
	}

	// Fall back to ~/.config
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config")
	}

	// Last resort: current directory
	return "."
}

// mergeHookConfigs merges source hooks into destination
func mergeHookConfigs(dst, src *HookConfig) {
	for event, matchers := range src.Hooks {
		if dst.Hooks[event] == nil {
			dst.Hooks[event] = matchers
			continue
		}

		// Handle merge strategies
		for _, srcMatcher := range matchers {
			if srcMatcher.Merge == "replace" {
				// Replace all matchers for this event
				dst.Hooks[event] = []HookMatcher{srcMatcher}
			} else {
				// Append or update existing matcher
				found := false
				for i, dstMatcher := range dst.Hooks[event] {
					if dstMatcher.Matcher == srcMatcher.Matcher {
						dst.Hooks[event][i] = srcMatcher
						found = true
						break
					}
				}
				if !found {
					dst.Hooks[event] = append(dst.Hooks[event], srcMatcher)
				}
			}
		}
	}
}
