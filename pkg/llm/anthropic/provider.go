package anthropic

import (
	"context"
	"encoding/json"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcphost/pkg/history"
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
		// content := []ContentBlock{{
		// 	Type: "text",
		// 	Text: strings.TrimSpace(msg.GetContent()),
		// }}
		content := []ContentBlock{}

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
			if historyMsg, ok := msg.(*history.HistoryMessage); ok {
				log.Debug(
					"processing history message content",
					"content",
					historyMsg.Content,
				)
				for _, block := range historyMsg.Content {
					if block.Type == "tool_result" {
						toolBlock := ContentBlock{
							Type:      "tool_result",
							ToolUseID: block.ToolUseID,
							Content:   block.Content,
						}
						content = append(content, toolBlock)
						log.Debug(
							"created tool result block",
							"block",
							toolBlock,
						)
					}
				}
			} else {
				// Fallback to simple content handling
				log.Debug("handling non-history tool response",
					"id", msg.GetToolResponseID(),
					"content", msg.GetContent())

				content = []ContentBlock{{
					Type:      "tool_result",
					ToolUseID: msg.GetToolResponseID(),
					Content:   msg.GetContent(),
				}}
				log.Debug("created fallback tool result block", "block", content[0])
			}
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
	log.Debug("creating tool response",
		"toolCallID", toolCallID,
		"content", content)

	// If content is already a string, use it directly
	if contentStr, ok := content.(string); ok {
		msg := &Message{
			Msg: APIMessage{
				Role: "tool",
				Content: []ContentBlock{{
					Type:      "tool_result",
					ToolUseID: toolCallID,
					Content:   content,
					Text:      contentStr,
				}},
			},
		}
		log.Debug("created tool response message", "message", msg)
		return msg, nil
	}

	// For structured content, preserve both the original structure and a string representation
	contentJSON, err := json.Marshal(content)
	if err != nil {
		log.Warn("failed to marshal content to JSON", "error", err)
		// Still continue with the original content
	}

	msg := &Message{
		Msg: APIMessage{
			Role: "tool",
			Content: []ContentBlock{{
				Type:      "tool_result",
				ToolUseID: toolCallID,
				Content:   content,             // Preserve original structure
				Text:      string(contentJSON), // Add string representation
			}},
		},
	}

	log.Debug("created tool response message", "message", msg)
	return msg, nil
}
