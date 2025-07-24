package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mark3labs/mcphost/internal/hooks"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage MCPHost hooks",
	Long:  "Commands for managing and testing MCPHost hooks configuration",
}

var hooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := hooks.LoadHooksConfig()
		if err != nil {
			return fmt.Errorf("loading hooks config: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "EVENT\tMATCHER\tCOMMAND\tTIMEOUT")

		for event, matchers := range config.Hooks {
			for _, matcher := range matchers {
				for _, hook := range matcher.Hooks {
					timeout := "60s"
					if hook.Timeout > 0 {
						timeout = fmt.Sprintf("%ds", hook.Timeout)
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						event, matcher.Matcher, hook.Command, timeout)
				}
			}
		}

		return w.Flush()
	},
}

var hooksValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate hooks configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := hooks.LoadHooksConfig()
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		// Additional validation
		if err := hooks.ValidateHookConfig(config); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		fmt.Println("✓ Hooks configuration is valid")
		return nil
	},
}

var hooksInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate example hooks configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		example := &hooks.HookConfig{
			Hooks: map[hooks.HookEvent][]hooks.HookMatcher{
				// PreToolUse - runs before any tool execution
				hooks.PreToolUse: {
					{
						Matcher: "bash.*",
						Hooks: []hooks.HookEntry{
							{
								Type:    "command",
								Command: `mkdir -p "${XDG_CONFIG_HOME:-$HOME/.config}/mcphost/logs" && jq -r '"[" + (now | strftime("%Y-%m-%d %H:%M:%S")) + "] $ " + .tool_input.command' >> "${XDG_CONFIG_HOME:-$HOME/.config}/mcphost/logs/bash-commands.log"`,
								Timeout: 5,
							},
						},
					},
					{
						Matcher: ".*", // Log all tool usage
						Hooks: []hooks.HookEntry{
							{
								Type:    "command",
								Command: `jq -c '{time: now | strftime("%Y-%m-%d %H:%M:%S"), event: "pre", tool: .tool_name, input: .tool_input}' >> "${XDG_CONFIG_HOME:-$HOME/.config}/mcphost/logs/all-tools.jsonl"`,
								Timeout: 5,
							},
						},
					},
				},
				// PostToolUse - runs after tool execution completes
				hooks.PostToolUse: {
					{
						Matcher: "bash.*",
						Hooks: []hooks.HookEntry{
							{
								Type:    "command",
								Command: `jq -c '{time: now | strftime("%Y-%m-%d %H:%M:%S"), cmd: .tool_input.command, exit: .tool_response._meta.exit, stdout: (.tool_response._meta.stdout | rtrimstr("\n") | .[0:100]), stderr: (.tool_response._meta.stderr | rtrimstr("\n"))}' >> "${XDG_CONFIG_HOME:-$HOME/.config}/mcphost/logs/bash-audit.jsonl"`,
								Timeout: 5,
							},
						},
					},
					{
						Matcher: "mcp__.*", // Log MCP tool responses
						Hooks: []hooks.HookEntry{
							{
								Type:    "command",
								Command: `jq -c '{time: now | strftime("%Y-%m-%d %H:%M:%S"), tool: .tool_name, response_preview: (.tool_response | tostring | .[0:200])}' >> "${XDG_CONFIG_HOME:-$HOME/.config}/mcphost/logs/mcp-tools.jsonl"`,
								Timeout: 5,
							},
						},
					},
				},
				// UserPromptSubmit - runs when user submits a prompt
				hooks.UserPromptSubmit: {
					{
						Hooks: []hooks.HookEntry{
							{
								Type:    "command",
								Command: `mkdir -p "${XDG_CONFIG_HOME:-$HOME/.config}/mcphost/logs" && jq -r '"[" + (now | strftime("%Y-%m-%d %H:%M:%S")) + "] " + .prompt' >> "${XDG_CONFIG_HOME:-$HOME/.config}/mcphost/logs/prompts.log"`,
							},
						},
					},
				},
				// Stop - runs when the main agent finishes responding
				hooks.Stop: {
					{
						Hooks: []hooks.HookEntry{
							{
								Type:    "command",
								Command: `jq -r '"[" + (now | strftime("%Y-%m-%d %H:%M:%S")) + "] Session " + .session_id + " stopped"' >> "${XDG_CONFIG_HOME:-$HOME/.config}/mcphost/logs/sessions.log"`,
							},
						},
					},
				},
			},
		}

		// Create .mcphost directory if it doesn't exist
		if err := os.MkdirAll(".mcphost", 0755); err != nil {
			return fmt.Errorf("creating .mcphost directory: %w", err)
		}

		// Write example configuration
		data, err := yaml.Marshal(example)
		if err != nil {
			return fmt.Errorf("marshaling example: %w", err)
		}

		if err := os.WriteFile(".mcphost/hooks.yml", data, 0644); err != nil {
			return fmt.Errorf("writing example: %w", err)
		}

		fmt.Println("Created .mcphost/hooks.yml with example configuration")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(hooksCmd)
	hooksCmd.AddCommand(hooksListCmd)
	hooksCmd.AddCommand(hooksValidateCmd)
	hooksCmd.AddCommand(hooksInitCmd)
}
