package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcphost/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var scriptCmd = &cobra.Command{
	Use:   "script <script-file>",
	Short: "Execute a script file with YAML frontmatter configuration",
	Long: `Execute a script file that contains YAML frontmatter with configuration
and a prompt. The script file can contain MCP server configurations,
model settings, and other options.

Example script file:
---
model: "anthropic:claude-sonnet-4-20250514"
max-steps: 10
mcp-servers:
  filesystem:
    command: "npx"
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
---
prompt: "List the files in the current directory"

The script command supports the same flags as the main command,
which will override any settings in the script file.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scriptFile := args[0]
		return runScriptCommand(context.Background(), scriptFile)
	},
}

func init() {
	rootCmd.AddCommand(scriptCmd)
	
	// Add the same flags as the root command, but they will override script settings
	scriptCmd.Flags().StringVar(&systemPromptFile, "system-prompt", "", "system prompt text or path to system prompt json file")
	scriptCmd.Flags().IntVar(&messageWindow, "message-window", 40, "number of messages to keep in context")
	scriptCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "model to use (format: provider:model)")
	scriptCmd.Flags().BoolVar(&debugMode, "debug", false, "enable debug logging")
	scriptCmd.Flags().StringVarP(&promptFlag, "prompt", "p", "", "override the prompt from the script file")
	scriptCmd.Flags().BoolVar(&quietFlag, "quiet", false, "suppress all output")
	scriptCmd.Flags().IntVar(&maxSteps, "max-steps", 0, "maximum number of agent steps (0 for unlimited)")
	scriptCmd.Flags().StringVar(&openaiBaseURL, "openai-url", "", "base URL for OpenAI API")
	scriptCmd.Flags().StringVar(&anthropicBaseURL, "anthropic-url", "", "base URL for Anthropic API")
	scriptCmd.Flags().StringVar(&openaiAPIKey, "openai-api-key", "", "OpenAI API key")
	scriptCmd.Flags().StringVar(&anthropicAPIKey, "anthropic-api-key", "", "Anthropic API key")
	scriptCmd.Flags().StringVar(&googleAPIKey, "google-api-key", "", "Google (Gemini) API key")
}

func runScriptCommand(ctx context.Context, scriptFile string) error {
	// Parse the script file
	scriptConfig, err := parseScriptFile(scriptFile)
	if err != nil {
		return fmt.Errorf("failed to parse script file: %v", err)
	}

	// Store original flag values
	originalConfigFile := configFile
	originalPromptFlag := promptFlag
	originalModelFlag := modelFlag
	originalMaxSteps := maxSteps
	originalMessageWindow := messageWindow
	originalDebugMode := debugMode
	originalSystemPromptFile := systemPromptFile
	originalOpenAIAPIKey := openaiAPIKey
	originalAnthropicAPIKey := anthropicAPIKey
	originalGoogleAPIKey := googleAPIKey
	originalOpenAIURL := openaiBaseURL
	originalAnthropicURL := anthropicBaseURL

	// Create config from script or load normal config
	var mcpConfig *config.Config
	if len(scriptConfig.MCPServers) > 0 {
		// Use servers from script
		mcpConfig = scriptConfig
	} else {
		// Fall back to normal config loading
		mcpConfig, err = config.LoadMCPConfig(configFile)
		if err != nil {
			return fmt.Errorf("failed to load MCP config: %v", err)
		}
		// Merge script config values into loaded config
		mergeScriptConfig(mcpConfig, scriptConfig)
	}

	// Override the global config for normal mode
	scriptMCPConfig = mcpConfig

	// Apply script configuration to global flags (only if not overridden by command flags)
	applyScriptFlags(mcpConfig)

	// Restore original values after execution
	defer func() {
		configFile = originalConfigFile
		promptFlag = originalPromptFlag
		modelFlag = originalModelFlag
		maxSteps = originalMaxSteps
		messageWindow = originalMessageWindow
		debugMode = originalDebugMode
		systemPromptFile = originalSystemPromptFile
		openaiAPIKey = originalOpenAIAPIKey
		anthropicAPIKey = originalAnthropicAPIKey
		googleAPIKey = originalGoogleAPIKey
		openaiBaseURL = originalOpenAIURL
		anthropicBaseURL = originalAnthropicURL
		scriptMCPConfig = nil
	}()

	// Now run the normal execution path which will use our overridden config
	return runNormalMode(ctx)
}

func mergeScriptConfig(mcpConfig *config.Config, scriptConfig *config.Config) {
	if scriptConfig.Model != "" {
		mcpConfig.Model = scriptConfig.Model
	}
	if scriptConfig.MaxSteps != 0 {
		mcpConfig.MaxSteps = scriptConfig.MaxSteps
	}
	if scriptConfig.MessageWindow != 0 {
		mcpConfig.MessageWindow = scriptConfig.MessageWindow
	}
	if scriptConfig.Debug {
		mcpConfig.Debug = scriptConfig.Debug
	}
	if scriptConfig.SystemPrompt != "" {
		mcpConfig.SystemPrompt = scriptConfig.SystemPrompt
	}
	if scriptConfig.OpenAIAPIKey != "" {
		mcpConfig.OpenAIAPIKey = scriptConfig.OpenAIAPIKey
	}
	if scriptConfig.AnthropicAPIKey != "" {
		mcpConfig.AnthropicAPIKey = scriptConfig.AnthropicAPIKey
	}
	if scriptConfig.GoogleAPIKey != "" {
		mcpConfig.GoogleAPIKey = scriptConfig.GoogleAPIKey
	}
	if scriptConfig.OpenAIURL != "" {
		mcpConfig.OpenAIURL = scriptConfig.OpenAIURL
	}
	if scriptConfig.AnthropicURL != "" {
		mcpConfig.AnthropicURL = scriptConfig.AnthropicURL
	}
	if scriptConfig.Prompt != "" {
		mcpConfig.Prompt = scriptConfig.Prompt
	}
}

func applyScriptFlags(mcpConfig *config.Config) {
	// Only apply script values if the corresponding flag wasn't explicitly set
	if promptFlag == "" && mcpConfig.Prompt != "" {
		promptFlag = mcpConfig.Prompt
	}
	if modelFlag == "" && mcpConfig.Model != "" {
		modelFlag = mcpConfig.Model
	}
	if maxSteps == 0 && mcpConfig.MaxSteps != 0 {
		maxSteps = mcpConfig.MaxSteps
	}
	if messageWindow == 40 && mcpConfig.MessageWindow != 0 { // 40 is the default
		messageWindow = mcpConfig.MessageWindow
	}
	if !debugMode && mcpConfig.Debug {
		debugMode = mcpConfig.Debug
	}
	if systemPromptFile == "" && mcpConfig.SystemPrompt != "" {
		systemPromptFile = mcpConfig.SystemPrompt
	}
	if openaiAPIKey == "" && mcpConfig.OpenAIAPIKey != "" {
		openaiAPIKey = mcpConfig.OpenAIAPIKey
	}
	if anthropicAPIKey == "" && mcpConfig.AnthropicAPIKey != "" {
		anthropicAPIKey = mcpConfig.AnthropicAPIKey
	}
	if googleAPIKey == "" && mcpConfig.GoogleAPIKey != "" {
		googleAPIKey = mcpConfig.GoogleAPIKey
	}
	if openaiBaseURL == "" && mcpConfig.OpenAIURL != "" {
		openaiBaseURL = mcpConfig.OpenAIURL
	}
	if anthropicBaseURL == "" && mcpConfig.AnthropicURL != "" {
		anthropicBaseURL = mcpConfig.AnthropicURL
	}
}

// parseScriptFile parses a script file with YAML frontmatter and returns config
func parseScriptFile(filename string) (*config.Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Skip shebang line if present
	if scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#!") {
			// If it's not a shebang, we need to process this line
			return parseScriptContent(line + "\n" + readRemainingLines(scanner))
		}
	}

	// Read the rest of the file
	content := readRemainingLines(scanner)
	return parseScriptContent(content)
}

// readRemainingLines reads all remaining lines from a scanner
func readRemainingLines(scanner *bufio.Scanner) string {
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return strings.Join(lines, "\n")
}

// parseScriptContent parses the content to extract YAML frontmatter
func parseScriptContent(content string) (*config.Config, error) {
	lines := strings.Split(content, "\n")

	// Find YAML frontmatter
	var yamlLines []string

	for _, line := range lines {
		yamlLines = append(yamlLines, line)
	}

	// Parse YAML
	yamlContent := strings.Join(yamlLines, "\n")
	var scriptConfig config.Config
	if err := yaml.Unmarshal([]byte(yamlContent), &scriptConfig); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
	}

	return &scriptConfig, nil
}

