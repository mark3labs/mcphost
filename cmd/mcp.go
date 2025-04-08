package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	"github.com/charmbracelet/log"

	"strings"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcphost/pkg/history"
	"github.com/mark3labs/mcphost/pkg/llm"
)

var (
	// Tokyo Night theme colors
	tokyoPurple = lipgloss.Color("99")  // #9d7cd8
	tokyoCyan   = lipgloss.Color("73")  // #7dcfff
	tokyoBlue   = lipgloss.Color("111") // #7aa2f7
	tokyoGreen  = lipgloss.Color("120") // #73daca
	tokyoRed    = lipgloss.Color("203") // #f7768e
	tokyoOrange = lipgloss.Color("215") // #ff9e64
	tokyoFg     = lipgloss.Color("189") // #c0caf5
	tokyoGray   = lipgloss.Color("237") // #3b4261
	tokyoBg     = lipgloss.Color("234") // #1a1b26

	promptStyle = lipgloss.NewStyle().
			Foreground(tokyoBlue).
			PaddingLeft(2)

	responseStyle = lipgloss.NewStyle().
			Foreground(tokyoFg).
			PaddingLeft(2)

	errorStyle = lipgloss.NewStyle().
			Foreground(tokyoRed).
			Bold(true)

	toolNameStyle = lipgloss.NewStyle().
			Foreground(tokyoCyan).
			Bold(true)

	descriptionStyle = lipgloss.NewStyle().
				Foreground(tokyoFg).
				PaddingBottom(1)

	contentStyle = lipgloss.NewStyle().
			Background(tokyoBg).
			PaddingLeft(4).
			PaddingRight(4)
)

type MCPConfig struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

type ServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

func mcpToolsToAnthropicTools(
	serverName string,
	mcpTools []mcp.Tool,
) []llm.Tool {
	anthropicTools := make([]llm.Tool, len(mcpTools))

	for i, tool := range mcpTools {
		namespacedName := fmt.Sprintf("%s__%s", serverName, tool.Name)

		anthropicTools[i] = llm.Tool{
			Name:        namespacedName,
			Description: tool.Description,
			InputSchema: llm.Schema{
				Type:       tool.InputSchema.Type,
				Properties: tool.InputSchema.Properties,
				Required:   tool.InputSchema.Required,
			},
		}
	}

	return anthropicTools
}

func loadMCPConfig() (*MCPConfig, error) {
	var configPath string
	if configFile != "" {
		configPath = configFile
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("error getting home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, ".mcp.json")
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		defaultConfig := MCPConfig{
			MCPServers: make(map[string]ServerConfig),
		}

		// Create the file with default config
		configData, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("error creating default config: %w", err)
		}

		if err := os.WriteFile(configPath, configData, 0644); err != nil {
			return nil, fmt.Errorf("error writing default config file: %w", err)
		}

		log.Info("Created default config file", "path", configPath)
		return &defaultConfig, nil
	}

	// Read existing config
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf(
			"error reading config file %s: %w",
			configPath,
			err,
		)
	}

	var config MCPConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return &config, nil
}

