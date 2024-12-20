package anthropic

import (
	"encoding/json"
	"strings"

	"github.com/mark3labs/mcphost/pkg/llm"
)

type CreateRequest struct {
	Model     string         `json:"model"`
	Messages  []MessageParam `json:"messages"`
	MaxTokens int            `json:"max_tokens"`
	Tools     []Tool         `json:"tools,omitempty"`
}

type MessageParam struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Content   interface{}     `json:"content,omitempty"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"input_schema"`
}

type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

type APIMessage struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Message implements the llm.Message interface
type Message struct {
	Msg APIMessage
}

func (m *Message) GetRole() string {
	return m.Msg.Role
}

func (m *Message) GetContent() string {
	// Concatenate all text content blocks
	var content string
	for _, block := range m.Msg.Content {
		if block.Type == "text" {
			content += block.Text + " "
		}
	}
	return strings.TrimSpace(content)
}

func (m *Message) GetToolCalls() []llm.ToolCall {
	var calls []llm.ToolCall
	for _, block := range m.Msg.Content {
		if block.Type == "tool_use" {
			calls = append(calls, &ToolCall{
				id:   block.ID,
				name: block.Name,
				args: block.Input,
			})
		}
	}
	return calls
}

func (m *Message) IsToolResponse() bool {
	for _, block := range m.Msg.Content {
		if block.Type == "tool_result" {
			return true
		}
	}
	return false
}

func (m *Message) GetToolResponseID() string {
	for _, block := range m.Msg.Content {
		if block.Type == "tool_result" {
			return block.ToolUseID
		}
	}
	return ""
}

func (m *Message) GetUsage() (input int, output int) {
	return m.Msg.Usage.InputTokens, m.Msg.Usage.OutputTokens
}

// ToolCall implements the llm.ToolCall interface
type ToolCall struct {
	id   string
	name string
	args json.RawMessage
}

func (t *ToolCall) GetName() string {
	return t.name
}

func (t *ToolCall) GetArguments() map[string]interface{} {
	var args map[string]interface{}
	if err := json.Unmarshal(t.args, &args); err != nil {
		return make(map[string]interface{})
	}
	return args
}

func (t *ToolCall) GetID() string {
	return t.id
}
