package huggingface

import (
	"context"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ChatModel wraps the eino-ext OpenAI model for Huggingface
type ChatModel struct {
	wrapped *einoopenai.ChatModel
}

// NewChatModel creates a new Huggingface chat model
func NewChatModel(ctx context.Context, config *einoopenai.ChatModelConfig) (*ChatModel, error) {
	// The underlying provider is OpenAI compatible, so we can reuse the einoopenai.ChatModel
	wrapped, err := einoopenai.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}

	return &ChatModel{
		wrapped: wrapped,
	}, nil
}

// Generate implements model.ChatModel
func (c *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return c.wrapped.Generate(ctx, in, opts...)
}

// Stream implements model.ChatModel
func (c *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return c.wrapped.Stream(ctx, in, opts...)
}

// WithTools implements model.ToolCallingChatModel
func (c *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	wrappedWithTools, err := c.wrapped.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &ChatModel{wrapped: wrappedWithTools.(*einoopenai.ChatModel)}, nil
}

// BindTools implements model.ToolCallingChatModel
func (c *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return c.wrapped.BindTools(tools)
}

// BindForcedTools implements model.ToolCallingChatModel
func (c *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return c.wrapped.BindForcedTools(tools)
}

// GetType implements model.ChatModel
func (c *ChatModel) GetType() string {
	return "Huggingface"
}

// IsCallbacksEnabled implements model.ChatModel
func (c *ChatModel) IsCallbacksEnabled() bool {
	return c.wrapped.IsCallbacksEnabled()
}
