package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcphost/internal/config"
)

// MCPTool wraps an MCP tool to implement eino's InvokableTool interface
type MCPTool struct {
	client   client.MCPClient
	toolInfo *mcp.Tool
	name     string
}

// Info returns the tool information for eino
func (t *MCPTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	// Convert MCP tool schema to eino schema
	properties := make(map[string]*schema.ParameterInfo)

	// Handle the input schema
	if t.toolInfo.InputSchema.Properties != nil {
		for name, prop := range t.toolInfo.InputSchema.Properties {
			if propMap, ok := prop.(map[string]interface{}); ok {
				paramInfo := &schema.ParameterInfo{
					Type: schema.String, // Default type
				}
				if typeVal, ok := propMap["type"].(string); ok {
					paramInfo.Type = schema.DataType(typeVal)
				}
				if desc, ok := propMap["description"].(string); ok {
					paramInfo.Desc = desc
				}
				properties[name] = paramInfo
			}
		}
	}

	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.toolInfo.Description,
		ParamsOneOf: schema.NewParamsOneOfByParams(properties),
	}, nil
}

// InvokableRun implements the InvokableTool interface
func (t *MCPTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %v", err)
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = t.toolInfo.Name
	req.Params.Arguments = args

	result, err := t.client.CallTool(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to call tool: %v", err)
	}

	// Convert result to string
	if result.Content != nil {
		var resultText string
		for _, item := range result.Content {
			if textContent, ok := item.(mcp.TextContent); ok {
				resultText += textContent.Text + " "
			}
		}
		return resultText, nil
	}

	return "", nil
}

// MCPToolManager manages MCP tools and clients
type MCPToolManager struct {
	clients map[string]client.MCPClient
	tools   []tool.BaseTool
}

// NewMCPToolManager creates a new MCP tool manager
func NewMCPToolManager() *MCPToolManager {
	return &MCPToolManager{
		clients: make(map[string]client.MCPClient),
		tools:   make([]tool.BaseTool, 0),
	}
}

// LoadTools loads tools from MCP servers based on configuration
func (m *MCPToolManager) LoadTools(ctx context.Context, config *config.Config) error {
	for serverName, serverConfig := range config.MCPServers {
		client, err := m.createMCPClient(ctx, serverName, serverConfig)
		if err != nil {
			return fmt.Errorf("failed to create MCP client for %s: %v", serverName, err)
		}

		m.clients[serverName] = client

		// Initialize the client
		if err := m.initializeClient(ctx, client); err != nil {
			return fmt.Errorf("failed to initialize MCP client for %s: %v", serverName, err)
		}

		// Get tools from this server
		toolsResult, err := client.ListTools(ctx, mcp.ListToolsRequest{})
		if err != nil {
			return fmt.Errorf("failed to list tools from server %s: %v", serverName, err)
		}

		// Convert MCP tools to eino tools
		for _, mcpTool := range toolsResult.Tools {
			einoTool := &MCPTool{
				client:   client,
				toolInfo: &mcpTool,
				name:     fmt.Sprintf("%s__%s", serverName, mcpTool.Name),
			}
			m.tools = append(m.tools, einoTool)
		}
	}

	return nil
}

// GetTools returns all loaded tools
func (m *MCPToolManager) GetTools() []tool.BaseTool {
	return m.tools
}

// Close closes all MCP clients
func (m *MCPToolManager) Close() error {
	for name, client := range m.clients {
		if err := client.Close(); err != nil {
			return fmt.Errorf("failed to close client %s: %v", name, err)
		}
	}
	return nil
}

func (m *MCPToolManager) createMCPClient(ctx context.Context, serverName string, serverConfig config.MCPServerConfig) (client.MCPClient, error) {
	if serverConfig.Command != "" {
		// STDIO client
		return client.NewStdioMCPClient(serverConfig.Command, nil, serverConfig.Args...)
	} else if serverConfig.URL != "" {
		// SSE client
		sseClient, err := client.NewSSEMCPClient(serverConfig.URL)
		if err != nil {
			return nil, err
		}
		
		// Start the SSE client
		if err := sseClient.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start SSE client: %v", err)
		}
		
		return sseClient, nil
	}

	return nil, fmt.Errorf("invalid server configuration for %s: must specify either command or url", serverName)
}

func (m *MCPToolManager) initializeClient(ctx context.Context, client client.MCPClient) error {
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "mcphost",
		Version: "1.0.0",
	}

	_, err := client.Initialize(ctx, initRequest)
	return err
}