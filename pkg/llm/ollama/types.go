package ollama

import (
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcphost/pkg/llm"
	api "github.com/ollama/ollama/api"
)

// OllamaMessage adapts Ollama's message format to our Message interface
type OllamaMessage struct {
	Message    api.Message
	ToolCallID string // Store tool call ID separately since Ollama API doesn't have this field
}

func (m *OllamaMessage) GetRole() string {
	return m.Message.Role
}

func (m *OllamaMessage) GetContent() string {
	// For tool responses and regular messages, just return the content string
	return strings.TrimSpace(m.Message.Content)
}

func (m *OllamaMessage) GetToolCalls() []llm.ToolCall {
	var calls []llm.ToolCall
	for _, call := range m.Message.ToolCalls {
		calls = append(calls, NewOllamaToolCall(call))
	}
	return calls
}

func (m *OllamaMessage) GetUsage() (int, int) {
	return 0, 0 // Ollama doesn't provide token usage info
}

func (m *OllamaMessage) IsToolResponse() bool {
	return m.Message.Role == "tool"
}

func (m *OllamaMessage) GetToolResponseID() string {
	return m.ToolCallID
}

// OllamaToolCall adapts Ollama's tool call format
type OllamaToolCall struct {
	call api.ToolCall
	id   string // Store a unique ID for the tool call
}

func NewOllamaToolCall(call api.ToolCall) *OllamaToolCall {
	return &OllamaToolCall{
		call: call,
		id: fmt.Sprintf(
			"tc_%s_%d",
			call.Function.Name,
			time.Now().UnixNano(),
		),
	}
}

func (t *OllamaToolCall) GetName() string {
	return t.call.Function.Name
}

func (t *OllamaToolCall) GetArguments() map[string]interface{} {
	return t.call.Function.Arguments
}

func (t *OllamaToolCall) GetID() string {
	return t.id
}
