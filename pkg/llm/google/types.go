package google

import (
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/mark3labs/mcphost/pkg/llm"
)

type ToolCall struct {
	genai.FunctionCall
}

func (t *ToolCall) GetName() string {
	return t.Name
}

func (t *ToolCall) GetArguments() map[string]any {
	return t.Args
}

func (t *ToolCall) GetID() string {
	return "TODO"
}

type Message struct {
	*genai.Candidate
}

func (m *Message) GetRole() string {
	return m.Candidate.Content.Role
}

func (m *Message) GetContent() string {
	var sb strings.Builder
	for _, part := range m.Candidate.Content.Parts {
		if text, ok := part.(genai.Text); ok {
			sb.WriteString(string(text))
		}
	}
	return sb.String()
}

func (m *Message) GetToolCalls() []llm.ToolCall {
	var calls []llm.ToolCall
	for _, call := range m.Candidate.FunctionCalls() {
		calls = append(calls, &ToolCall{call})
	}
	return calls
}

func (m *Message) IsToolResponse() bool {
	for _, part := range m.Candidate.Content.Parts {
		if _, ok := part.(*genai.FunctionResponse); ok {
			return true
		}
	}

	return false
}

// GetToolResponseID returns the ID of the tool call this message is responding to
func (m *Message) GetToolResponseID() string {
	return "TODO"
}

// GetUsage returns token usage statistics if available
func (m *Message) GetUsage() (input int, output int) {
	return 0, 0
}
