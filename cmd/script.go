package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
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
		
		return runScriptCommand(context.Background(), scriptFile, variables, cmd)
	},
}

func init() {
	rootCmd.AddCommand(scriptCmd)
	
	// Add the same flags as the root command, but they will override script settings
	scriptCmd.Flags().StringVar(&systemPromptFile, "system-prompt", "", "system prompt text or path to text file")
	scriptCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "model to use (format: provider:model)")
	scriptCmd.Flags().BoolVar(&debugMode, "debug", false, "enable debug logging")
	scriptCmd.Flags().StringVarP(&promptFlag, "prompt", "p", "", "override the prompt from the script file")
	scriptCmd.Flags().BoolVar(&quietFlag, "quiet", false, "suppress all output")
	scriptCmd.Flags().IntVar(&maxSteps, "max-steps", 0, "maximum number of agent steps (0 for unlimited)")
	scriptCmd.Flags().StringVar(&providerURL, "provider-url", "", "base URL for the provider API (applies to OpenAI, Anthropic, Ollama, and Google)")
	scriptCmd.Flags().StringVar(&providerAPIKey, "provider-api-key", "", "API key for the provider (applies to OpenAI, Anthropic, and Google)")
	
	// Model generation parameters
	scriptCmd.Flags().IntVar(&maxTokens, "max-tokens", 4096, "maximum number of tokens in the response")
	scriptCmd.Flags().Float32Var(&temperature, "temperature", 0.7, "controls randomness in responses (0.0-1.0)")
	scriptCmd.Flags().Float32Var(&topP, "top-p", 0.95, "controls diversity via nucleus sampling (0.0-1.0)")
	scriptCmd.Flags().Int32Var(&topK, "top-k", 40, "controls diversity by limiting top K tokens to sample from")
	scriptCmd.Flags().StringSliceVar(&stopSequences, "stop-sequences", nil, "custom stop sequences (comma-separated)")
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


