package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcphost/internal/agent"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
mcpServers:
  filesystem:
    type: "local"
    command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "${directory:-/tmp}"]
---
Hello ${name:-World}! List the files in ${directory:-/tmp} and tell me about them.

The script command supports the same flags as the main command,
which will override any settings in the script file.

Variable substitution:
Variables in the script can be substituted using ${variable} syntax.
Variables can have default values using ${variable:-default} syntax.
Pass variables using --args:variable value syntax:

  mcphost script myscript.sh --args:directory /tmp --args:name "John"

This will replace ${directory} with "/tmp" and ${name} with "John" in the script.
Variables with defaults (${var:-default}) are optional and use the default if not provided.`,
	Args: cobra.ExactArgs(1),
	FParseErrWhitelist: cobra.FParseErrWhitelist{
		UnknownFlags: true, // Allow unknown flags for variable substitution
	},
	PreRun: func(cmd *cobra.Command, args []string) {
		// Override config with frontmatter values from the script file
		scriptFile := args[0]
		variables := parseCustomVariables(cmd)
		overrideConfigWithFrontmatter(scriptFile, variables, cmd)
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
}

// overrideConfigWithFrontmatter parses the script file and overrides viper config with frontmatter values
// This is the only purpose of this function - to apply frontmatter configuration to viper
func overrideConfigWithFrontmatter(scriptFile string, variables map[string]string, cmd *cobra.Command) {
	// Parse the script file to get frontmatter configuration
	scriptConfig, err := parseScriptFile(scriptFile, variables)
	if err != nil {
		// If we can't parse the script file, just continue with existing config
		// The error will be handled again in runScriptCommand
		return
	}

	// Override viper values with frontmatter values (only if flags weren't explicitly set)
	// Check both local flags and persistent flags since script inherits from root
	flagChanged := func(name string) bool {
		return cmd.Flags().Changed(name) || rootCmd.PersistentFlags().Changed(name)
	}

	if scriptConfig.Model != "" && !flagChanged("model") {
		viper.Set("model", scriptConfig.Model)
	}
	if scriptConfig.MaxSteps != 0 && !flagChanged("max-steps") {
		viper.Set("max-steps", scriptConfig.MaxSteps)
	}
	if scriptConfig.Debug && !flagChanged("debug") {
		viper.Set("debug", scriptConfig.Debug)
	}
	if scriptConfig.SystemPrompt != "" && !flagChanged("system-prompt") {
		viper.Set("system-prompt", scriptConfig.SystemPrompt)
	}
	if scriptConfig.ProviderAPIKey != "" && !flagChanged("provider-api-key") {
		viper.Set("provider-api-key", scriptConfig.ProviderAPIKey)
	}
	if scriptConfig.ProviderURL != "" && !flagChanged("provider-url") {
		viper.Set("provider-url", scriptConfig.ProviderURL)
	}
	if scriptConfig.MaxTokens != 0 && !flagChanged("max-tokens") {
		viper.Set("max-tokens", scriptConfig.MaxTokens)
	}
	if scriptConfig.Temperature != nil && !flagChanged("temperature") {
		viper.Set("temperature", *scriptConfig.Temperature)
	}
	if scriptConfig.TopP != nil && !flagChanged("top-p") {
		viper.Set("top-p", *scriptConfig.TopP)
	}
	if scriptConfig.TopK != nil && !flagChanged("top-k") {
		viper.Set("top-k", *scriptConfig.TopK)
	}
	if len(scriptConfig.StopSequences) > 0 && !flagChanged("stop-sequences") {
		viper.Set("stop-sequences", scriptConfig.StopSequences)
	}
	if scriptConfig.NoExit && !flagChanged("no-exit") {
		// Set the global noExitFlag variable if it wasn't explicitly set via command line
		noExitFlag = scriptConfig.NoExit
	}
	if scriptConfig.Stream != nil && !flagChanged("stream") {
		viper.Set("stream", *scriptConfig.Stream)
	}
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

func runScriptCommand(ctx context.Context, scriptFile string, variables map[string]string, _ *cobra.Command) error {
	// Parse the script file to get MCP servers and prompt
	scriptConfig, err := parseScriptFile(scriptFile, variables)
	if err != nil {
		return fmt.Errorf("failed to parse script file: %v", err)
	}

	// Get MCP config - use script servers if available, otherwise use global viper config
	var mcpConfig *config.Config
	if len(scriptConfig.MCPServers) > 0 {
		// Load base config and merge with script config
		baseConfig, err := config.LoadAndValidateConfig()
		if err != nil {
			return fmt.Errorf("failed to load base config: %v", err)
		}
		mcpConfig = config.MergeConfigs(baseConfig, scriptConfig)
	} else {
		// Use the new config loader
		var err error
		mcpConfig, err = config.LoadAndValidateConfig()
		if err != nil {
			return fmt.Errorf("failed to load MCP config: %v", err)
		}
	}

	// Get final prompt - prioritize command line flag, then script content
	finalPrompt := viper.GetString("prompt")
	if finalPrompt == "" && scriptConfig.Prompt != "" {
		finalPrompt = scriptConfig.Prompt
	}

	// Get final no-exit setting - prioritize command line flag, then script config
	finalNoExit := noExitFlag || scriptConfig.NoExit

	// Validate that --no-exit is only used when there's a prompt
	if finalNoExit && finalPrompt == "" {
		return fmt.Errorf("--no-exit flag can only be used when there's a prompt (either from script content or --prompt flag)")
	}

	// Run the script using the unified agentic loop
	return runScriptMode(ctx, mcpConfig, finalPrompt, finalNoExit)
}

// mergeScriptConfig and setScriptValuesInViper functions removed
// Configuration override is now handled in overrideConfigWithFrontmatter in the PreRun hook

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
	// STEP 1: Apply environment variable substitution FIRST
	envSubstituter := &config.EnvSubstituter{}
	processedContent, err := envSubstituter.SubstituteEnvVars(content)
	if err != nil {
		return nil, fmt.Errorf("script env substitution failed: %v", err)
	}

	// STEP 2: Validate that all declared script variables are provided
	if err := validateVariables(processedContent, variables); err != nil {
		return nil, err
	}

	// STEP 3: Apply script args substitution
	argsSubstituter := config.NewArgsSubstituter(variables)
	content, err = argsSubstituter.SubstituteArgs(processedContent)
	if err != nil {
		return nil, fmt.Errorf("script args substitution failed: %v", err)
	}

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

// Variable represents a script variable with optional default value
type Variable struct {
	Name         string
	DefaultValue string
	HasDefault   bool
}

// findVariables extracts all unique variable names from ${variable} patterns in content
// Maintains backward compatibility by returning just variable names
func findVariables(content string) []string {
	variables := findVariablesWithDefaults(content)
	var names []string
	for _, v := range variables {
		names = append(names, v.Name)
	}
	return names
}

// findVariablesWithDefaults extracts all unique variables with their default values
// Supports both ${variable} and ${variable:-default} syntax
func findVariablesWithDefaults(content string) []Variable {
	// Pattern matches:
	// ${varname} - simple variable
	// ${varname:-default} - variable with default value
	re := regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)
	matches := re.FindAllStringSubmatch(content, -1)

	seenVars := make(map[string]bool)
	var variables []Variable

	for _, match := range matches {
		if len(match) >= 2 {
			varName := match[1]
			if !seenVars[varName] {
				seenVars[varName] = true

				// Check if the original match contains the :- pattern
				hasDefault := strings.Contains(match[0], ":-")

				variable := Variable{
					Name:       varName,
					HasDefault: hasDefault,
				}

				if hasDefault && len(match) >= 3 {
					variable.DefaultValue = match[2] // Can be empty string
				}

				variables = append(variables, variable)
			}
		}
	}

	return variables
}

// validateVariables checks that all declared variables in the content are provided
// Variables with default values are not required
func validateVariables(content string, variables map[string]string) error {
	declaredVars := findVariablesWithDefaults(content)

	var missingVars []string
	for _, variable := range declaredVars {
		if _, exists := variables[variable.Name]; !exists && !variable.HasDefault {
			missingVars = append(missingVars, variable.Name)
		}
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("missing required variables: %s\nProvide them using --args:variable value syntax", strings.Join(missingVars, ", "))
	}

	return nil
}

// substituteVariables replaces ${variable} and ${variable:-default} patterns with their values
// This function is kept for backward compatibility but now uses the shared ArgsSubstituter
func substituteVariables(content string, variables map[string]string) string {
	substituter := config.NewArgsSubstituter(variables)
	result, err := substituter.SubstituteArgs(content)
	if err != nil {
		// For backward compatibility, if there's an error, return the original content
		// This maintains the existing behavior where missing variables were left as-is
		return content
	}
	return result
}

// runScriptMode executes the script using the unified agentic loop
func runScriptMode(ctx context.Context, mcpConfig *config.Config, prompt string, noExit bool) error {
	// Set up logging
	if debugMode || mcpConfig.Debug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// Get final values from viper and script config
	finalModel := viper.GetString("model")
	if finalModel == "" && mcpConfig.Model != "" {
		finalModel = mcpConfig.Model
	}
	if finalModel == "" {
		finalModel = "anthropic:claude-sonnet-4-20250514" // default
	}

	finalSystemPrompt := viper.GetString("system-prompt")
	if finalSystemPrompt == "" && mcpConfig.SystemPrompt != "" {
		finalSystemPrompt = mcpConfig.SystemPrompt
	}

	finalDebug := viper.GetBool("debug") || mcpConfig.Debug
	finalCompact := viper.GetBool("compact")
	finalMaxSteps := viper.GetInt("max-steps")
	if finalMaxSteps == 0 && mcpConfig.MaxSteps != 0 {
		finalMaxSteps = mcpConfig.MaxSteps
	}

	finalProviderURL := viper.GetString("provider-url")
	if finalProviderURL == "" && mcpConfig.ProviderURL != "" {
		finalProviderURL = mcpConfig.ProviderURL
	}

	finalProviderAPIKey := viper.GetString("provider-api-key")
	if finalProviderAPIKey == "" && mcpConfig.ProviderAPIKey != "" {
		finalProviderAPIKey = mcpConfig.ProviderAPIKey
	}

	finalMaxTokens := viper.GetInt("max-tokens")
	if finalMaxTokens == 0 && mcpConfig.MaxTokens != 0 {
		finalMaxTokens = mcpConfig.MaxTokens
	}
	if finalMaxTokens == 0 {
		finalMaxTokens = 4096 // default
	}

	finalTemperature := float32(viper.GetFloat64("temperature"))
	if finalTemperature == 0 && mcpConfig.Temperature != nil {
		finalTemperature = *mcpConfig.Temperature
	}
	if finalTemperature == 0 {
		finalTemperature = 0.7 // default
	}

	finalTopP := float32(viper.GetFloat64("top-p"))
	if finalTopP == 0 && mcpConfig.TopP != nil {
		finalTopP = *mcpConfig.TopP
	}
	if finalTopP == 0 {
		finalTopP = 0.95 // default
	}

	finalTopK := int32(viper.GetInt("top-k"))
	if finalTopK == 0 && mcpConfig.TopK != nil {
		finalTopK = *mcpConfig.TopK
	}
	if finalTopK == 0 {
		finalTopK = 40 // default
	}

	finalStopSequences := viper.GetStringSlice("stop-sequences")
	if len(finalStopSequences) == 0 && len(mcpConfig.StopSequences) > 0 {
		finalStopSequences = mcpConfig.StopSequences
	}

	// Load system prompt
	systemPrompt, err := config.LoadSystemPrompt(finalSystemPrompt)
	if err != nil {
		return fmt.Errorf("failed to load system prompt: %v", err)
	}

	// Create model configuration
	modelConfig := &models.ProviderConfig{
		ModelString:    finalModel,
		SystemPrompt:   systemPrompt,
		ProviderAPIKey: finalProviderAPIKey,
		ProviderURL:    finalProviderURL,
		MaxTokens:      finalMaxTokens,
		Temperature:    &finalTemperature,
		TopP:           &finalTopP,
		TopK:           &finalTopK,
		StopSequences:  finalStopSequences,
	}

	// Create the agent using the factory (scripts don't need spinners)
	mcpAgent, err := agent.CreateAgent(ctx, &agent.AgentCreationOptions{
		ModelConfig:      modelConfig,
		MCPConfig:        mcpConfig,
		SystemPrompt:     systemPrompt,
		MaxSteps:         finalMaxSteps,
		StreamingEnabled: viper.GetBool("stream"),
		ShowSpinner:      false, // Scripts don't need spinners
		Quiet:            quietFlag,
		SpinnerFunc:      nil, // No spinner function needed
	})
	if err != nil {
		return fmt.Errorf("failed to create agent: %v", err)
	}
	defer mcpAgent.Close()

	// Get model name for display
	parts := strings.SplitN(finalModel, ":", 2)
	modelName := "Unknown"
	if len(parts) == 2 {
		modelName = parts[1]
	}

	// Create an adapter for the agent to match the UI interface
	agentAdapter := &agentUIAdapter{agent: mcpAgent}

	// Create CLI interface using the factory
	cli, err := ui.SetupCLI(&ui.CLISetupOptions{
		Agent:          agentAdapter,
		ModelString:    finalModel,
		Debug:          finalDebug,
		Compact:        finalCompact,
		Quiet:          quietFlag,
		ShowDebug:      false, // Will be handled separately below
		ProviderAPIKey: finalProviderAPIKey,
	})
	if err != nil {
		return fmt.Errorf("failed to setup CLI: %v", err)
	}

	// Display debug configuration if debug mode is enabled
	if !quietFlag && cli != nil && finalDebug {
		debugConfig := map[string]any{
			"model":         finalModel,
			"max-steps":     finalMaxSteps,
			"max-tokens":    finalMaxTokens,
			"temperature":   finalTemperature,
			"top-p":         finalTopP,
			"top-k":         finalTopK,
			"provider-url":  finalProviderURL,
			"system-prompt": finalSystemPrompt,
		}

		// Only include non-empty stop sequences
		if len(finalStopSequences) > 0 {
			debugConfig["stop-sequences"] = finalStopSequences
		}

		// Only include API keys if they're set (but don't show the actual values for security)
		if finalProviderAPIKey != "" {
			debugConfig["provider-api-key"] = "[SET]"
		}

		cli.DisplayDebugConfig(debugConfig)
	}

	// Prepare data for slash commands
	var serverNames []string
	for name := range mcpConfig.MCPServers {
		serverNames = append(serverNames, name)
	}

	tools := mcpAgent.GetTools()
	var toolNames []string
	for _, tool := range tools {
		if info, err := tool.Info(ctx); err == nil {
			toolNames = append(toolNames, info.Name)
		}
	}

	// Configure and run unified agentic loop
	var messages []*schema.Message
	config := AgenticLoopConfig{
		IsInteractive:    prompt == "", // If no prompt, start in interactive mode
		InitialPrompt:    prompt,
		ContinueAfterRun: noExit,
		Quiet:            quietFlag,
		ServerNames:      serverNames,
		ToolNames:        toolNames,
		ModelName:        modelName,
		MCPConfig:        mcpConfig,
	}

	return runAgenticLoop(ctx, mcpAgent, cli, messages, config)
}