func createMCPClients(
	config *MCPConfig,
) (map[string]mcpclient.MCPClient, error) {
	clients := make(map[string]mcpclient.MCPClient)

	for name, server := range config.MCPServers {
		var env []string
		for k, v := range server.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		var client mcpclient.MCPClient
		var err error

		if server.Command == "sse_server" {
			if len(server.Args) == 0 {
				return nil, fmt.Errorf(
					"no arguments provided for sse command",
				)
			}

			client, err = mcpclient.NewSSEMCPClient(
				server.Args[0],
			)
			if err == nil {
				err = client.(*mcpclient.SSEMCPClient).Start(context.Background())
			}
		} else {
			client, err = mcpclient.NewStdioMCPClient(
				server.Command,
				env,
				server.Args...)
		}
		if err != nil {
			for _, c := range clients {
				c.Close()
			}
			return nil, fmt.Errorf(
				"failed to create MCP client for %s: %w",
				name,
				err,
			)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		log.Info("Initializing server...", "name", name)
		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{
			Name:    "mcphost",
			Version: "0.1.0",
		}
		initRequest.Params.Capabilities = mcp.ClientCapabilities{}

		_, err = client.Initialize(ctx, initRequest)
		if err != nil {
			client.Close()
			for _, c := range clients {
				c.Close()
			}
			return nil, fmt.Errorf(
				"failed to initialize MCP client for %s: %w",
				name,
				err,
			)
		}

		clients[name] = client
	}

	return clients, nil
}

func handleSlashCommand(
	prompt string,
	mcpConfig *MCPConfig,
	mcpClients map[string]mcpclient.MCPClient,
	messages interface{},
) (bool, error) {
	if !strings.HasPrefix(prompt, "/") {
		return false, nil
	}

	switch strings.ToLower(strings.TrimSpace(prompt)) {
	case "/tools":
		handleToolsCommand(mcpClients)
		return true, nil
	case "/help":
		handleHelpCommand()
		return true, nil
	case "/history":
		handleHistoryCommand(messages.([]history.HistoryMessage))
		return true, nil
	case "/servers":
		handleServersCommand(mcpConfig)
		return true, nil
	case "/quit":
		fmt.Println("\nGoodbye!")
		defer os.Exit(0)
		return true, nil
	default:
		fmt.Printf("%s\nType /help to see available commands\n\n",
			errorStyle.Render("Unknown command: "+prompt))
		return true, nil
	}
}

func handleHelpCommand() {
	if err := updateRenderer(); err != nil {
		fmt.Printf(
			"\n%s\n",
			errorStyle.Render(fmt.Sprintf("Error updating renderer: %v", err)),
		)
		return
	}
	var markdown strings.Builder

	markdown.WriteString("# Available Commands\n\n")
	markdown.WriteString("The following commands are available:\n\n")
	markdown.WriteString("- **/help**: Show this help message\n")
	markdown.WriteString("- **/tools**: List all available tools\n")
	markdown.WriteString("- **/servers**: List configured MCP servers\n")
	markdown.WriteString("- **/history**: Display conversation history\n")
	markdown.WriteString("- **/quit**: Exit the application\n")
	markdown.WriteString("\nYou can also press Ctrl+C at any time to quit.\n")

	markdown.WriteString("\n## Available Models\n\n")
	markdown.WriteString("Specify models using the --model or -m flag:\n\n")
	markdown.WriteString(
		"- **Anthropic Claude**: `anthropic:claude-3-5-sonnet-latest`\n",
	)
	markdown.WriteString("- **Ollama Models**: `ollama:modelname`\n")
	markdown.WriteString("\nExamples:\n")
	markdown.WriteString("```\n")
	markdown.WriteString("mcphost -m anthropic:claude-3-5-sonnet-latest\n")
	markdown.WriteString("mcphost -m ollama:qwen2.5:3b\n")
	markdown.WriteString("```\n")

	rendered, err := renderer.Render(markdown.String())
	if err != nil {
		fmt.Printf(
			"\n%s\n",
			errorStyle.Render(fmt.Sprintf("Error rendering help: %v", err)),
		)
		return
	}

	fmt.Print(rendered)
}

func handleServersCommand(config *MCPConfig) {
	if err := updateRenderer(); err != nil {
		fmt.Printf(
			"\n%s\n",
			errorStyle.Render(fmt.Sprintf("Error updating renderer: %v", err)),
		)
		return
	}

	var markdown strings.Builder
	action := func() {
		if len(config.MCPServers) == 0 {
			markdown.WriteString("No servers configured.\n")
		} else {
			for name, server := range config.MCPServers {
				markdown.WriteString(fmt.Sprintf("# %s\n\n", name))
				markdown.WriteString("*Command*\n")
				markdown.WriteString(fmt.Sprintf("`%s`\n\n", server.Command))

				markdown.WriteString("*Arguments*\n")
				if len(server.Args) > 0 {
					markdown.WriteString(fmt.Sprintf("`%s`\n", strings.Join(server.Args, " ")))
				} else {
					markdown.WriteString("*None*\n")
				}
				markdown.WriteString("\n") // Add spacing between servers
			}
		}
	}

	_ = spinner.New().
		Title("Loading server configuration...").
		Action(action).
		Run()

	rendered, err := renderer.Render(markdown.String())
	if err != nil {
		fmt.Printf(
			"\n%s\n",
			errorStyle.Render(fmt.Sprintf("Error rendering servers: %v", err)),
		)
		return
	}

	// Create a container style with margins
	containerStyle := lipgloss.NewStyle().
		MarginLeft(4).
		MarginRight(4)

	// Wrap the rendered content in the container
	fmt.Print("\n" + containerStyle.Render(rendered) + "\n")
}

func handleToolsCommand(mcpClients map[string]mcpclient.MCPClient) {
	// Get terminal width for proper wrapping
	width := getTerminalWidth()

	// Adjust width to account for margins and list indentation
	contentWidth := width - 12 // Account for margins and list markers

	// If tools are disabled (empty client map), show a message
	if len(mcpClients) == 0 {
		fmt.Print(
			"\n" + contentStyle.Render(
				"Tools are currently disabled for this model.\n",
			) + "\n\n",
		)
		return
	}

	type serverTools struct {
		tools []mcp.Tool
		err   error
	}
	results := make(map[string]serverTools)

	action := func() {
		for serverName, mcpClient := range mcpClients {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				10*time.Second,
			)
			defer cancel()

			toolsResult, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
			if err != nil {
				results[serverName] = serverTools{
					tools: nil,
					err:   err,
				}
				continue
			}

			var tools []mcp.Tool
			if toolsResult != nil {
				tools = toolsResult.Tools
			}

			results[serverName] = serverTools{
				tools: tools,
				err:   nil,
			}
		}
	}
	_ = spinner.New().
		Title("Fetching tools from all servers...").
		Action(action).
		Run()

	// Create a list for all servers
	l := list.New().
		EnumeratorStyle(lipgloss.NewStyle().Foreground(tokyoPurple).MarginRight(1))

	for serverName, result := range results {
		if result.err != nil {
			fmt.Printf(
				"\n%s\n",
				errorStyle.Render(
					fmt.Sprintf(
						"Error fetching tools from %s: %v",
						serverName,
						result.err,
					),
				),
			)
			continue
		}

		// Create a sublist for each server's tools
		serverList := list.New().
			EnumeratorStyle(lipgloss.NewStyle().Foreground(tokyoCyan).MarginRight(1))

		if len(result.tools) == 0 {
			serverList.Item("No tools available")
		} else {
			for _, tool := range result.tools {
				// Create a description style with word wrap
				descStyle := lipgloss.NewStyle().
					Foreground(tokyoFg).
					Width(contentWidth).
					Align(lipgloss.Left)

				// Create a description sublist for each tool
				toolDesc := list.New().
					EnumeratorStyle(lipgloss.NewStyle().Foreground(tokyoGreen).MarginRight(1)).
					Item(descStyle.Render(tool.Description))

				// Add the tool with its description as a nested list
				serverList.Item(toolNameStyle.Render(tool.Name)).
					Item(toolDesc)
			}
		}

		// Add the server and its tools to the main list
		l.Item(serverName).Item(serverList)
	}

	// Create a container style with margins
	containerStyle := lipgloss.NewStyle().
		Margin(2).
		Width(width)

	// Wrap the entire content in the container
	fmt.Print("\n" + containerStyle.Render(l.String()) + "\n")
}
func displayMessageHistory(messages []history.HistoryMessage) {
	if err := updateRenderer(); err != nil {
		fmt.Printf(
			"\n%s\n",
			errorStyle.Render(fmt.Sprintf("Error updating renderer: %v", err)),
		)
		return
	}

	var markdown strings.Builder
	markdown.WriteString("# Conversation History\n\n")

	for _, msg := range messages {
		roleTitle := "## User"
		if msg.Role == "assistant" {
			roleTitle = "## Assistant"
		} else if msg.Role == "system" {
			roleTitle = "## System"
		}
		markdown.WriteString(roleTitle + "\n\n")

		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				markdown.WriteString("### Text\n")
				markdown.WriteString(block.Text + "\n\n")

			case "tool_use":
				markdown.WriteString("### Tool Use\n")
				markdown.WriteString(
					fmt.Sprintf("**Tool:** %s\n\n", block.Name),
				)
				if block.Input != nil {
					prettyInput, err := json.MarshalIndent(
						block.Input,
						"",
						"  ",
					)
					if err != nil {
						markdown.WriteString(
							fmt.Sprintf("Error formatting input: %v\n\n", err),
						)
					} else {
						markdown.WriteString("**Input:**\n```json\n")
						markdown.WriteString(string(prettyInput))
						markdown.WriteString("\n```\n\n")
					}
				}

			case "tool_result":
				markdown.WriteString("### Tool Result\n")
				markdown.WriteString(
					fmt.Sprintf("**Tool ID:** %s\n\n", block.ToolUseID),
				)
				switch v := block.Content.(type) {
				case string:
					markdown.WriteString("```\n")
					markdown.WriteString(v)
					markdown.WriteString("\n```\n\n")
				case []history.ContentBlock:
					for _, contentBlock := range v {
						if contentBlock.Type == "text" {
							markdown.WriteString("```\n")
							markdown.WriteString(contentBlock.Text)
							markdown.WriteString("\n```\n\n")
						}
					}
				}
			}
		}
		markdown.WriteString("---\n\n")
	}

	// Render the markdown
	rendered, err := renderer.Render(markdown.String())
	if err != nil {
		fmt.Printf(
			"\n%s\n",
			errorStyle.Render(fmt.Sprintf("Error rendering history: %v", err)),
		)
		return
	}

	// Print directly without box
	fmt.Print("\n" + rendered + "\n")
}
