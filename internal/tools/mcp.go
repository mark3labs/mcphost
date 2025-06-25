package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcphost/internal/builtin"
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
	var loadErrors []string

	for serverName, serverConfig := range config.MCPServers {
		if err := m.loadServerTools(ctx, serverName, serverConfig); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("server %s: %v", serverName, err))
			fmt.Printf("Warning: Failed to load MCP server '%s': %v\n", serverName, err)
			continue
		}
	}

	// If all servers failed to load, return an error
	if len(loadErrors) == len(config.MCPServers) && len(config.MCPServers) > 0 {
		return fmt.Errorf("all MCP servers failed to load: %s", strings.Join(loadErrors, "; "))
	}

	return nil
}

// loadServerTools loads tools from a single MCP server
func (m *MCPToolManager) loadServerTools(ctx context.Context, serverName string, serverConfig config.MCPServerConfig) error {
	client, err := m.createMCPClient(ctx, serverName, serverConfig)
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %v", err)
	}

	m.clients[serverName] = client

	// Initialize the client
	if err := m.initializeClient(ctx, client); err != nil {
		return fmt.Errorf("failed to initialize MCP client: %v", err)
	}

	// Get tools from this server
	listResults, err := client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %v", err)
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
	transportType := serverConfig.GetTransportType()

	switch transportType {
	case "stdio":
		// STDIO client
		var env []string
		var command string
		var args []string

		// Handle command and environment
		if len(serverConfig.Command) > 0 {
			command = serverConfig.Command[0]
			if len(serverConfig.Command) > 1 {
				args = serverConfig.Command[1:]
			} else if len(serverConfig.Args) > 0 {
				// Legacy fallback: Command only has the command, Args has the arguments
				// This handles cases where legacy config conversion didn't work properly
				args = serverConfig.Args
			}
		}

		// Convert environment variables
		if serverConfig.Environment != nil {
			for k, v := range serverConfig.Environment {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
		}

		// Legacy environment support
		if serverConfig.Env != nil {
			for k, v := range serverConfig.Env {
				env = append(env, fmt.Sprintf("%s=%v", k, v))
			}
		}

		stdioClient, err := client.NewStdioMCPClient(command, env, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdio client: %v", err)
		}

		// Add a brief delay to allow the process to start and potentially fail
		time.Sleep(100 * time.Millisecond)

		// TODO: Add process health check here if the mcp-go library exposes process info
		// For now, we rely on the timeout in initializeClient to catch dead processes

		return stdioClient, nil

	case "sse":
		// SSE client
		var options []transport.ClientOption

		// Add headers if specified
		if len(serverConfig.Headers) > 0 {
			headers := make(map[string]string)
			for _, header := range serverConfig.Headers {
				parts := strings.SplitN(header, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					headers[key] = value
				}
			}
			if len(headers) > 0 {
				options = append(options, transport.WithHeaders(headers))
			}
		}

		sseClient, err := client.NewSSEMCPClient(serverConfig.URL, options...)
		if err != nil {
			return nil, err
		}

		// Start the SSE client
		if err := sseClient.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start SSE client: %v", err)
		}

		return sseClient, nil

	case "streamable":
		// Streamable HTTP client
		var options []transport.StreamableHTTPCOption

		// Add headers if specified
		if len(serverConfig.Headers) > 0 {
			headers := make(map[string]string)
			for _, header := range serverConfig.Headers {
				parts := strings.SplitN(header, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					headers[key] = value
				}
			}
			if len(headers) > 0 {
				options = append(options, transport.WithHTTPHeaders(headers))
			}
		}

		streamableClient, err := client.NewStreamableHttpClient(serverConfig.URL, options...)
		if err != nil {
			return nil, err
		}

		// Start the streamable HTTP client
		if err := streamableClient.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start streamable HTTP client: %v", err)
		}

		return streamableClient, nil

	case "inprocess":
		// Builtin server
		return m.createBuiltinClient(ctx, serverName, serverConfig)

	default:
		return nil, fmt.Errorf("unsupported transport type '%s' for server %s", transportType, serverName)
	}
}

func (m *MCPToolManager) initializeClient(ctx context.Context, client client.MCPClient) error {
	// Create a timeout context for initialization to prevent deadlocks
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "mcphost",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	_, err := client.Initialize(initCtx, initRequest)
	if err != nil {
		return fmt.Errorf("initialization timeout or failed: %v", err)
	}
	return nil
}

// createBuiltinClient creates an in-process MCP client for builtin servers
func (m *MCPToolManager) createBuiltinClient(ctx context.Context, serverName string, serverConfig config.MCPServerConfig) (client.MCPClient, error) {
	registry := builtin.NewRegistry()

	// Create the builtin server
	builtinServer, err := registry.CreateServer(serverConfig.Name, serverConfig.Options)
	if err != nil {
		return nil, fmt.Errorf("failed to create builtin server: %v", err)
	}

	// Create an in-process client that wraps the builtin server
	inProcessClient, err := client.NewInProcessClient(builtinServer.GetServer())
	if err != nil {
		return nil, fmt.Errorf("failed to create in-process client: %v", err)
	}

	return inProcessClient, nil
}
