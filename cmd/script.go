package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
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
List the files in ${directory} and tell me about them.

The script command supports the same flags as the main command,
which will override any settings in the script file.

Variable substitution:
Variables in the script can be substituted using ${variable} syntax.
Pass variables using --args:variable value syntax:

  mcphost script myscript.sh --args:directory /tmp --args:name "John"

This will replace ${directory} with "/tmp" and ${name} with "John" in the script.`,
	Args: cobra.ExactArgs(1),
	FParseErrWhitelist: cobra.FParseErrWhitelist{
		UnknownFlags: true, // Allow unknown flags for variable substitution
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		scriptFile := args[0]
		
		// Parse custom variables from unknown flags
		variables := parseCustomVariables(cmd)
		
		return runScriptCommand(context.Background(), scriptFile, variables)
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

// parseCustomVariables extracts custom variables from command line arguments
func parseCustomVariables(_ *cobra.Command) map[string]string {
	variables := make(map[string]string)
	
	// Get all arguments passed to the command
	args := os.Args[1:] // Skip program name
	
	// Find the script subcommand position
	scriptPos := -1
	for i, arg := range args {
		if arg == "script" {
			scriptPos = i
			break
		}
	}
	
	if scriptPos == -1 {
		return variables
	}
	
	// Parse arguments after the script file
	scriptFileFound := false
	
	for i := scriptPos + 1; i < len(args); i++ {
		arg := args[i]
		
		// Skip the script file argument (first non-flag after "script")
		if !scriptFileFound && !strings.HasPrefix(arg, "-") {
			scriptFileFound = true
			continue
		}
		
		// Parse custom variables with --args: prefix
		if strings.HasPrefix(arg, "--args:") {
			varName := strings.TrimPrefix(arg, "--args:")
			if varName == "" {
				continue // Skip malformed --args: without name
			}
			
			// Check if we have a value
			if i+1 < len(args) {
				varValue := args[i+1]
				
				// Make sure the next arg isn't a flag
				if !strings.HasPrefix(varValue, "-") {
					variables[varName] = varValue
					i++ // Skip the value
				} else {
					// No value provided, treat as empty string
					variables[varName] = ""
				}
			} else {
				// No value provided, treat as empty string
				variables[varName] = ""
			}
		}
	}
	
	return variables
}


func runScriptCommand(ctx context.Context, scriptFile string, variables map[string]string) error {
	// Parse the script file
	scriptConfig, err := parseScriptFile(scriptFile, variables)
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
	// Set default model if none specified anywhere
	if modelFlag == "" {
		modelFlag = "anthropic:claude-sonnet-4-20250514"
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
func parseScriptFile(filename string, variables map[string]string) (*config.Config, error) {
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
			return parseScriptContent(line+"\n"+readRemainingLines(scanner), variables)
		}
	}

	// Read the rest of the file
	content := readRemainingLines(scanner)
	return parseScriptContent(content, variables)
}

// readRemainingLines reads all remaining lines from a scanner
func readRemainingLines(scanner *bufio.Scanner) string {
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return strings.Join(lines, "\n")
}

// parseScriptContent parses the content to extract YAML frontmatter and prompt
func parseScriptContent(content string, variables map[string]string) (*config.Config, error) {
	// Substitute variables in the content
	content = substituteVariables(content, variables)
	
	lines := strings.Split(content, "\n")

	// Find YAML frontmatter between --- delimiters
	var yamlLines []string
	var promptLines []string
	var inFrontmatter bool
	var foundFrontmatter bool
	var frontmatterEnd int = -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip comment lines (lines starting with #)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		
		// Check for frontmatter start
		if trimmed == "---" && !inFrontmatter {
			// Start of frontmatter
			inFrontmatter = true
			foundFrontmatter = true
			continue
		}
		
		// Check for frontmatter end
		if trimmed == "---" && inFrontmatter {
			// End of frontmatter
			inFrontmatter = false
			frontmatterEnd = i + 1
			continue
		}
		
		// Collect frontmatter lines
		if inFrontmatter {
			yamlLines = append(yamlLines, line)
		}
	}

	// Extract prompt (everything after frontmatter)
	if foundFrontmatter && frontmatterEnd != -1 && frontmatterEnd < len(lines) {
		promptLines = lines[frontmatterEnd:]
	} else if !foundFrontmatter {
		// If no frontmatter found, treat entire content as prompt
		promptLines = lines
		yamlLines = []string{} // Empty YAML
	}

	// Parse YAML frontmatter
	var scriptConfig config.Config
	if len(yamlLines) > 0 {
		yamlContent := strings.Join(yamlLines, "\n")
		if err := yaml.Unmarshal([]byte(yamlContent), &scriptConfig); err != nil {
			return nil, fmt.Errorf("failed to parse YAML frontmatter: %v\nYAML content:\n%s", err, yamlContent)
		}
	}

	// Parse comment-based configuration (lines starting with # key: value)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			// Remove the # and trim whitespace
			configLine := strings.TrimSpace(trimmed[1:])
			
			// Parse key: value format
			if parts := strings.SplitN(configLine, ":", 2); len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				
				// Apply comment-based config
				switch key {
				case "model":
					if scriptConfig.Model == "" {
						scriptConfig.Model = value
					}
				case "max-steps":
					if scriptConfig.MaxSteps == 0 {
						if steps, err := strconv.Atoi(value); err == nil {
							scriptConfig.MaxSteps = steps
						}
					}
				case "message-window":
					if scriptConfig.MessageWindow == 0 {
						if window, err := strconv.Atoi(value); err == nil {
							scriptConfig.MessageWindow = window
						}
					}
				case "debug":
					if !scriptConfig.Debug {
						if debug, err := strconv.ParseBool(value); err == nil {
							scriptConfig.Debug = debug
						}
					}
				case "system-prompt":
					if scriptConfig.SystemPrompt == "" {
						scriptConfig.SystemPrompt = value
					}
				}
			}
		}
	}

	// Set prompt from content after frontmatter
	if len(promptLines) > 0 {
		prompt := strings.Join(promptLines, "\n")
		prompt = strings.TrimSpace(prompt) // Remove leading/trailing whitespace
		if prompt != "" {
			scriptConfig.Prompt = prompt
		}
	}

	return &scriptConfig, nil
}

// substituteVariables replaces ${variable} patterns with their values
func substituteVariables(content string, variables map[string]string) string {
	result := content
	
	// Use regex to find and replace ${variable} patterns
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	
	result = re.ReplaceAllStringFunc(result, func(match string) string {
		// Extract variable name (remove ${ and })
		varName := match[2 : len(match)-1]
		
		// Look up the variable value
		if value, exists := variables[varName]; exists {
			return value
		}
		
		// If variable not found, leave it as is
		return match
	})
	
	return result
}

