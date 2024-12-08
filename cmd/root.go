package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/log"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/charmbracelet/glamour"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	renderer *glamour.TermRenderer

	configFile string
)

var rootCmd = &cobra.Command{
	Use:   "mcphost",
	Short: "Chat with Claude 3.5 Sonnet or Ollama models",
	Long: `MCPHost is a CLI tool that allows you to interact with Claude 3.5 Sonnet or Ollama models.
It supports various tools through MCP servers and provides streaming responses.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPHost()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file (default is $HOME/mcp.json)")
}

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80 // Fallback width
	}
	return width - 20
}

func updateRenderer() error {
	width := getTerminalWidth()
	var err error
	renderer, err = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	return err
}

func runPrompt(
	client *anthropic.Client,
	mcpClients map[string]*mcpclient.StdioMCPClient,
	tools []anthropic.ToolParam,
	prompt string,
	messages *[]anthropic.MessageParam,
) error {
	// Display the user's prompt if it's not empty (i.e., not a tool response)
	if prompt != "" {
		fmt.Printf("\n%s\n", promptStyle.Render("You: "+prompt))
		*messages = append(
			*messages,
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		)
	}

	var messagePtr *anthropic.Message
	var err error
	action := func() {
		messagePtr, err = client.Messages.New(
			context.Background(),
			anthropic.MessageNewParams{
				Model: anthropic.F(
					anthropic.ModelClaude_3_5_Sonnet_20240620,
				),
				MaxTokens: anthropic.F(int64(4096)),
				Messages:  anthropic.F(*messages),
				Tools:     anthropic.F(tools),
			},
		)
	}
	_ = spinner.New().Title("Thinking...").Action(action).Run()

	if err != nil {
		return err
	}

	message := *messagePtr // Dereference the pointer
	fmt.Print(responseStyle.Render("\nClaude: "))

	toolResults := []anthropic.ContentBlockParamUnion{}

	for _, block := range message.Content {
		switch block := block.AsUnion().(type) {
		case anthropic.TextBlock:
			if err := updateRenderer(); err != nil {
				return fmt.Errorf("error updating renderer: %v", err)
			}
			str, err := renderer.Render(block.Text + "\n")
			if err != nil {
				log.Error("Failed to render response", "error", err)
				fmt.Print(block.Text + "\n")
				continue
			}
			fmt.Print(str)

		case anthropic.ToolUseBlock:
			log.Info("🔧 Using tool", "name", block.Name)

			parts := strings.Split(block.Name, "__")
			if len(parts) != 2 {
				fmt.Printf("Error: Invalid tool name format: %s\n", block.Name)
				continue
			}

			serverName, toolName := parts[0], parts[1]
			mcpClient, ok := mcpClients[serverName]
			if !ok {
				fmt.Printf("Error: Server not found: %s\n", serverName)
				continue
			}

			var toolArgs map[string]interface{}
			if err := json.Unmarshal(block.Input, &toolArgs); err != nil {
				fmt.Printf("Error parsing tool arguments: %v\n", err)
				continue
			}

			var toolResultPtr *mcp.CallToolResult
			action := func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				toolResultPtr, err = mcpClient.CallTool(ctx, mcp.CallToolRequest{
					Params: struct {
						Name      string                 `json:"name"`
						Arguments map[string]interface{} `json:"arguments,omitempty"`
					}{
						Name:      toolName,
						Arguments: toolArgs,
					},
				})
			}
			_ = spinner.New().
				Title(fmt.Sprintf("Running tool %s...", toolName)).
				Action(action).
				Run()

			if err != nil {
				errMsg := fmt.Sprintf("Error calling tool %s: %v", toolName, err)
				fmt.Printf("\n%s\n", errorStyle.Render(errMsg))

				// Add error message as tool result
				toolResults = append(toolResults,
					anthropic.NewToolResultBlock(block.ID, errMsg, true))
				continue
			}

			toolResult := *toolResultPtr // Dereference the pointer
			resultJSON, err := json.Marshal(toolResult)
			if err != nil {
				fmt.Printf("Error marshaling tool result: %v\n", err)
				continue
			}

			toolResults = append(toolResults,
				anthropic.NewToolResultBlock(block.ID, string(resultJSON), toolResult.IsError))
		}
	}

	*messages = append(*messages, message.ToParam())

	if len(toolResults) > 0 {
		*messages = append(*messages, anthropic.NewUserMessage(toolResults...))
		// Make another call to get Claude's response to the tool results
		return runPrompt(client, mcpClients, tools, "", messages)
	}

	fmt.Println() // Add spacing
	return nil
}

func runMCPHost() error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	mcpConfig, err := loadMCPConfig()
	if err != nil {
		return fmt.Errorf("error loading MCP config: %v", err)
	}

	mcpClients, err := createMCPClients(mcpConfig)
	if err != nil {
		return fmt.Errorf("error creating MCP clients: %v", err)
	}

	defer func() {
		log.Info("Shutting down MCP servers...")
		for name, client := range mcpClients {
			if err := client.Close(); err != nil {
				log.Error("Failed to close server", "name", name, "error", err)
			} else {
				log.Info("Server closed", "name", name)
			}
		}
	}()

	for name := range mcpClients {
		log.Info("Server connected", "name", name)
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	var allTools []anthropic.ToolParam
	for serverName, mcpClient := range mcpClients {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		toolsResult, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
		cancel()

		if err != nil {
			log.Error(
				"Error fetching tools",
				"server",
				serverName,
				"error",
				err,
			)
			continue
		}

		serverTools := mcpToolsToAnthropicTools(serverName, toolsResult.Tools)
		allTools = append(allTools, serverTools...)
		log.Info(
			"Tools loaded",
			"server",
			serverName,
			"count",
			len(toolsResult.Tools),
		)
	}

	if err := updateRenderer(); err != nil {
		return fmt.Errorf("error initializing renderer: %v", err)
	}

	messages := make([]anthropic.MessageParam, 0)

	// Main interaction loop
	for {
		width := getTerminalWidth()
		var prompt string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewText().
					Key("prompt").
					Title("Enter your prompt (Type /help for commands, Ctrl+C to quit)").
					Value(&prompt),
			),
		).WithWidth(width)

		err := form.Run()
		if err != nil {
			// Check if it's a user abort (Ctrl+C)
			if err.Error() == "user aborted" {
				fmt.Println("\nGoodbye!")
				return nil // Exit cleanly
			}
			return err // Return other errors normally
		}

		prompt = form.GetString("prompt")
		if prompt == "" {
			continue
		}

		// Handle slash commands
		handled, err := handleSlashCommand(prompt, mcpConfig, mcpClients)
		if err != nil {
			return err
		}
		if handled {
			continue
		}

		err = runPrompt(client, mcpClients, allTools, prompt, &messages)
		if err != nil {
			return err
		}
	}
}
