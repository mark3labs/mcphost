package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcphost/internal/builtin"
	"github.com/mark3labs/mcphost/internal/config"
)

// ConnectionPoolConfig configuration for connection pool
type ConnectionPoolConfig struct {
	MaxIdleTime         time.Duration
	MaxRetries          int
	HealthCheckInterval time.Duration
	MaxErrorCount       int
	ReconnectDelay      time.Duration
}

// DefaultConnectionPoolConfig returns default configuration
func DefaultConnectionPoolConfig() *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		MaxIdleTime:         5 * time.Minute,
		MaxRetries:          3,
		HealthCheckInterval: 30 * time.Second,
		MaxErrorCount:       3,
		ReconnectDelay:      2 * time.Second,
	}
}

// MCPConnection represents an MCP connection
type MCPConnection struct {
	client       client.MCPClient
	serverName   string
	serverConfig config.MCPServerConfig
	lastUsed     time.Time
	isHealthy    bool
	errorCount   int
	lastError    error
	mu           sync.RWMutex
}

// MCPConnectionPool manages MCP connections
type MCPConnectionPool struct {
	connections map[string]*MCPConnection
	config      *ConnectionPoolConfig
	mu          sync.RWMutex
	model       model.ToolCallingChatModel
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewMCPConnectionPool creates a new connection pool
func NewMCPConnectionPool(config *ConnectionPoolConfig, model model.ToolCallingChatModel) *MCPConnectionPool {
	if config == nil {
		config = DefaultConnectionPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	pool := &MCPConnectionPool{
		connections: make(map[string]*MCPConnection),
		config:      config,
		model:       model,
		ctx:         ctx,
		cancel:      cancel,
	}

	go pool.startHealthCheck()
	return pool
}

// GetConnection gets a connection from the pool
func (p *MCPConnectionPool) GetConnection(ctx context.Context, serverName string, serverConfig config.MCPServerConfig) (*MCPConnection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, exists := p.connections[serverName]; exists {
		conn.mu.RLock()
		isHealthy := conn.isHealthy && time.Since(conn.lastUsed) < p.config.MaxIdleTime
		conn.mu.RUnlock()

		if isHealthy {
			conn.mu.Lock()
			conn.lastUsed = time.Now()
			conn.mu.Unlock()
			fmt.Printf("🔄 [POOL] Reusing connection for %s\n", serverName)
			return conn, nil
		} else {
			fmt.Printf("🔍 [POOL] Connection %s unhealthy, removing\n", serverName)
			conn.client.Close()
			delete(p.connections, serverName)
		}
	}

	fmt.Printf("🆕 [POOL] Creating new connection for %s\n", serverName)
	conn, err := p.createConnection(ctx, serverName, serverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection for %s: %w", serverName, err)
	}

	p.connections[serverName] = conn
	return conn, nil
}

// GetConnectionWithHealthCheck gets a connection from the pool with proactive health check
func (p *MCPConnectionPool) GetConnectionWithHealthCheck(ctx context.Context, serverName string, serverConfig config.MCPServerConfig) (*MCPConnection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, exists := p.connections[serverName]; exists {
		conn.mu.RLock()
		isHealthy := conn.isHealthy && time.Since(conn.lastUsed) < p.config.MaxIdleTime
		conn.mu.RUnlock()

		if isHealthy {
			// Perform proactive health check before reusing connection
			if p.performHealthCheck(ctx, conn) {
				conn.mu.Lock()
				conn.lastUsed = time.Now()
				conn.mu.Unlock()
				fmt.Printf("✅ [POOL] Reusing healthy connection for %s\n", serverName)
				return conn, nil
			} else {
				fmt.Printf("🔍 [POOL] Connection %s failed health check, removing\n", serverName)
				conn.client.Close()
				delete(p.connections, serverName)
			}
		} else {
			fmt.Printf("🔍 [POOL] Connection %s unhealthy, removing\n", serverName)
			conn.client.Close()
			delete(p.connections, serverName)
		}
	}

	fmt.Printf("🆕 [POOL] Creating new connection for %s\n", serverName)
	conn, err := p.createConnection(ctx, serverName, serverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection for %s: %w", serverName, err)
	}

	p.connections[serverName] = conn
	return conn, nil
}

// performHealthCheck performs a quick health check on the connection
func (p *MCPConnectionPool) performHealthCheck(ctx context.Context, conn *MCPConnection) bool {
	// Create a short timeout context for health check
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try to list tools as a health check - this is a lightweight operation
	_, err := conn.client.ListTools(healthCtx, mcp.ListToolsRequest{})
	if err != nil {
		fmt.Printf("⚠️ [HEALTH_CHECK] Connection %s failed health check: %v\n", conn.serverName, err)
		conn.mu.Lock()
		conn.isHealthy = false
		conn.errorCount++
		conn.lastError = err
		conn.mu.Unlock()
		return false
	}

	// Reset error count on successful health check
	conn.mu.Lock()
	conn.errorCount = 0
	conn.lastError = nil
	conn.mu.Unlock()

	return true
}

// createConnection creates a new connection
func (p *MCPConnectionPool) createConnection(ctx context.Context, serverName string, serverConfig config.MCPServerConfig) (*MCPConnection, error) {
	client, err := p.createMCPClient(ctx, serverName, serverConfig)
	if err != nil {
		return nil, err
	}

	if err := p.initializeClient(ctx, client); err != nil {
		client.Close()
		return nil, err
	}

	conn := &MCPConnection{
		client:       client,
		serverName:   serverName,
		serverConfig: serverConfig,
		lastUsed:     time.Now(),
		isHealthy:    true,
		errorCount:   0,
		lastError:    nil,
	}

	fmt.Printf("✅ [POOL] Created connection for %s\n", serverName)
	return conn, nil
}

// createMCPClient creates an MCP client
func (p *MCPConnectionPool) createMCPClient(ctx context.Context, serverName string, serverConfig config.MCPServerConfig) (client.MCPClient, error) {
	transportType := serverConfig.GetTransportType()

	switch transportType {
	case "stdio":
		return p.createStdioClient(ctx, serverConfig)
	case "sse":
		return p.createSSEClient(ctx, serverConfig)
	case "streamable":
		return p.createStreamableClient(ctx, serverConfig)
	case "inprocess":
		return p.createBuiltinClient(ctx, serverName, serverConfig)
	default:
		return nil, fmt.Errorf("unsupported transport type '%s' for server %s", transportType, serverName)
	}
}

// createStdioClient creates a STDIO client
func (p *MCPConnectionPool) createStdioClient(ctx context.Context, serverConfig config.MCPServerConfig) (client.MCPClient, error) {
	var env []string
	var command string
	var args []string

	if len(serverConfig.Command) > 0 {
		command = serverConfig.Command[0]
		if len(serverConfig.Command) > 1 {
			args = serverConfig.Command[1:]
		} else if len(serverConfig.Args) > 0 {
			args = serverConfig.Args
		}
	}

	if serverConfig.Environment != nil {
		for k, v := range serverConfig.Environment {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	stdioTransport := transport.NewStdio(command, env, args...)
	stdioClient := client.NewClient(stdioTransport)

	if err := stdioTransport.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start stdio transport: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	return stdioClient, nil
}

// createSSEClient creates an SSE client
func (p *MCPConnectionPool) createSSEClient(ctx context.Context, serverConfig config.MCPServerConfig) (client.MCPClient, error) {
	var options []transport.ClientOption

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

	if err := sseClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start SSE client: %v", err)
	}

	return sseClient, nil
}

// createStreamableClient creates a Streamable client
func (p *MCPConnectionPool) createStreamableClient(ctx context.Context, serverConfig config.MCPServerConfig) (client.MCPClient, error) {
	var options []transport.StreamableHTTPCOption

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

	if err := streamableClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start streamable HTTP client: %v", err)
	}

	return streamableClient, nil
}

// createBuiltinClient creates a builtin client
func (p *MCPConnectionPool) createBuiltinClient(ctx context.Context, serverName string, serverConfig config.MCPServerConfig) (client.MCPClient, error) {
	registry := builtin.NewRegistry()

	builtinServer, err := registry.CreateServer(serverConfig.Name, serverConfig.Options, p.model)
	if err != nil {
		return nil, fmt.Errorf("failed to create builtin server: %v", err)
	}

	inProcessClient, err := client.NewInProcessClient(builtinServer.GetServer())
	if err != nil {
		return nil, fmt.Errorf("failed to create in-process client: %v", err)
	}

	return inProcessClient, nil
}

// initializeClient initializes the client
func (p *MCPConnectionPool) initializeClient(ctx context.Context, client client.MCPClient) error {
	initCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
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

	fmt.Printf("✅ [POOL] Initialized MCP client\n")
	return nil
}

// startHealthCheck starts the health check routine
func (p *MCPConnectionPool) startHealthCheck() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.checkConnectionsHealth()
		}
	}
}

