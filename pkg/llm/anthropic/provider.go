package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcphost/pkg/llm"
)

type Provider struct {
	client *Client
	model  string
}

func NewProvider(apiKey string) *Provider {
	return &Provider{
		client: NewClient(apiKey),
		model:  "claude-3-5-sonnet-20240620",
	}
}

func (p *Provider) CreateMessage(
	ctx context.Context,
	prompt string,
	messages []llm.Message,
	tools []llm.Tool,
) (llm.Message, error) {
	// Convert generic messages to Anthropic format
	anthropicMessages := make([]MessageParam, 0, len(messages))

	for _, msg := range messages {
		content := []ContentBlock{{
			Type: "text",
			Text: strings.TrimSpace(msg.GetContent()),
		}}

		// Add tool calls if present
		for _, call := range msg.GetToolCalls() {
			input, _ := json.Marshal(call.GetArguments())
			content = append(content, ContentBlock{
				Type:  "tool_use",
				ID:    call.GetID(),
				Name:  call.GetName(),
				Input: input,
			})
		}

		// Handle tool responses
		if msg.IsToolResponse() {
			content = []ContentBlock{{
				Type:      "tool_result",
				ToolUseID: msg.GetToolResponseID(),
				Content: []ContentBlock{{
					Type: "text",
					Text: msg.GetContent(),
				}},
			}}
		}

		if len(content) > 0 {
			anthropicMessages = append(anthropicMessages, MessageParam{
				Role:    msg.GetRole(),
				Content: content,
			})
		}
	}

	// Add the new prompt if provided
	if prompt != "" {
		anthropicMessages = append(anthropicMessages, MessageParam{
			Role: "user",
			Content: []ContentBlock{{
				Type: "text",
				Text: prompt,
			}},
		})
	}

	// Convert tools to Anthropic format
	anthropicTools := make([]Tool, len(tools))
	for i, tool := range tools {
		anthropicTools[i] = Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: InputSchema{
				Type:       tool.InputSchema.Type,
				Properties: tool.InputSchema.Properties,
				Required:   tool.InputSchema.Required,
			},
		}
	}

	// Make the API call
	resp, err := p.client.CreateMessage(ctx, CreateRequest{
		Model:     p.model,
		Messages:  anthropicMessages,
		MaxTokens: 4096,
		Tools:     anthropicTools,
	})
	if err != nil {
		return nil, err
	}

	return &Message{Msg: *resp}, nil
}

func (p *Provider) SupportsTools() bool {
	return true
}

func (p *Provider) Name() string {
	return "anthropic"
}

func (p *Provider) CreateToolResponse(
	toolCallID string,
	content interface{},
) (llm.Message, error) {
	// Convert content to string if needed
	var contentStr string
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

	return &Message{
		Msg: APIMessage{
			Role: "assistant",
			Content: []ContentBlock{{
				Type:      "tool_result",
				ToolUseID: toolCallID,
				Content: []ContentBlock{{
					Type: "text",
					Text: contentStr,
				}},
			}},
		},
	}, nil
}
