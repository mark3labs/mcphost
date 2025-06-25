package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/tools"
)

// AgentConfig is the config for agent.
type AgentConfig struct {
	ModelConfig  *models.ProviderConfig
	MCPConfig    *config.Config
	SystemPrompt string
	MaxSteps     int
}

// ToolCallHandler is a function type for handling tool calls as they happen
type ToolCallHandler func(toolName, toolArgs string)

// ToolExecutionHandler is a function type for handling tool execution start/end
type ToolExecutionHandler func(toolName string, isStarting bool)

// ToolResultHandler is a function type for handling tool results
type ToolResultHandler func(toolName, toolArgs, result string, isError bool)

// ResponseHandler is a function type for handling LLM responses
type ResponseHandler func(content string)

// ToolCallContentHandler is a function type for handling content that accompanies tool calls
type ToolCallContentHandler func(content string)

// Agent is the agent with real-time tool call display.
type Agent struct {
	toolManager  *tools.MCPToolManager
	model        model.ToolCallingChatModel
	maxSteps     int
	systemPrompt string
}

// NewAgent creates an agent with MCP tool integration and real-time tool call display
func NewAgent(ctx context.Context, config *AgentConfig) (*Agent, error) {
	// Create the LLM provider
	model, err := models.CreateProvider(ctx, config.ModelConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create model provider: %v", err)
	}

	// Create and load MCP tools
	toolManager := tools.NewMCPToolManager()
	if err := toolManager.LoadTools(ctx, config.MCPConfig); err != nil {
		return nil, fmt.Errorf("failed to load MCP tools: %v", err)
	}

	return &Agent{
		toolManager:  toolManager,
		model:        model,
		maxSteps:     config.MaxSteps, // Keep 0 for infinite, handle in loop
		systemPrompt: config.SystemPrompt,
	}, nil
}

// GenerateWithLoopResult contains the result and conversation history
type GenerateWithLoopResult struct {
	FinalResponse *schema.Message
	ConversationMessages []*schema.Message // All messages in the conversation (including tool calls and results)
}

// GenerateWithLoop processes messages with a custom loop that displays tool calls in real-time
func (a *Agent) GenerateWithLoop(ctx context.Context, messages []*schema.Message,
	onToolCall ToolCallHandler, onToolExecution ToolExecutionHandler, onToolResult ToolResultHandler, onResponse ResponseHandler, onToolCallContent ToolCallContentHandler) (*GenerateWithLoopResult, error) {

	// Create a copy of messages to avoid modifying the original
	workingMessages := make([]*schema.Message, len(messages))
	copy(workingMessages, messages)

	// Add system prompt if provided
	if a.systemPrompt != "" {
		hasSystemMessage := false
		if len(workingMessages) > 0 && workingMessages[0].Role == schema.System {
			hasSystemMessage = true
		}

		if !hasSystemMessage {
			systemMsg := schema.SystemMessage(a.systemPrompt)
			workingMessages = append([]*schema.Message{systemMsg}, workingMessages...)
		}
	}

	// Get available tools
	availableTools := a.toolManager.GetTools()
	var toolInfos []*schema.ToolInfo
	toolMap := make(map[string]tool.BaseTool)

	for _, t := range availableTools {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		if info == nil {
			continue
		}
		toolInfos = append(toolInfos, info)
		toolMap[info.Name] = t
	}

	// Main loop
	for step := 0; a.maxSteps == 0 || step < a.maxSteps; step++ {
		// Check if context was cancelled before making LLM call
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Call the LLM with cancellation support
		response, err := a.generateWithCancellation(ctx, workingMessages, toolInfos)
		if err != nil {
			return nil, err
		}

		// Add response to working messages
		workingMessages = append(workingMessages, response)

		// Check if this is a tool call or final response
		if len(response.ToolCalls) > 0 {

			// Display any content that accompanies the tool calls
			if response.Content != "" && onToolCallContent != nil {
				onToolCallContent(response.Content)
			}

			// Handle tool calls
			for _, toolCall := range response.ToolCalls {
				// Notify about tool call
				if onToolCall != nil {
					onToolCall(toolCall.Function.Name, toolCall.Function.Arguments)
				}

				// Execute the tool
				if selectedTool, exists := toolMap[toolCall.Function.Name]; exists {
					// Notify tool execution start
					if onToolExecution != nil {
						onToolExecution(toolCall.Function.Name, true)
					}

					output, err := selectedTool.(tool.InvokableTool).InvokableRun(ctx, toolCall.Function.Arguments)

					// Notify tool execution end
					if onToolExecution != nil {
						onToolExecution(toolCall.Function.Name, false)
					}

					if err != nil {
						errorMsg := fmt.Sprintf("Tool execution error: %v", err)
						toolMessage := schema.ToolMessage(errorMsg, toolCall.ID)
						workingMessages = append(workingMessages, toolMessage)

						if onToolResult != nil {
							onToolResult(toolCall.Function.Name, toolCall.Function.Arguments, errorMsg, true)
						}
					} else {
						// Check if this is an MCP tool response with an error
						isError := false
						if output != "" {
							var mcpResult mcp.CallToolResult
							if err := json.Unmarshal([]byte(output), &mcpResult); err == nil && mcpResult.IsError {
								isError = true
							}
						}

						toolMessage := schema.ToolMessage(output, toolCall.ID)
						workingMessages = append(workingMessages, toolMessage)

						if onToolResult != nil {
							onToolResult(toolCall.Function.Name, toolCall.Function.Arguments, output, isError)
						}
					}
				} else {
					errorMsg := fmt.Sprintf("Tool not found: %s", toolCall.Function.Name)
					toolMessage := schema.ToolMessage(errorMsg, toolCall.ID)
					workingMessages = append(workingMessages, toolMessage)

					if onToolResult != nil {
						onToolResult(toolCall.Function.Name, toolCall.Function.Arguments, errorMsg, true)
					}
				}
			}
		} else {
			// This is a final response
			if onResponse != nil && response.Content != "" {
				onResponse(response.Content)
			}
			return &GenerateWithLoopResult{
				FinalResponse: response,
				ConversationMessages: workingMessages,
			}, nil
		}
	}

	// If we reach here, we've exceeded max steps
	finalResponse := schema.AssistantMessage("Maximum number of steps reached.", nil)
	return &GenerateWithLoopResult{
		FinalResponse: finalResponse,
		ConversationMessages: workingMessages,
	}, nil
}

