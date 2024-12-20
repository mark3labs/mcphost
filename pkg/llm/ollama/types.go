package ollama

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
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
	// Handle tool responses
	if m.Message.Role == "tool" {
		log.Debug("processing tool response content",
			"raw_content", m.Message.Content)

		// Try to unmarshal content if it's JSON
		var contentMap []map[string]interface{}
		if err := json.Unmarshal([]byte(m.Message.Content), &contentMap); err == nil {
			log.Debug("successfully unmarshaled JSON content",
				"content_map", contentMap)

			var texts []string
			for _, item := range contentMap {
				if text, ok := item["text"].(interface{}); ok {
					log.Debug("found text field in content",
						"text", text,
						"type", fmt.Sprintf("%T", text))

					switch v := text.(type) {
					case string:
						texts = append(texts, v)
					case []interface{}:
						// Handle array of text items
						log.Debug("processing array of text items",
							"items", v)
						for _, t := range v {
							if str, ok := t.(string); ok {
								texts = append(texts, str)
							}
						}
					}
				}
			}
			if len(texts) > 0 {
				result := strings.TrimSpace(strings.Join(texts, " "))
				log.Debug("extracted text content",
					"result", result)
				return result
			}
		} else {
			log.Debug("failed to unmarshal content as JSON",
				"error", err)
		}
		// Fallback to raw content if not JSON or no text found
		log.Debug("falling back to raw content")
		return strings.TrimSpace(m.Message.Content)
	}

	// For regular messages
	log.Debug("processing regular message content",
		"role", m.Message.Role,
		"content", m.Message.Content)
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
