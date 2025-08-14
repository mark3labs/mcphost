package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// MCPServerConfig represents configuration for an MCP server
type MCPServerConfig struct {
	Type          string            `json:"type"`
	Command       []string          `json:"command,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	URL           string            `json:"url,omitempty"`
	Name          string            `json:"name,omitempty"`    // For builtin servers
	Options       map[string]any    `json:"options,omitempty"` // For builtin servers
	AllowedTools  []string          `json:"allowedTools,omitempty" yaml:"allowedTools,omitempty"`
	ExcludedTools []string          `json:"excludedTools,omitempty" yaml:"excludedTools,omitempty"`

	// Legacy fields for backward compatibility
	Transport string         `json:"transport,omitempty"`
	Args      []string       `json:"args,omitempty"`
	Env       map[string]any `json:"env,omitempty"`
	Headers   []string       `json:"headers,omitempty"`
}

// UnmarshalJSON handles both new and legacy config formats
func (s *MCPServerConfig) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as the new format
	type newFormat struct {
		Type          string            `json:"type"`
		Command       []string          `json:"command,omitempty"`
		Environment   map[string]string `json:"environment,omitempty"`
		URL           string            `json:"url,omitempty"`
		Headers       []string          `json:"headers,omitempty"`
		Name          string            `json:"name,omitempty"`
		Options       map[string]any    `json:"options,omitempty"`
		AllowedTools  []string          `json:"allowedTools,omitempty" yaml:"allowedTools,omitempty"`
		ExcludedTools []string          `json:"excludedTools,omitempty" yaml:"excludedTools,omitempty"`
	}

	// Also try legacy format
	type legacyFormat struct {
		Transport     string         `json:"transport,omitempty"`
		Command       string         `json:"command,omitempty"`
		Args          []string       `json:"args,omitempty"`
		Env           map[string]any `json:"env,omitempty"`
		URL           string         `json:"url,omitempty"`
		Headers       []string       `json:"headers,omitempty"`
		AllowedTools  []string       `json:"allowedTools,omitempty" yaml:"allowedTools,omitempty"`
		ExcludedTools []string       `json:"excludedTools,omitempty" yaml:"excludedTools,omitempty"`
	}

	// Try new format first
	var newConfig newFormat
	if err := json.Unmarshal(data, &newConfig); err == nil && newConfig.Type != "" {
		s.Type = newConfig.Type
		s.Command = newConfig.Command
		s.Environment = newConfig.Environment
		s.URL = newConfig.URL
		s.Headers = newConfig.Headers
		s.Name = newConfig.Name
		s.Options = newConfig.Options
		s.AllowedTools = newConfig.AllowedTools
		s.ExcludedTools = newConfig.ExcludedTools
		return nil
	}

	// Fall back to legacy format
	var legacyConfig legacyFormat
	if err := json.Unmarshal(data, &legacyConfig); err != nil {
		return err
	}

	// Convert legacy format to new format
	s.Transport = legacyConfig.Transport
	if legacyConfig.Command != "" {
		s.Command = append([]string{legacyConfig.Command}, legacyConfig.Args...)
	}
	s.Args = legacyConfig.Args
	s.Env = legacyConfig.Env
	s.URL = legacyConfig.URL
	s.Headers = legacyConfig.Headers
	s.AllowedTools = legacyConfig.AllowedTools
	s.ExcludedTools = legacyConfig.ExcludedTools

	// Infer type from legacy format for better compatibility
	// Only set Type when it doesn't change existing transport behavior
	if legacyConfig.Command != "" {
		s.Type = "local" // This maps to "stdio" which matches legacy behavior
	}
	// Don't set Type for URL-only configs to preserve legacy "sse" behavior
	// The URL will be handled by the legacy fallback logic in GetTransportType()

	return nil
}

type AdaptiveColor struct {
	Light string `json:"light,omitempty" yaml:"light,omitempty"`
	Dark  string `json:"dark,omitempty" yaml:"dark,omitempty"`
}

type Theme struct {
	Primary     AdaptiveColor `json:"primary" yaml:"primary"`
	Secondary   AdaptiveColor `json:"secondary" yaml:"secondary"`
	Success     AdaptiveColor `json:"success" yaml:"success"`
	Warning     AdaptiveColor `json:"warning" yaml:"warning"`
	Error       AdaptiveColor `json:"error" yaml:"error"`
	Info        AdaptiveColor `json:"info" yaml:"info"`
	Text        AdaptiveColor `json:"text" yaml:"text"`
	Muted       AdaptiveColor `json:"muted" yaml:"muted"`
	VeryMuted   AdaptiveColor `json:"very-muted" yaml:"very-muted"`
	Background  AdaptiveColor `json:"background" yaml:"background"`
	Border      AdaptiveColor `json:"border" yaml:"border"`
	MutedBorder AdaptiveColor `json:"muted-border" yaml:"muted-border"`
	System      AdaptiveColor `json:"system" yaml:"system"`
	Tool        AdaptiveColor `json:"tool" yaml:"tool"`
	Accent      AdaptiveColor `json:"accent" yaml:"accent"`
	Highlight   AdaptiveColor `json:"highlight" yaml:"highlight"`
}

type MarkdownTheme struct {
	Text    AdaptiveColor `json:"text" yaml:"text"`
	Muted   AdaptiveColor `json:"muted" yaml:"muted"`
	Heading AdaptiveColor `json:"heading" yaml:"heading"`
	Emph    AdaptiveColor `json:"emph" yaml:"emph"`
	Strong  AdaptiveColor `json:"strong" yaml:"strong"`
	Link    AdaptiveColor `json:"link" yaml:"link"`
	Code    AdaptiveColor `json:"code" yaml:"code"`
	Error   AdaptiveColor `json:"error" yaml:"error"`
	Keyword AdaptiveColor `json:"keyword" yaml:"keyword"`
	String  AdaptiveColor `json:"string" yaml:"string"`
	Number  AdaptiveColor `json:"number" yaml:"number"`
	Comment AdaptiveColor `json:"comment" yaml:"comment"`
}

// Config represents the application configuration
type Config struct {
	MCPServers     map[string]MCPServerConfig `json:"mcpServers" yaml:"mcpServers"`
	Model          string                     `json:"model,omitempty" yaml:"model,omitempty"`
	MaxSteps       int                        `json:"max-steps,omitempty" yaml:"max-steps,omitempty"`
	Debug          bool                       `json:"debug,omitempty" yaml:"debug,omitempty"`
	Compact        bool                       `json:"compact,omitempty" yaml:"compact,omitempty"`
	SystemPrompt   string                     `json:"system-prompt,omitempty" yaml:"system-prompt,omitempty"`
	ProviderAPIKey string                     `json:"provider-api-key,omitempty" yaml:"provider-api-key,omitempty"`
	ProviderURL    string                     `json:"provider-url,omitempty" yaml:"provider-url,omitempty"`
	Prompt         string                     `json:"prompt,omitempty" yaml:"prompt,omitempty"`
	NoExit         bool                       `json:"no-exit,omitempty" yaml:"no-exit,omitempty"`
	Stream         *bool                      `json:"stream,omitempty" yaml:"stream,omitempty"`
	Theme          any                        `json:"theme" yaml:"theme"`
	MarkdownTheme  any                        `json:"markdown-theme" yaml:"markdown-theme"`

	// Model generation parameters
	MaxTokens     int      `json:"max-tokens,omitempty" yaml:"max-tokens,omitempty"`
	Temperature   *float32 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	TopP          *float32 `json:"top-p,omitempty" yaml:"top-p,omitempty"`
	TopK          *int32   `json:"top-k,omitempty" yaml:"top-k,omitempty"`
	StopSequences []string `json:"stop-sequences,omitempty" yaml:"stop-sequences,omitempty"`

	// TLS configuration
	TLSSkipVerify bool `json:"tls-skip-verify,omitempty" yaml:"tls-skip-verify,omitempty"`
}

// GetTransportType returns the transport type for the server config
func (s *MCPServerConfig) GetTransportType() string {
	// Legacy format support - check explicit transport first
	if s.Transport != "" {
		return s.Transport
	}

	// New simplified format
	if s.Type != "" {
		switch s.Type {
		case "local":
			return "stdio"
		case "remote":
			return "streamable"
		case "builtin":
			return "inprocess"
		default:
			return s.Type
		}
	}

	// Backward compatibility: infer transport type
	if len(s.Command) > 0 {
		return "stdio"
	}
	if s.URL != "" {
		return "sse"
	}
	return "stdio" // default
}

// Validate validates the configuration
func (c *Config) Validate() error {
	for serverName, serverConfig := range c.MCPServers {
		if len(serverConfig.AllowedTools) > 0 && len(serverConfig.ExcludedTools) > 0 {
			return fmt.Errorf("server %s: allowedTools and excludedTools are mutually exclusive", serverName)
		}

		transport := serverConfig.GetTransportType()
		switch transport {
		case "stdio":
			// Check both new and legacy command formats
			if len(serverConfig.Command) == 0 && serverConfig.Transport == "" {
				return fmt.Errorf("server %s: command is required for stdio transport", serverName)
			}
		case "sse", "streamable":
			if serverConfig.URL == "" {
				return fmt.Errorf("server %s: url is required for %s transport", serverName, transport)
			}
		case "inprocess":
			if serverConfig.Name == "" {
				return fmt.Errorf("server %s: name is required for builtin servers", serverName)
			}
		default:
			return fmt.Errorf("server %s: unsupported transport type '%s'. Supported types: stdio, sse, streamable, inprocess", serverName, transport)
		}
	}
	return nil
}

// LoadSystemPrompt loads system prompt from file or returns the string directly
func LoadSystemPrompt(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	// Check if input is a file that exists
	if _, err := os.Stat(input); err == nil {
		// Read the entire file as plain text
		content, err := os.ReadFile(input)
		if err != nil {
			return "", fmt.Errorf("error reading system prompt file: %v", err)
		}
		return strings.TrimSpace(string(content)), nil
	}

	// Treat as direct string
	return input, nil
}

// EnsureConfigExists checks if a config file exists and creates a default one if not
func EnsureConfigExists() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting home directory: %v", err)
	}

	// Check for existing config files (new format first, then legacy)
	configNames := []string{".mcphost", ".mcp"}
	configTypes := []string{"yml", "yaml", "json"}

	for _, configName := range configNames {
		for _, configType := range configTypes {
			configPath := filepath.Join(homeDir, configName+"."+configType)
			if _, err := os.Stat(configPath); err == nil {
				// Config file exists, no need to create
				return nil
			}
		}
	}

	// No config file found, create default
	return createDefaultConfig(homeDir)
}

// createDefaultConfig creates a default .mcphost.yml file in the user's home directory
func createDefaultConfig(homeDir string) error {
	configPath := filepath.Join(homeDir, ".mcphost.yml")

	// Create the file
	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("error creating config file: %v", err)
	}
	defer file.Close()

	// Write a comprehensive YAML template with examples
	content := `# MCPHost Configuration File
# All command-line flags can be configured here
# This demonstrates the simplified local/remote/builtin server configuration

# MCP Servers configuration
# Add your MCP servers here
# Examples for different server types:
# mcpServers:
#   # Local MCP servers - run commands locally via stdio transport
#   filesystem-local:
#     type: "local"
#     command: ["npx", "@modelcontextprotocol/server-filesystem", "/tmp"]
#     environment:
#       DEBUG: "true"
#       LOG_LEVEL: "info"
#   
#   sqlite:
#     type: "local" 
#     command: ["uvx", "mcp-server-sqlite", "--db-path", "/tmp/example.db"]
#     environment:
#       SQLITE_DEBUG: "1"
#   
#   # Builtin MCP servers - run in-process for optimal performance
#   filesystem-builtin:
#     type: "builtin"
#     name: "fs"
#     options:
#       allowed_directories: ["/tmp", "/home/user/documents"]
#     allowedTools: ["read_file", "write_file", "list_directory"]
#   
#   # Minimal builtin server - defaults to current working directory
#   filesystem-cwd:
#     type: "builtin"
#     name: "fs"
#   
#   # Bash server for shell commands
#   bash:
#     type: "builtin"
#     name: "bash"
#   
#   # Todo server for task management
#   todo:
#     type: "builtin"
#     name: "todo"
#   
#   # Fetch server for web content
#   fetch:
#     type: "builtin"
#     name: "fetch"
#   
#   # Remote MCP servers - connect via StreamableHTTP transport
#   # Optional 'headers' field can be used for authentication and custom headers
#   websearch:
#     type: "remote"
#     url: "https://api.example.com/mcp"
#   
#   weather:
#     type: "remote"
#     url: "https://weather-mcp.example.com"
#   
#   # Legacy format still supported for backward compatibility:
#   # legacy-server:
#   #   command: npx
#   #   args: ["@modelcontextprotocol/server-filesystem", "/path"]
#   #   env:
#   #     MY_VAR: "value"

mcpServers:

# Application settings (all optional)
# model: "anthropic:claude-sonnet-4-20250514"  # Default model to use
# max-steps: 10                                # Maximum agent steps (0 for unlimited)
# debug: false                                 # Enable debug logging
# system-prompt: "/path/to/system-prompt.txt" # System prompt text file

# Model generation parameters (all optional)
# max-tokens: 4096                             # Maximum tokens in response
# temperature: 0.7                             # Randomness (0.0-1.0)
# top-p: 0.95                                  # Nucleus sampling (0.0-1.0)
# top-k: 40                                    # Top K sampling
# stop-sequences: ["Human:", "Assistant:"]     # Custom stop sequences

# API Configuration (can also use environment variables)
# provider-api-key: "your-api-key"         # API key for OpenAI, Anthropic, or Google
# provider-url: "https://api.openai.com/v1" # Base URL for OpenAI, Anthropic, or Ollama
`

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("error writing config content: %v", err)
	}

	return nil
}

func FilepathOr[T any](key string, value *T) error {
	var field any
	err := viper.UnmarshalKey(key, &field)
	if err != nil {
		value = nil
		return err
	}
	switch f := field.(type) {
	case string:
		{
			absPath := f
			if strings.HasPrefix(absPath, "~/") {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				filepath.Join(home, absPath[2:])
			}
			if !filepath.IsAbs(absPath) {
				// base := GetConfigPath()
				base := configPath
				if base == "" {
					fmt.Fprintf(os.Stderr, "unable to build relative path to config.")
					os.Exit(1)
				}
				absPath = filepath.Join(filepath.Dir(base), absPath)
			}
			b, err := os.ReadFile(absPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%q", err)
				os.Exit(1)
			}
			if filepath.Ext(absPath) == ".json" {
				return json.Unmarshal(b, value)
			}

			if filepath.Ext(absPath) == ".yaml" {
				return yaml.Unmarshal(b, value)
			}
		}
	case map[string]any:
		return viper.UnmarshalKey(key, value)
	default:
		return fmt.Errorf("invalid type for field %q", key)
	}
	return nil
}

var configPath string

func SetConfigPath(path string) {
	configPath = path
}
