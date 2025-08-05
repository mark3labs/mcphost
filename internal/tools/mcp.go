package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
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
	connectionPool *MCPConnectionPool
	tools          []tool.BaseTool
	toolMap        map[string]*toolMapping    // maps prefixed tool names to their server and original name
	model          model.ToolCallingChatModel // LLM model for sampling
	config         *config.Config
}

// toolMapping stores the mapping between prefixed tool names and their original details
type toolMapping struct {
	serverName   string
	originalName string
	serverConfig config.MCPServerConfig
	manager      *MCPToolManager
}

// mcpToolImpl implements the eino tool interface with server prefixing
type mcpToolImpl struct {
	info    *schema.ToolInfo
	mapping *toolMapping
}

// NewMCPToolManager creates a new MCP tool manager
func NewMCPToolManager() *MCPToolManager {
	return &MCPToolManager{
		tools:   make([]tool.BaseTool, 0),
		toolMap: make(map[string]*toolMapping),
	}
}

// SetModel sets the LLM model for sampling support
func (m *MCPToolManager) SetModel(model model.ToolCallingChatModel) {
	m.model = model
}

// samplingHandler implements the MCP sampling handler interface
type samplingHandler struct {
	model model.ToolCallingChatModel
}

// CreateMessage handles sampling requests from MCP servers
func (h *samplingHandler) CreateMessage(ctx context.Context, request mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
	if h.model == nil {
		return nil, fmt.Errorf("no model available for sampling")
	}

	// Convert MCP messages to eino messages
	var messages []*schema.Message

	// Add system message if provided
	if request.SystemPrompt != "" {
		messages = append(messages, schema.SystemMessage(request.SystemPrompt))
	}

	// Convert sampling messages
	for _, msg := range request.Messages {
		// Extract text content
		var content string
		if textContent, ok := msg.Content.(mcp.TextContent); ok {
			content = textContent.Text
		} else {
			content = fmt.Sprintf("%v", msg.Content)
		}

		switch msg.Role {
		case mcp.RoleUser:
			messages = append(messages, schema.UserMessage(content))
		case mcp.RoleAssistant:
			messages = append(messages, schema.AssistantMessage(content, nil))
		default:
			messages = append(messages, schema.UserMessage(content)) // Default to user
		}
	}

	// Generate response using the model (no config options for now)
	response, err := h.model.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("model generation failed: %w", err)
	}

	// Convert response back to MCP format
	result := &mcp.CreateMessageResult{
		Model:      "mcphost-model", // Generic model name
		StopReason: "endTurn",
	}
	result.SamplingMessage = mcp.SamplingMessage{
		Role: mcp.RoleAssistant,
		Content: mcp.TextContent{
			Type: "text",
			Text: response.Content,
		},
	}

	return result, nil
}

// LoadTools loads tools from MCP servers based on configuration
func (m *MCPToolManager) LoadTools(ctx context.Context, config *config.Config) error {
	// Initialize connection pool
	m.config = config
	m.connectionPool = NewMCPConnectionPool(DefaultConnectionPoolConfig(), m.model)

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
	// Add debug logging
	debugLogConnectionInfo(serverName, serverConfig)

	// Get connection from pool
	conn, err := m.connectionPool.GetConnection(ctx, serverName, serverConfig)
	if err != nil {
		return fmt.Errorf("failed to get connection from pool: %v", err)
	}

	// Get tools from this server
	listResults, err := conn.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		// Handle connection error
		m.connectionPool.HandleConnectionError(serverName, err)
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

		// Fix for issue #89: Ensure object schemas have a properties field
		// OpenAI function calling requires object schemas to have a "properties" field
		// even if it's empty, otherwise it throws "object schema missing properties" error
		if inputSchema.Type == "object" && inputSchema.Properties == nil {
			inputSchema.Properties = make(openapi3.Schemas)
		}

		// Create prefixed tool name
		prefixedName := fmt.Sprintf("%s__%s", serverName, mcpTool.Name)

		// Create tool mapping
		mapping := &toolMapping{
			serverName:   serverName,
			originalName: mcpTool.Name,
			serverConfig: serverConfig,
			manager:      m,
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
	// Handle empty or invalid JSON arguments
	var arguments any
	if argumentsInJSON == "" || argumentsInJSON == "{}" {
		arguments = nil
	} else {
		// Validate that argumentsInJSON is valid JSON before using it
		var temp any
		if err := json.Unmarshal([]byte(argumentsInJSON), &temp); err != nil {
			return "", fmt.Errorf("invalid JSON arguments: %w", err)
		}
		arguments = json.RawMessage(argumentsInJSON)
	}

	// Get connection from pool for this server with health check
	conn, err := t.mapping.manager.connectionPool.GetConnectionWithHealthCheck(ctx, t.mapping.serverName, t.mapping.serverConfig)
	if err != nil {
		return "", fmt.Errorf("failed to get healthy connection from pool: %w", err)
	}

	result, err := conn.client.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: struct {
			Name      string    `json:"name"`
			Arguments any       `json:"arguments,omitempty"`
			Meta      *mcp.Meta `json:"_meta,omitempty"`
		}{
			Name:      t.mapping.originalName, // Use original name, not prefixed
			Arguments: arguments,
		},
	})
	if err != nil {
		// Handle connection error in pool
		t.mapping.manager.connectionPool.HandleConnectionError(t.mapping.serverName, err)
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

// GetLoadedServerNames returns the names of successfully loaded MCP servers
func (m *MCPToolManager) GetLoadedServerNames() []string {
	var names []string
	for serverName := range m.connectionPool.GetClients() {
		names = append(names, serverName)
	}
	return names
}

// Close closes all MCP clients
func (m *MCPToolManager) Close() error {
	return m.connectionPool.Close()
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

		// Create stdio transport
		stdioTransport := transport.NewStdio(command, env, args...)

		stdioClient := client.NewClient(stdioTransport)

		// Start the transport
		if err := stdioTransport.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start stdio transport: %v", err)
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

	// Create the builtin server, passing the model for servers that need it
	builtinServer, err := registry.CreateServer(serverConfig.Name, serverConfig.Options, m.model)
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

// debugLogConnectionInfo logs detailed connection information for debugging
func debugLogConnectionInfo(serverName string, serverConfig config.MCPServerConfig) {
	fmt.Printf("🔍 [DEBUG] Connecting to MCP server: %s\n", serverName)
	fmt.Printf("🔍 [DEBUG] Transport type: %s\n", serverConfig.GetTransportType())

	switch serverConfig.GetTransportType() {
	case "stdio":
		if len(serverConfig.Command) > 0 {
			fmt.Printf("🔍 [DEBUG] Command: %s %v\n", serverConfig.Command[0], serverConfig.Command[1:])
		}
		if len(serverConfig.Environment) > 0 {
			fmt.Printf("🔍 [DEBUG] Environment variables: %d\n", len(serverConfig.Environment))
		}
	case "sse", "streamable":
		fmt.Printf("🔍 [DEBUG] URL: %s\n", serverConfig.URL)
		if len(serverConfig.Headers) > 0 {
			fmt.Printf("🔍 [DEBUG] Headers: %v\n", serverConfig.Headers)
		}
	}
}
