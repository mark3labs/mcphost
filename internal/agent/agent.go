package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/tools"
)

// MCPAgent wraps the eino React Agent with MCP tool integration
type MCPAgent struct {
	agent       *react.Agent
	toolManager *tools.MCPToolManager
	model       model.ToolCallingChatModel
}

// Config holds configuration for creating an MCPAgent
type Config struct {
	ModelConfig      *models.ProviderConfig
	MCPConfig        *config.Config
	SystemPrompt     string
	MaxSteps         int
	MessageWindow    int
}

// NewMCPAgent creates a new agent with MCP tool integration
func NewMCPAgent(ctx context.Context, config *Config) (*MCPAgent, error) {
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

	// Configure the React Agent
	agentConfig := &react.AgentConfig{
		ToolCallingModel: model,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: toolManager.GetTools(),
		},
		MaxStep: config.MaxSteps,
	}

	// Add system prompt if provided
	if config.SystemPrompt != "" {
		agentConfig.MessageModifier = func(ctx context.Context, input []*schema.Message) []*schema.Message {
			result := make([]*schema.Message, 0, len(input)+1)
			result = append(result, schema.SystemMessage(config.SystemPrompt))
			result = append(result, input...)
			return result
		}
	}

	// Create the React Agent
	reactAgent, err := react.NewAgent(ctx, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create React agent: %v", err)
	}

	return &MCPAgent{
		agent:       reactAgent,
		toolManager: toolManager,
		model:       model,
	}, nil
}

// Generate processes a message and returns the agent's response
func (a *MCPAgent) Generate(ctx context.Context, messages []*schema.Message, opts ...compose.Option) (*schema.Message, error) {
	// Convert compose options to agent options
	agentOpts := []agent.AgentOption{}
	if len(opts) > 0 {
		agentOpts = append(agentOpts, agent.WithComposeOptions(opts...))
	}
	return a.agent.Generate(ctx, messages, agentOpts...)
}

// Stream processes a message and returns a streaming response
func (a *MCPAgent) Stream(ctx context.Context, messages []*schema.Message, opts ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	// Convert compose options to agent options
	agentOpts := []agent.AgentOption{}
	if len(opts) > 0 {
		agentOpts = append(agentOpts, agent.WithComposeOptions(opts...))
	}
	return a.agent.Stream(ctx, messages, agentOpts...)
}

// GetTools returns the list of available tools
func (a *MCPAgent) GetTools() []tool.BaseTool {
	return a.toolManager.GetTools()
}

// Close closes the agent and cleans up resources
func (a *MCPAgent) Close() error {
	return a.toolManager.Close()
}