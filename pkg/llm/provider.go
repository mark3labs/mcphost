package llm

import "context"

// Message represents a generic message in the conversation
type Message interface {
	GetRole() string
	GetContent() string
	GetToolCalls() []ToolCall
	GetUsage() (inputTokens, outputTokens int)
}

// ToolCall represents a generic tool call
type ToolCall interface {
	GetName() string
	GetArguments() map[string]interface{}
	GetID() string
}

// ToolResult represents a generic tool result
type ToolResult interface {
	GetToolCallID() string
	GetContent() interface{}
}

// Provider defines the interface that all LLM providers must implement
type Provider interface {
	// CreateMessage sends a message to the LLM and returns the response
	CreateMessage(ctx context.Context, prompt string, messages []Message, tools []Tool) (Message, error)

	// SupportsTools returns whether this provider supports tool/function calling
	SupportsTools() bool

	// Name returns the provider's name
	Name() string
}

// Tool represents a generic tool that can be used by the LLM
type Tool struct {
	Name        string
	Description string
	InputSchema Schema
}

// Schema represents the input schema for a tool
type Schema struct {
	Type       string
	Properties map[string]interface{}
	Required   []string
}