func runScriptCommand(ctx context.Context, scriptFile string, variables map[string]string, cmd *cobra.Command) error {
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
	originalDebugMode := debugMode
	originalSystemPromptFile := systemPromptFile
	originalProviderAPIKey := providerAPIKey
	originalProviderURL := providerURL
	originalMaxTokens := maxTokens
	originalTemperature := temperature
	originalTopP := topP
	originalTopK := topK
	originalStopSequences := stopSequences

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
	applyScriptFlags(mcpConfig, cmd)

	// Restore original values after execution
	defer func() {
		configFile = originalConfigFile
		promptFlag = originalPromptFlag
		modelFlag = originalModelFlag
		maxSteps = originalMaxSteps
		debugMode = originalDebugMode
		systemPromptFile = originalSystemPromptFile
		providerAPIKey = originalProviderAPIKey
		providerURL = originalProviderURL
		maxTokens = originalMaxTokens
		temperature = originalTemperature
		topP = originalTopP
		topK = originalTopK
		stopSequences = originalStopSequences
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
	if scriptConfig.Debug {
		mcpConfig.Debug = scriptConfig.Debug
	}
	if scriptConfig.SystemPrompt != "" {
		mcpConfig.SystemPrompt = scriptConfig.SystemPrompt
	}
	if scriptConfig.ProviderAPIKey != "" {
		mcpConfig.ProviderAPIKey = scriptConfig.ProviderAPIKey
	}
	if scriptConfig.ProviderURL != "" {
		mcpConfig.ProviderURL = scriptConfig.ProviderURL
	}
	if scriptConfig.Prompt != "" {
		mcpConfig.Prompt = scriptConfig.Prompt
	}
	if scriptConfig.MaxTokens != 0 {
		mcpConfig.MaxTokens = scriptConfig.MaxTokens
	}
	if scriptConfig.Temperature != nil {
		mcpConfig.Temperature = scriptConfig.Temperature
	}
	if scriptConfig.TopP != nil {
		mcpConfig.TopP = scriptConfig.TopP
	}
	if scriptConfig.TopK != nil {
		mcpConfig.TopK = scriptConfig.TopK
	}
	if len(scriptConfig.StopSequences) > 0 {
		mcpConfig.StopSequences = scriptConfig.StopSequences
	}
}

func applyScriptFlags(mcpConfig *config.Config, cmd *cobra.Command) {
	// For scripts, we need to respect: flag > script > config > default
	// We can use cobra's Changed() method to detect if flags were explicitly set
	
	// Only apply script values if the corresponding flag wasn't explicitly set
	if !cmd.Flags().Changed("prompt") && mcpConfig.Prompt != "" {
		promptFlag = mcpConfig.Prompt
	}
	if !cmd.Flags().Changed("model") && mcpConfig.Model != "" {
		modelFlag = mcpConfig.Model
	}
	if !cmd.Flags().Changed("max-steps") && mcpConfig.MaxSteps != 0 {
		maxSteps = mcpConfig.MaxSteps
	}
	if !cmd.Flags().Changed("debug") && mcpConfig.Debug {
		debugMode = mcpConfig.Debug
	}
	if !cmd.Flags().Changed("system-prompt") && mcpConfig.SystemPrompt != "" {
		systemPromptFile = mcpConfig.SystemPrompt
	}
	if !cmd.Flags().Changed("provider-api-key") && mcpConfig.ProviderAPIKey != "" {
		providerAPIKey = mcpConfig.ProviderAPIKey
	}
	if !cmd.Flags().Changed("provider-url") && mcpConfig.ProviderURL != "" {
		providerURL = mcpConfig.ProviderURL
	}
	if !cmd.Flags().Changed("max-tokens") && mcpConfig.MaxTokens != 0 {
		maxTokens = mcpConfig.MaxTokens
	}
	if !cmd.Flags().Changed("temperature") && mcpConfig.Temperature != nil {
		temperature = *mcpConfig.Temperature
	}
	if !cmd.Flags().Changed("top-p") && mcpConfig.TopP != nil {
		topP = *mcpConfig.TopP
	}
	if !cmd.Flags().Changed("top-k") && mcpConfig.TopK != nil {
		topK = *mcpConfig.TopK
	}
	if !cmd.Flags().Changed("stop-sequences") && len(mcpConfig.StopSequences) > 0 {
		stopSequences = mcpConfig.StopSequences
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
	// First, validate that all declared variables are provided
	if err := validateVariables(content, variables); err != nil {
		return nil, err
	}
	
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

// findVariables extracts all unique variable names from ${variable} patterns in content
func findVariables(content string) []string {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	matches := re.FindAllStringSubmatch(content, -1)
	
	seenVars := make(map[string]bool)
	var variables []string
	
	for _, match := range matches {
		if len(match) > 1 {
			varName := match[1]
			if !seenVars[varName] {
				seenVars[varName] = true
				variables = append(variables, varName)
			}
		}
	}
	
	return variables
}

// validateVariables checks that all declared variables in the content are provided
func validateVariables(content string, variables map[string]string) error {
	declaredVars := findVariables(content)
	
	var missingVars []string
	for _, varName := range declaredVars {
		if _, exists := variables[varName]; !exists {
			missingVars = append(missingVars, varName)
		}
	}
	
	if len(missingVars) > 0 {
		return fmt.Errorf("missing required variables: %s\nProvide them using --args:variable value syntax", strings.Join(missingVars, ", "))
	}
	
	return nil
}

// substituteVariables replaces ${variable} patterns with their values
func substituteVariables(content string, variables map[string]string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	
	return re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract variable name (remove ${ and })
		varName := match[2 : len(match)-1]
		
		// Look up the variable value
		if value, exists := variables[varName]; exists {
			return value
		}
		
		// If variable not found, leave it as is
		return match
	})
}