// checkConnectionsHealth checks the health of all connections
func (p *MCPConnectionPool) checkConnectionsHealth() {
	p.mu.RLock()
	connections := make(map[string]*MCPConnection)
	for k, v := range p.connections {
		connections[k] = v
	}
	p.mu.RUnlock()

	for serverName, conn := range connections {
		conn.mu.Lock()

		if time.Since(conn.lastUsed) > p.config.MaxIdleTime {
			conn.isHealthy = false
			fmt.Printf("🔍 [HEALTH_CHECK] Connection %s marked as unhealthy due to inactivity\n", serverName)
		}

		if conn.errorCount > p.config.MaxErrorCount {
			conn.isHealthy = false
			fmt.Printf("🔍 [HEALTH_CHECK] Connection %s marked as unhealthy due to errors\n", serverName)
		}

		conn.mu.Unlock()
	}
}

// HandleConnectionError handles connection errors
func (p *MCPConnectionPool) HandleConnectionError(serverName string, err error) {
	p.mu.RLock()
	conn, exists := p.connections[serverName]
	p.mu.RUnlock()

	if !exists {
		return
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	conn.errorCount++
	conn.lastError = err

	if isConnectionError(err) {
		conn.isHealthy = false
		fmt.Printf("❌ [POOL] Connection %s marked as unhealthy: %v\n", serverName, err)

		if strings.Contains(err.Error(), "404") {
			fmt.Printf("🔄 [POOL] 404 error for %s, will recreate on next request\n", serverName)
		}
	}
}

// GetConnectionStats returns connection statistics
func (p *MCPConnectionPool) GetConnectionStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]interface{})
	for serverName, conn := range p.connections {
		conn.mu.RLock()
		stats[serverName] = map[string]interface{}{
			"is_healthy":  conn.isHealthy,
			"last_used":   conn.lastUsed,
			"last_error":  conn.lastError,
			"error_count": conn.errorCount,
			"server_name": conn.serverName,
		}
		conn.mu.RUnlock()
	}
	return stats
}

// ServerName returns the server name for this connection
func (c *MCPConnection) ServerName() string {
	return c.serverName
}

// GetClients returns all client names in the pool
func (p *MCPConnectionPool) GetClients() map[string]client.MCPClient {
	p.mu.RLock()
	defer p.mu.RUnlock()

	clients := make(map[string]client.MCPClient)
	for name, conn := range p.connections {
		clients[name] = conn.client
	}
	return clients
}

// Close closes the connection pool
func (p *MCPConnectionPool) Close() error {
	p.cancel()

	p.mu.Lock()
	defer p.mu.Unlock()

	for name, conn := range p.connections {
		if err := conn.client.Close(); err != nil {
			fmt.Printf("⚠️ [POOL] Failed to close connection %s: %v\n", name, err)
		}
	}

	fmt.Printf("🛑 [POOL] Connection pool closed\n")
	return nil
}

// isConnectionError checks if the error is connection-related
func isConnectionError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "Connection not found") ||
		strings.Contains(errStr, "transport error") ||
		strings.Contains(errStr, "request failed with status 404") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "Client.Timeout exceeded")
}
