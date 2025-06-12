package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcphost/internal/config"
)

// MCPToolManager manages MCP tools and clients
type MCPToolManager struct {
	clients map[string]client.MCPClient
	tools   []tool.BaseTool
	toolMap map[string]*toolMapping // maps prefixed tool names to their server and original name
}

// toolMapping stores the mapping between prefixed tool names and their original details
type toolMapping struct {
	serverName   string
	originalName string
	client       client.MCPClient
}

// mcpToolImpl implements the eino tool interface with server prefixing
type mcpToolImpl struct {
	info    *schema.ToolInfo
	mapping *toolMapping
}

// NewMCPToolManager creates a new MCP tool manager
func NewMCPToolManager() *MCPToolManager {
	return &MCPToolManager{
		clients: make(map[string]client.MCPClient),
		tools:   make([]tool.BaseTool, 0),
		toolMap: make(map[string]*toolMapping),
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
		listResults, err := client.ListTools(ctx, mcp.ListToolsRequest{})
		if err != nil {
			return fmt.Errorf("failed to list tools from server %s: %v", serverName, err)
		}

		// Create name set for allowed tools
		var nameSet map[string]struct{}
		if len(serverConfig.AllowedTools) > 0 {
			nameSet = make(map[string]struct{})
			for _, name := range serverConfig.AllowedTools {
				nameSet[name] = struct{}{}
			}
		}

		// Convert MCP tools to eino tools with prefixed names
		for _, mcpTool := range listResults.Tools {
			// Filter tools based on allowedTools/excludedTools
			if len(serverConfig.AllowedTools) > 0 {
				if _, ok := nameSet[mcpTool.Name]; !ok {
					continue
				}
			}

			// Check if tool should be excluded
			if m.shouldExcludeTool(mcpTool.Name, serverConfig) {
				continue
			}

			// Convert schema
			marshaledInputSchema, err := sonic.Marshal(mcpTool.InputSchema)
			if err != nil {
				return fmt.Errorf("conv mcp tool input schema fail(marshal): %w, tool name: %s", err, mcpTool.Name)
			}
			inputSchema := &openapi3.Schema{}
			err = sonic.Unmarshal(marshaledInputSchema, inputSchema)
			if err != nil {
				return fmt.Errorf("conv mcp tool input schema fail(unmarshal): %w, tool name: %s", err, mcpTool.Name)
			}

			// Create prefixed tool name
			prefixedName := fmt.Sprintf("%s__%s", serverName, mcpTool.Name)

			// Create tool mapping
			mapping := &toolMapping{
				serverName:   serverName,
				originalName: mcpTool.Name,
				client:       client,
			}
			m.toolMap[prefixedName] = mapping

			// Create eino tool
			einoTool := &mcpToolImpl{
				info: &schema.ToolInfo{
					Name:        prefixedName,
					Desc:        mcpTool.Description,
					ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(inputSchema),
				},
				mapping: mapping,
			}

			m.tools = append(m.tools, einoTool)
		}
	}

	return nil
}

// Info returns the tool information
func (t *mcpToolImpl) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

// InvokableRun executes the tool by mapping back to the original name and server
func (t *mcpToolImpl) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	result, err := t.mapping.client.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: struct {
			Name      string    `json:"name"`
			Arguments any       `json:"arguments,omitempty"`
			Meta      *mcp.Meta `json:"_meta,omitempty"`
		}{
			Name:      t.mapping.originalName, // Use original name, not prefixed
			Arguments: json.RawMessage(argumentsInJSON),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to call mcp tool: %w", err)
	}

	marshaledResult, err := sonic.MarshalString(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal mcp tool result: %w", err)
	}

	// If the MCP server returned an error, we still return the error content as the response
	// to the LLM so it can see what went wrong. The error will be shown to the user via
	// the UI callbacks, but the LLM needs to see the actual error details to continue
	// the conversation appropriately.
	return marshaledResult, nil
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

// shouldExcludeTool determines if a tool should be excluded based on excludedTools
func (m *MCPToolManager) shouldExcludeTool(toolName string, serverConfig config.MCPServerConfig) bool {
	// If excludedTools is specified, exclude tools in the list
	if len(serverConfig.ExcludedTools) > 0 {
		for _, excludedTool := range serverConfig.ExcludedTools {
			if excludedTool == toolName {
				return true
			}
		}
	}

	return false
}

func (m *MCPToolManager) createMCPClient(ctx context.Context, serverName string, serverConfig config.MCPServerConfig) (client.MCPClient, error) {
	if serverConfig.Command != "" {
		// STDIO client
		env := make([]string, 0, len(serverConfig.Env))
		for k, v := range serverConfig.Env {
			env = append(env, fmt.Sprintf("%s=%v", k, v))
		}

		return client.NewStdioMCPClient(serverConfig.Command, env, serverConfig.Args...)
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
