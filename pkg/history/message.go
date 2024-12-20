package history

import (
	"encoding/json"
	"strings"

	"github.com/mark3labs/mcphost/pkg/llm"
)

// HistoryMessage implements the llm.Message interface for stored messages
type HistoryMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

func (m *HistoryMessage) GetRole() string {
	return m.Role
}

func (m *HistoryMessage) GetContent() string {
	// Concatenate all text content blocks
	var content string
	for _, block := range m.Content {
		if block.Type == "text" {
			content += block.Text + " "
		}
	}
	return strings.TrimSpace(content)
}

func (m *HistoryMessage) GetToolCalls() []llm.ToolCall {
	var calls []llm.ToolCall
	for _, block := range m.Content {
		if block.Type == "tool_use" {
			calls = append(calls, &HistoryToolCall{
				id:   block.ID,
				name: block.Name,
				args: block.Input,
			})
		}
	}
	return calls
}

func (m *HistoryMessage) IsToolResponse() bool {
	for _, block := range m.Content {
		if block.Type == "tool_result" {
			return true
		}
	}
	return false
}

func (m *HistoryMessage) GetToolResponseID() string {
	for _, block := range m.Content {
		if block.Type == "tool_result" {
			return block.ToolUseID
		}
	}
	return ""
}

func (m *HistoryMessage) GetUsage() (int, int) {
	return 0, 0 // History doesn't track usage
}

// HistoryToolCall implements llm.ToolCall for stored tool calls
type HistoryToolCall struct {
	id   string
	name string
	args json.RawMessage
}

func (t *HistoryToolCall) GetID() string {
	return t.id
}

func (t *HistoryToolCall) GetName() string {
	return t.name
}

func (t *HistoryToolCall) GetArguments() map[string]interface{} {
	var args map[string]interface{}
	if err := json.Unmarshal(t.args, &args); err != nil {
		return make(map[string]interface{})
	}
	return args
}

// ContentBlock represents a block of content in a message
type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Content   interface{}     `json:"content,omitempty"`
}
