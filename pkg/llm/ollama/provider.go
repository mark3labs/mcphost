package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcphost/pkg/llm"
	api "github.com/ollama/ollama/api"
)

func boolPtr(b bool) *bool {
	return &b
}

// Provider implements the Provider interface for Ollama
type Provider struct {
	client *api.Client
	model  string
}

// NewProvider creates a new Ollama provider
func NewProvider(model string) (*Provider, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, err
	}
	return &Provider{
		client: client,
		model:  model,
	}, nil
}

func (p *Provider) CreateMessage(
	ctx context.Context,
	prompt string,
	messages []llm.Message,
	tools []llm.Tool,
) (llm.Message, error) {
	log.Debug(
		"creating message",
		"prompt",
		prompt,
		"num_messages",
		len(messages),
		"num_tools",
		len(tools),
	)

	// Convert generic messages to Ollama format
	ollamaMessages := make([]api.Message, 0, len(messages)+1)

	// Add existing messages
	for _, msg := range messages {
		log.Debug("processing message",
			"role", msg.GetRole(),
			"content", msg.GetContent(),
			"is_tool_response", msg.IsToolResponse())

		// Skip completely empty messages
		if msg.GetContent() == "" && len(msg.GetToolCalls()) == 0 &&
			!msg.IsToolResponse() {
			log.Debug("skipping empty message")
			continue
		}

		ollamaMsg := api.Message{
			Role:    msg.GetRole(),
			Content: msg.GetContent(),
		}

		// Add tool calls for assistant messages
		if msg.GetRole() == "assistant" {
			for _, call := range msg.GetToolCalls() {
				if call.GetName() != "" {
					args := call.GetArguments()
					ollamaMsg.ToolCalls = append(
						ollamaMsg.ToolCalls,
						api.ToolCall{
							Function: api.ToolCallFunction{
								Name:      call.GetName(),
								Arguments: args,
							},
						},
					)
					log.Debug("added tool call",
						"name", call.GetName(),
						"arguments", args)
				}
			}
		}

		ollamaMessages = append(ollamaMessages, ollamaMsg)
		log.Debug("added message",
			"role", ollamaMsg.Role,
			"content", ollamaMsg.Content,
			"num_tool_calls", len(ollamaMsg.ToolCalls))
	}

	// Add the new prompt if not empty
	if prompt != "" {
		log.Debug("adding prompt message", "prompt", prompt)
		ollamaMessages = append(ollamaMessages, api.Message{
			Role:    "user",
			Content: prompt,
		})
	}

	// Convert tools to Ollama format
	ollamaTools := make([]api.Tool, len(tools))
	for i, tool := range tools {
		ollamaTools[i] = api.Tool{
			Type: "function",
			Function: api.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters: struct {
					Type       string   `json:"type"`
					Required   []string `json:"required"`
					Properties map[string]struct {
						Type        string   `json:"type"`
						Description string   `json:"description"`
						Enum        []string `json:"enum,omitempty"`
					} `json:"properties"`
				}{
					Type:       tool.InputSchema.Type,
					Required:   tool.InputSchema.Required,
					Properties: convertProperties(tool.InputSchema.Properties),
				},
			},
		}
	}

	var response api.Message
	log.Debug("sending chat request",
		"model", p.model,
		"num_messages", len(ollamaMessages),
		"num_tools", len(ollamaTools))

	err := p.client.Chat(ctx, &api.ChatRequest{
		Model:    p.model,
		Messages: ollamaMessages,
		Tools:    ollamaTools,
		Stream:   boolPtr(false),
	}, func(r api.ChatResponse) error {
		if r.Done {
			response = r.Message
			log.Debug("received final response",
				"role", response.Role,
				"content", response.Content,
				"num_tool_calls", len(response.ToolCalls))
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &OllamaMessage{Message: response}, nil
}

func (p *Provider) SupportsTools() bool {
	// Check if model supports function calling
	resp, err := p.client.Show(context.Background(), &api.ShowRequest{
		Model: p.model,
	})
	if err != nil {
		return false
	}
	return strings.Contains(resp.Modelfile, "<tools>")
}

func (p *Provider) Name() string {
	return "ollama"
}

func (p *Provider) CreateToolResponse(
	toolCallID string,
	content interface{},
) (llm.Message, error) {
	log.Debug("creating tool response",
		"toolCallID", toolCallID,
		"content_type", fmt.Sprintf("%T", content),
		"content", content)

	contentStr := ""
	switch v := content.(type) {
	case string:
		contentStr = v
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("error marshaling tool response: %w", err)
		}
		contentStr = string(bytes)
	}

	// Create message with explicit tool role
	msg := &OllamaMessage{
		Message: api.Message{
			Role:    "tool", // Explicitly set role to "tool"
			Content: contentStr,
			// No need to set ToolCalls for a tool response
		},
		ToolCallID: toolCallID,
	}

	log.Debug("created tool response message",
		"role", msg.GetRole(),
		"content", msg.GetContent(),
		"tool_call_id", msg.GetToolResponseID())

	return msg, nil
}

// Helper function to convert properties to Ollama's format
func convertProperties(props map[string]interface{}) map[string]struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
} {
	result := make(map[string]struct {
		Type        string   `json:"type"`
		Description string   `json:"description"`
		Enum        []string `json:"enum,omitempty"`
	})

	for name, prop := range props {
		if propMap, ok := prop.(map[string]interface{}); ok {
			prop := struct {
				Type        string   `json:"type"`
				Description string   `json:"description"`
				Enum        []string `json:"enum,omitempty"`
			}{
				Type:        getString(propMap, "type"),
				Description: getString(propMap, "description"),
			}

			// Handle enum if present
			if enumRaw, ok := propMap["enum"].([]interface{}); ok {
				for _, e := range enumRaw {
					if str, ok := e.(string); ok {
						prop.Enum = append(prop.Enum, str)
					}
				}
			}

			result[name] = prop
		}
	}
	return result
}

// Helper function to safely get string values from map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
