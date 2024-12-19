package llm

import "context"

// Message represents a message in the conversation
type Message interface {
	// GetRole returns the role of the message sender (e.g., "user", "assistant", "system")
	GetRole() string
	
	// GetContent returns the text content of the message
	GetContent() string
	
	// GetToolCalls returns any tool calls made in this message
	GetToolCalls() []ToolCall
	
	// GetUsage returns token usage statistics if available
	GetUsage() (input int, output int)
	
	// GetToolCallID returns the ID of the tool call this message is responding to (for tool responses)
	GetToolCallID() string
}

// ToolCall represents a tool invocation
type ToolCall interface {
	// GetName returns the tool's name
	GetName() string
	
	// GetArguments returns the arguments passed to the tool
	GetArguments() map[string]interface{}
	
	// GetID returns the unique identifier for this tool call
	GetID() string
}

// Tool represents a tool definition
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema Schema `json:"input_schema"`
}

// Schema defines the input parameters for a tool
type Schema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string              `json:"required"`
}

// Provider defines the interface for LLM providers
type Provider interface {
	// CreateMessage sends a message to the LLM and returns the response
	CreateMessage(ctx context.Context, prompt string, messages []Message, tools []Tool) (Message, error)
	
	// CreateToolResponse creates a message representing a tool response
	CreateToolResponse(toolCallID string, content interface{}) (Message, error)
	
	// SupportsTools returns whether this provider supports tool/function calling
	SupportsTools() bool
	
	// Name returns the provider's name
	Name() string
}
