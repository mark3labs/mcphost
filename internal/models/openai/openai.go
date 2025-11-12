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

// CustomChatModel wraps the eino-ext OpenAI model with custom tool schema handling.
// It provides a compatibility layer that ensures proper JSON schema formatting
// for OpenAI's function calling feature. This wrapper addresses cases where
// tool schemas might have missing or empty properties that would cause API errors.
type CustomChatModel struct {
	// wrapped is the underlying eino-ext OpenAI model instance
	wrapped *einoopenai.ChatModel
}

// CustomRoundTripper intercepts HTTP requests to fix OpenAI function schemas.
// It acts as middleware that modifies outgoing requests to ensure that
// function/tool schemas are properly formatted according to OpenAI's requirements.
// This is particularly important for handling edge cases where tool schemas
// might have missing or empty properties fields.
type CustomRoundTripper struct {
	// wrapped is the underlying HTTP transport to use for actual requests
	wrapped http.RoundTripper
}

// NewCustomChatModel creates a new custom OpenAI chat model.
// It wraps the standard eino-ext OpenAI model with additional request
// preprocessing to ensure compatibility with OpenAI's API requirements,
// particularly for function calling and tool schemas.
//
// Parameters:
//   - ctx: Context for the operation
//   - config: Configuration for the OpenAI model including API key, model name, and parameters
//
// Returns:
//   - *CustomChatModel: A wrapped OpenAI model with enhanced compatibility
//   - error: Returns an error if model creation fails
//
// The custom model automatically:
//   - Ensures function parameter schemas have properties fields
//   - Fixes missing or empty properties in tool schemas
//   - Maintains compatibility with OpenAI's function calling requirements
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

// RoundTrip implements http.RoundTripper to intercept and fix OpenAI requests.
// It preprocesses outgoing requests to the OpenAI API to ensure tool/function
// schemas meet the API's requirements.
//
// Parameters:
//   - req: The HTTP request to be sent to the OpenAI API
//
// Returns:
//   - *http.Response: The response from the OpenAI API
//   - error: Any error that occurred during the request
//
// The method performs the following fixes:
//   - Ensures function parameter schemas of type "object" have a properties field
//   - Adds empty properties object if missing to prevent API validation errors
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

// Generate implements model.ChatModel interface.
// It generates a single response from the OpenAI model based on the input messages.
//
// Parameters:
//   - ctx: Context for the operation, supporting cancellation and deadlines
//   - in: The conversation history as a slice of messages
//   - opts: Optional configuration options for the generation
//
// Returns:
//   - *schema.Message: The generated response message
//   - error: Any error that occurred during generation
func (c *CustomChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return c.wrapped.Generate(ctx, in, opts...)
}

// Stream implements model.ChatModel interface.
// It generates a streaming response from the OpenAI model, allowing
// incremental processing of the model's output as it's generated.
//
// Parameters:
//   - ctx: Context for the operation, supporting cancellation and deadlines
//   - in: The conversation history as a slice of messages
//   - opts: Optional configuration options for the generation
//
// Returns:
//   - *schema.StreamReader[*schema.Message]: A reader for the streaming response
//   - error: Any error that occurred during stream setup
func (c *CustomChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return c.wrapped.Stream(ctx, in, opts...)
}

// WithTools implements model.ToolCallingChatModel interface.
// It creates a new model instance with the specified tools available for function calling.
// The original model instance remains unchanged.
//
// Parameters:
//   - tools: A slice of tool definitions that the model can use
//
// Returns:
//   - model.ToolCallingChatModel: A new model instance with tools enabled
//   - error: Returns an error if tool binding fails
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

// BindTools implements model.ToolCallingChatModel interface.
// It binds tools to the current model instance, modifying it in place
// rather than creating a new instance.
//
// Parameters:
//   - tools: A slice of tool definitions to bind to the model
//
// Returns:
//   - error: Returns an error if tool binding fails
func (c *CustomChatModel) BindTools(tools []*schema.ToolInfo) error {
	return c.wrapped.BindTools(tools)
}

// BindForcedTools implements model.ToolCallingChatModel interface.
// It binds tools to the current model instance in forced mode,
// ensuring the model will always use one of the provided tools.
//
// Parameters:
//   - tools: A slice of tool definitions to bind to the model
//
// Returns:
//   - error: Returns an error if tool binding fails
func (c *CustomChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return c.wrapped.BindForcedTools(tools)
}

// GetType implements model.ChatModel interface.
// It returns the type identifier for this model implementation.
//
// Returns:
//   - string: Returns "CustomOpenAI" as the model type identifier
func (c *CustomChatModel) GetType() string {
	return "CustomOpenAI"
}

// IsCallbacksEnabled implements model.ChatModel interface.
// It indicates whether this model supports callbacks for monitoring
// and tracking purposes.
//
// Returns:
//   - bool: Returns the callback enabled status from the wrapped model
func (c *CustomChatModel) IsCallbacksEnabled() bool {
	return c.wrapped.IsCallbacksEnabled()
}
