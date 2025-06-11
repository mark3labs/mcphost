package agent

import (
	"context"
	"encoding/json"
	"fmt"

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

// GenerateWithLoop processes messages with a custom loop that displays tool calls in real-time
func (a *Agent) GenerateWithLoop(ctx context.Context, messages []*schema.Message,
	onToolCall ToolCallHandler, onToolExecution ToolExecutionHandler, onToolResult ToolResultHandler, onResponse ResponseHandler, onToolCallContent ToolCallContentHandler) (*schema.Message, error) {

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
		toolInfos = append(toolInfos, info)
		toolMap[info.Name] = t
	}

	// Main loop
	for step := 0; a.maxSteps == 0 || step < a.maxSteps; step++ {
		// Call the LLM
		response, err := a.model.Generate(ctx, workingMessages, model.WithTools(toolInfos))
		if err != nil {
			return nil, fmt.Errorf("failed to generate response: %v", err)
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
			return response, nil
		}
	}

	// If we reach here, we've exceeded max steps
	return schema.AssistantMessage("Maximum number of steps reached.", nil), nil
}

// GetTools returns the list of available tools
func (a *Agent) GetTools() []tool.BaseTool {
	return a.toolManager.GetTools()
}

// Close closes the agent and cleans up resources
func (a *Agent) Close() error {
	return a.toolManager.Close()
}
