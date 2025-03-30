package google

import (
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/mark3labs/mcphost/pkg/llm"
)

type ToolCall struct {
	genai.FunctionCall

	toolCallID int
}

func (t *ToolCall) GetName() string {
	return t.Name
}

func (t *ToolCall) GetArguments() map[string]any {
	return t.Args
}

func (t *ToolCall) GetID() string {
	return fmt.Sprintf("Tool<%d>", t.toolCallID)
}

type Message struct {
	*genai.Candidate

	toolCallID int
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
	for i, call := range m.Candidate.FunctionCalls() {
		calls = append(calls, &ToolCall{call, m.toolCallID + i})
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

func (m *Message) GetToolResponseID() string {
	return fmt.Sprintf("Tool<%d>", m.toolCallID)
}

func (m *Message) GetUsage() (input int, output int) {
	return 0, 0
}