// GetTools returns the list of available tools
func (a *Agent) GetTools() []tool.BaseTool {
	return a.toolManager.GetTools()
}

// generateWithCancellation calls the LLM with ESC key cancellation support
func (a *Agent) generateWithCancellation(ctx context.Context, messages []*schema.Message, toolInfos []*schema.ToolInfo) (*schema.Message, error) {
	// Create a cancellable context for just this LLM call
	llmCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Channel to receive the LLM result
	resultChan := make(chan struct {
		message *schema.Message
		err     error
	}, 1)

	// Start ESC key listener first and wait for it to be ready
	escChan := make(chan bool, 1)
	stopListening := make(chan bool, 1)
	escReady := make(chan bool, 1)

	go func() {
		if a.listenForESC(stopListening, escReady) {
			escChan <- true
		} else {
			escChan <- false
		}
	}()

	// Wait for ESC listener to be ready before starting LLM
	select {
	case <-escReady:
		// ESC listener is ready, proceed
	case <-time.After(100 * time.Millisecond):
		// Timeout waiting for ESC listener, proceed anyway
	case <-ctx.Done():
		close(stopListening)
		return nil, ctx.Err()
	}

	// Now start the LLM generation
	go func() {
		message, err := a.model.Generate(llmCtx, messages, model.WithTools(toolInfos))
		if err != nil {
			err = fmt.Errorf("failed to generate response: %v", err)
		}
		resultChan <- struct {
			message *schema.Message
			err     error
		}{message, err}
	}()

	// Wait for either LLM completion or ESC key
	select {
	case result := <-resultChan:
		// Stop the ESC listener
		close(stopListening)
		return result.message, result.err
	case escPressed := <-escChan:
		if escPressed {
			cancel() // Cancel the LLM context
			return nil, fmt.Errorf("generation cancelled by user")
		}
		// ESC listener stopped normally, wait for LLM result
		result := <-resultChan
		return result.message, result.err
	case <-ctx.Done():
		// Stop the ESC listener
		close(stopListening)
		return nil, ctx.Err()
	}
}

// escListenerModel is a simple Bubble Tea model for ESC key detection
type escListenerModel struct {
	escPressed chan bool
}

func (m escListenerModel) Init() tea.Cmd {
	return nil
}

func (m escListenerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			// Signal ESC was pressed
			select {
			case m.escPressed <- true:
			default:
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m escListenerModel) View() string {
	return "" // No visual output needed
}

// listenForESC listens for ESC key press using Bubble Tea and returns true if detected
func (a *Agent) listenForESC(stopChan chan bool, readyChan chan bool) bool {
	escPressed := make(chan bool, 1)

	model := escListenerModel{
		escPressed: escPressed,
	}

	// Create a Bubble Tea program
	p := tea.NewProgram(model, tea.WithoutRenderer())

	// Start the program in a goroutine
	go func() {
		if _, err := p.Run(); err != nil {
			// Program failed, try to signal completion
			select {
			case escPressed <- false:
			default:
			}
		}
	}()

	// Give the program a moment to initialize, then signal ready
	go func() {
		time.Sleep(10 * time.Millisecond)
		select {
		case readyChan <- true:
		default:
		}
	}()

	// Wait for either ESC key or stop signal
	select {
	case <-stopChan:
		p.Kill()
		// Give the program time to fully terminate
		time.Sleep(50 * time.Millisecond)
		return false
	case pressed := <-escPressed:
		p.Kill()
		// Give the program time to fully terminate
		time.Sleep(50 * time.Millisecond)
		return pressed
	case <-time.After(30 * time.Second):
		// Timeout after 30 seconds to prevent hanging
		p.Kill()
		time.Sleep(50 * time.Millisecond)
		return false
	}
}

// Close closes the agent and cleans up resources
func (a *Agent) Close() error {
	return a.toolManager.Close()
}
