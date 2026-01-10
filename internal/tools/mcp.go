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
	"github.com/eino-contrib/jsonschema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcphost/internal/builtin"
	"github.com/mark3labs/mcphost/internal/config"
)

// MCPToolManager manages MCP (Model Context Protocol) tools and clients across multiple servers.
// It provides a unified interface for loading, managing, and executing tools from various MCP servers,
// including stdio, SSE, streamable HTTP, and built-in server types. The manager handles connection
// pooling, health checks, tool name prefixing to avoid conflicts, and sampling support for LLM interactions.
// Thread-safe for concurrent tool invocations.
type MCPToolManager struct {
	connectionPool *MCPConnectionPool
	tools          []tool.BaseTool
	toolMap        map[string]*toolMapping    // maps prefixed tool names to their server and original name
	model          model.ToolCallingChatModel // LLM model for sampling
	config         *config.Config
	debug          bool
	debugLogger    DebugLogger
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

// NewMCPToolManager creates a new MCP tool manager instance.
// Returns an initialized manager with empty tool collections ready to load tools from MCP servers.
// The manager must be configured with SetModel and LoadTools before use.
func NewMCPToolManager() *MCPToolManager {
	return &MCPToolManager{
		tools:   make([]tool.BaseTool, 0),
		toolMap: make(map[string]*toolMapping),
	}
}

// SetModel sets the LLM model for sampling support.
// The model is used when MCP servers request sampling operations, allowing them to
// leverage the host's LLM capabilities for text generation tasks.
// This method should be called before LoadTools if any MCP servers require sampling support.
func (m *MCPToolManager) SetModel(model model.ToolCallingChatModel) {
	m.model = model
}

// SetDebugLogger sets the debug logger for the tool manager.
// The logger will be used to output detailed debugging information about MCP connections,
// tool loading, and execution. If a connection pool exists, it will also be configured
// to use the same logger for consistent debugging output.
func (m *MCPToolManager) SetDebugLogger(logger DebugLogger) {
	m.debugLogger = logger
	if m.connectionPool != nil {
		m.connectionPool.SetDebugLogger(logger)
	}
}

// samplingHandler implements the MCP sampling handler interface
type samplingHandler struct {
	model model.ToolCallingChatModel
}

// CreateMessage handles sampling requests from MCP servers by forwarding them to the configured LLM model.
// It converts MCP message formats to eino message formats, invokes the model for generation,
// and converts the response back to MCP format. Returns an error if no model is available
// or if generation fails.
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

// LoadTools loads tools from all configured MCP servers based on the provided configuration.
// It initializes the connection pool, connects to each configured server, and loads their tools.
// Tools from different servers are prefixed with the server name to avoid naming conflicts.
// Returns an error only if all configured servers fail to load; partial failures are logged as warnings.
// This method is thread-safe and idempotent.
func (m *MCPToolManager) LoadTools(ctx context.Context, config *config.Config) error {
	// Initialize connection pool
	m.config = config
	m.debug = config.Debug
	if m.debugLogger == nil {
		m.debugLogger = NewSimpleDebugLogger(config.Debug)
	}
	m.connectionPool = NewMCPConnectionPool(DefaultConnectionPoolConfig(), m.model, config.Debug)
	m.connectionPool.SetDebugLogger(m.debugLogger)

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
	m.debugLogConnectionInfo(serverName, serverConfig)

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

		// Fix for JSON Schema draft-07 vs draft-04 compatibility:
		// Chrome DevTools MCP uses draft-07 where exclusiveMinimum/exclusiveMaximum are numbers,
		// but kin-openapi (OpenAPI 3.0) expects them as booleans (draft-04 format).
		// Pre-process the schema to convert numeric exclusive bounds to boolean format.
		marshaledInputSchema = convertExclusiveBoundsToBoolean(marshaledInputSchema)

		inputSchema := &jsonschema.Schema{}
		err = sonic.Unmarshal(marshaledInputSchema, inputSchema)
		if err != nil {
			return fmt.Errorf("conv mcp tool input schema fail(unmarshal): %w, tool name: %s", err, mcpTool.Name)
		}

		// Fix for issue #89: Ensure object schemas have a properties field
		// OpenAI function calling requires object schemas to have a "properties" field
		// even if it's empty, otherwise it throws "object schema missing properties" error
		if inputSchema.Type == "object" && inputSchema.Properties == nil {
			inputSchema.Properties = jsonschema.NewProperties()
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
				ParamsOneOf: schema.NewParamsOneOfByJSONSchema(inputSchema),
			},
			mapping: mapping,
		}

		m.tools = append(m.tools, einoTool)
	}

	return nil
}

// Info returns the tool information including name, description, and parameter schema.
// This method implements the eino tool.BaseTool interface.
// The returned ToolInfo contains the prefixed tool name to ensure uniqueness across servers.
func (t *mcpToolImpl) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

