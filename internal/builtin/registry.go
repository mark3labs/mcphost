package builtin

import (
	"fmt"
	"os"

	"github.com/cloudwego/eino/components/model"
	"github.com/mark3labs/mcp-filesystem-server/filesystemserver"
	"github.com/mark3labs/mcp-go/server"
)

// BuiltinServerWrapper wraps an external MCP server for builtin use, providing
// a consistent interface for all builtin servers regardless of their implementation.
type BuiltinServerWrapper struct {
	server *server.MCPServer
}

// Initialize initializes the wrapped server. For builtin servers, this is typically
// a no-op as the server is initialized during creation. Returns an error if
// initialization fails.
func (w *BuiltinServerWrapper) Initialize() error {
	// The server is already initialized when created
	return nil
}

// GetServer returns the wrapped MCP server instance that can be used to handle
// tool calls and other MCP protocol operations.
func (w *BuiltinServerWrapper) GetServer() *server.MCPServer {
	return w.server
}

// Registry holds all available builtin servers and their factory functions.
// It provides a centralized registry for creating instances of builtin MCP servers
// with their respective configurations.
type Registry struct {
	servers map[string]func(options map[string]any, model model.ToolCallingChatModel) (*BuiltinServerWrapper, error)
}

// NewRegistry creates a new builtin server registry with all available builtin
// servers registered. The registry includes filesystem (fs), bash, todo, fetch,
// and HTTP servers.
func NewRegistry() *Registry {
	r := &Registry{
		servers: make(map[string]func(options map[string]any, model model.ToolCallingChatModel) (*BuiltinServerWrapper, error)),
	}

	// Register builtin servers
	r.registerFilesystemServer()
	r.registerBashServer()
	r.registerTodoServer()
	r.registerFetchServer()
	r.registerHTTPServer()

	return r
}

// CreateServer creates a new instance of a builtin server by name. The options
// parameter provides server-specific configuration, and the model parameter provides
// an optional LLM for AI-powered features. Returns an error if the server name
// is unknown or if creation fails.
func (r *Registry) CreateServer(name string, options map[string]any, model model.ToolCallingChatModel) (*BuiltinServerWrapper, error) {
	factory, exists := r.servers[name]
	if !exists {
		return nil, fmt.Errorf("unknown builtin server: %s", name)
	}

	return factory(options, model)
}

// ListServers returns a list of all available builtin server names that can be
// created using CreateServer. The order of names is not guaranteed.
func (r *Registry) ListServers() []string {
	names := make([]string, 0, len(r.servers))
	for name := range r.servers {
		names = append(names, name)
	}
	return names
}

// registerFilesystemServer registers the filesystem server
func (r *Registry) registerFilesystemServer() {
	r.servers["fs"] = func(options map[string]any, model model.ToolCallingChatModel) (*BuiltinServerWrapper, error) {
		// Extract allowed directories from options
		var allowedDirs []string
		if dirs, ok := options["allowed_directories"]; ok {
			switch v := dirs.(type) {
			case []string:
				allowedDirs = v
			case []any:
				allowedDirs = make([]string, len(v))
				for i, dir := range v {
					if s, ok := dir.(string); ok {
						allowedDirs[i] = s
					} else {
						return nil, fmt.Errorf("allowed_directories must be an array of strings")
					}
				}
			case string:
				allowedDirs = []string{v}
			default:
				return nil, fmt.Errorf("allowed_directories must be a string or array of strings")
			}
		} else {
			// Default to current working directory if no directories specified
			cwd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to get current working directory: %v", err)
			}
			allowedDirs = []string{cwd}
		}

		// Create the filesystem server
		server, err := filesystemserver.NewFilesystemServer(allowedDirs)
		if err != nil {
			return nil, fmt.Errorf("failed to create filesystem server: %v", err)
		}

		return &BuiltinServerWrapper{server: server}, nil
	}
}

// registerBashServer registers the bash server
func (r *Registry) registerBashServer() {
	r.servers["bash"] = func(options map[string]any, model model.ToolCallingChatModel) (*BuiltinServerWrapper, error) {
		// Create the bash server
		server, err := NewBashServer()
		if err != nil {
			return nil, fmt.Errorf("failed to create bash server: %v", err)
		}

		return &BuiltinServerWrapper{server: server}, nil
	}
}

// registerTodoServer registers the todo server
func (r *Registry) registerTodoServer() {
	r.servers["todo"] = func(options map[string]any, model model.ToolCallingChatModel) (*BuiltinServerWrapper, error) {
		// Create the todo server
		server, err := NewTodoServer()
		if err != nil {
			return nil, fmt.Errorf("failed to create todo server: %v", err)
		}

		return &BuiltinServerWrapper{server: server}, nil
	}
}

// registerFetchServer registers the fetch server
func (r *Registry) registerFetchServer() {
	r.servers["fetch"] = func(options map[string]any, model model.ToolCallingChatModel) (*BuiltinServerWrapper, error) {
		// Create the fetch server
		server, err := NewFetchServer()
		if err != nil {
			return nil, fmt.Errorf("failed to create fetch server: %v", err)
		}

		return &BuiltinServerWrapper{server: server}, nil
	}
}

// registerHTTPServer registers the HTTP server
func (r *Registry) registerHTTPServer() {
	r.servers["http"] = func(options map[string]any, model model.ToolCallingChatModel) (*BuiltinServerWrapper, error) {
		// Create the HTTP server
		server, err := NewHTTPServer(model)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP server: %v", err)
		}

		return &BuiltinServerWrapper{server: server}, nil
	}
}
