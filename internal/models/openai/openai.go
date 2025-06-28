package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// CustomChatModel wraps the eino-ext OpenAI model with custom tool schema handling
type CustomChatModel struct {
	wrapped *einoopenai.ChatModel
}

// CustomRoundTripper intercepts HTTP requests to fix OpenAI function schemas
type CustomRoundTripper struct {
	wrapped http.RoundTripper
}

// NewCustomChatModel creates a new custom OpenAI chat model
func NewCustomChatModel(ctx context.Context, config *einoopenai.ChatModelConfig) (*CustomChatModel, error) {
	// Create a custom HTTP client that intercepts requests
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{}
	}

	// Wrap the transport to intercept requests
	if config.HTTPClient.Transport == nil {
		config.HTTPClient.Transport = http.DefaultTransport
	}
	config.HTTPClient.Transport = &CustomRoundTripper{
		wrapped: config.HTTPClient.Transport,
	}

	wrapped, err := einoopenai.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}

	return &CustomChatModel{
		wrapped: wrapped,
	}, nil
}

// RoundTrip implements http.RoundTripper to intercept and fix OpenAI requests
func (c *CustomRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only intercept OpenAI chat completions requests
	if !strings.Contains(req.URL.Path, "/chat/completions") {
		return c.wrapped.RoundTrip(req)
	}

	// Read the request body
	if req.Body == nil {
		return c.wrapped.RoundTrip(req)
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return c.wrapped.RoundTrip(req)
	}
	req.Body.Close()

	// Parse the JSON request
	var requestData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &requestData); err != nil {
		// If we can't parse it, just pass it through
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return c.wrapped.RoundTrip(req)
	}

	// Fix function schemas if present
	if tools, ok := requestData["tools"].([]interface{}); ok {
		for _, tool := range tools {
			if toolMap, ok := tool.(map[string]interface{}); ok {
				if function, ok := toolMap["function"].(map[string]interface{}); ok {
					if parameters, ok := function["parameters"].(map[string]interface{}); ok {
						if typeVal, ok := parameters["type"].(string); ok && typeVal == "object" {
							// Check if properties is missing or empty
							if properties, exists := parameters["properties"]; !exists || properties == nil {
								parameters["properties"] = map[string]interface{}{}
							} else if propMap, ok := properties.(map[string]interface{}); ok && len(propMap) == 0 {
								parameters["properties"] = map[string]interface{}{}
							}
						}
					}
				}
			}
		}
	}
	// Marshal the fixed request back to JSON
	fixedBodyBytes, err := json.Marshal(requestData)
	if err != nil {
		// If we can't marshal it, use the original
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return c.wrapped.RoundTrip(req)
	}

	// Create new request body with fixed data
	req.Body = io.NopCloser(bytes.NewReader(fixedBodyBytes))
	req.ContentLength = int64(len(fixedBodyBytes))

	return c.wrapped.RoundTrip(req)
}

// Generate implements model.ChatModel
func (c *CustomChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return c.wrapped.Generate(ctx, in, opts...)
}

// Stream implements model.ChatModel
func (c *CustomChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return c.wrapped.Stream(ctx, in, opts...)
}

// WithTools implements model.ToolCallingChatModel
func (c *CustomChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	wrappedWithTools, err := c.wrapped.WithTools(tools)
	if err != nil {
		return nil, err
	}

	// Type assert back to *einoopenai.ChatModel
	wrappedChatModel, ok := wrappedWithTools.(*einoopenai.ChatModel)
	if !ok {
		return nil, fmt.Errorf("unexpected type returned from WithTools")
	}

	return &CustomChatModel{wrapped: wrappedChatModel}, nil
}

// BindTools implements model.ToolCallingChatModel
func (c *CustomChatModel) BindTools(tools []*schema.ToolInfo) error {
	return c.wrapped.BindTools(tools)
}

// BindForcedTools implements model.ToolCallingChatModel
func (c *CustomChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return c.wrapped.BindForcedTools(tools)
}

// GetType implements model.ChatModel
func (c *CustomChatModel) GetType() string {
	return "CustomOpenAI"
}

// IsCallbacksEnabled implements model.ChatModel
func (c *CustomChatModel) IsCallbacksEnabled() bool {
	return c.wrapped.IsCallbacksEnabled()
}