// InvokableRun executes the tool by mapping the prefixed name back to the original tool name and server.
// It retrieves a healthy connection from the pool, invokes the tool on the appropriate MCP server,
// and returns the result as a JSON string. The method handles connection errors by marking
// connections as unhealthy in the pool for automatic recovery on subsequent requests.
// Thread-safe for concurrent invocations.
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

// GetTools returns all loaded tools from all configured MCP servers.
// Tools are returned with their prefixed names (serverName__toolName) to ensure uniqueness.
// The returned slice is a copy and can be safely modified by the caller.
func (m *MCPToolManager) GetTools() []tool.BaseTool {
	return m.tools
}

// GetLoadedServerNames returns the names of all successfully loaded MCP servers.
// This includes servers that are currently connected and have had their tools loaded,
// regardless of their current health status. Useful for debugging and status reporting.
func (m *MCPToolManager) GetLoadedServerNames() []string {
	var names []string
	for serverName := range m.connectionPool.GetClients() {
		names = append(names, serverName)
	}
	return names
}

// Close closes all MCP client connections and cleans up resources.
// This method should be called when the tool manager is no longer needed to ensure
// proper cleanup of stdio processes, network connections, and other resources.
// It is safe to call Close multiple times.
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
func (m *MCPToolManager) debugLogConnectionInfo(serverName string, serverConfig config.MCPServerConfig) {
	if m.debugLogger == nil || !m.debugLogger.IsDebugEnabled() {
		return
	}

	m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] Connecting to MCP server: %s", serverName))
	m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] Transport type: %s", serverConfig.GetTransportType()))

	switch serverConfig.GetTransportType() {
	case "stdio":
		if len(serverConfig.Command) > 0 {
			m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] Command: %s %v", serverConfig.Command[0], serverConfig.Command[1:]))
		}
		if len(serverConfig.Environment) > 0 {
			m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] Environment variables: %d", len(serverConfig.Environment)))
		}
	case "sse", "streamable":
		m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] URL: %s", serverConfig.URL))
		if len(serverConfig.Headers) > 0 {
			m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] Headers: %v", serverConfig.Headers))
		}
	}
}

// convertExclusiveBoundsToBoolean converts JSON Schema draft-07 style exclusive bounds
// (where exclusiveMinimum/exclusiveMaximum are numbers) to draft-04 style
// (where they are booleans that modify minimum/maximum).
// This enables compatibility with kin-openapi which uses OpenAPI 3.0 (draft-04 based) schemas.
func convertExclusiveBoundsToBoolean(schemaJSON []byte) []byte {
	var data map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &data); err != nil {
		return schemaJSON // Return unchanged on error
	}

	convertSchemaRecursive(data)

	result, err := json.Marshal(data)
	if err != nil {
		return schemaJSON // Return unchanged on error
	}
	return result
}

// convertSchemaRecursive recursively processes a schema map and converts
// numeric exclusiveMinimum/exclusiveMaximum to boolean format.
func convertSchemaRecursive(schema map[string]interface{}) {
	// Convert exclusiveMinimum if it's a number
	if exMin, ok := schema["exclusiveMinimum"]; ok {
		if num, isNum := exMin.(float64); isNum {
			// JSON Schema draft-07: exclusiveMinimum is the limit value
			// Convert to draft-04: set minimum = value, exclusiveMinimum = true
			schema["minimum"] = num
			schema["exclusiveMinimum"] = true
		}
	}

	// Convert exclusiveMaximum if it's a number
	if exMax, ok := schema["exclusiveMaximum"]; ok {
		if num, isNum := exMax.(float64); isNum {
			// JSON Schema draft-07: exclusiveMaximum is the limit value
			// Convert to draft-04: set maximum = value, exclusiveMaximum = true
			schema["maximum"] = num
			schema["exclusiveMaximum"] = true
		}
	}

	// Recursively process properties
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for _, prop := range props {
			if propSchema, ok := prop.(map[string]interface{}); ok {
				convertSchemaRecursive(propSchema)
			}
		}
	}

	// Recursively process items (for arrays)
	if items, ok := schema["items"].(map[string]interface{}); ok {
		convertSchemaRecursive(items)
	}

	// Recursively process additionalProperties
	if addProps, ok := schema["additionalProperties"].(map[string]interface{}); ok {
		convertSchemaRecursive(addProps)
	}

	// Recursively process allOf, anyOf, oneOf
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		if arr, ok := schema[key].([]interface{}); ok {
			for _, item := range arr {
				if itemSchema, ok := item.(map[string]interface{}); ok {
					convertSchemaRecursive(itemSchema)
				}
			}
		}
	}

	// Recursively process not
	if not, ok := schema["not"].(map[string]interface{}); ok {
		convertSchemaRecursive(not)
	}
}
