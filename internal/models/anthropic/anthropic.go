package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	einoclaude "github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// CustomChatModel wraps the eino-ext Claude model with custom tool schema handling.
// It provides a compatibility layer that fixes malformed JSON in tool calls and
// ensures proper schema validation for Anthropic's API requirements.
// This wrapper is necessary to handle edge cases where the underlying library
// may generate invalid JSON for empty tool inputs or missing properties.
type CustomChatModel struct {
	// wrapped is the underlying eino-ext Claude model instance
	wrapped *einoclaude.ChatModel
}

// CustomRoundTripper intercepts HTTP requests to fix Anthropic function schemas.
// It acts as a middleware that modifies requests before they reach the Anthropic API,
// ensuring that tool schemas and function calls are properly formatted.
// This is particularly important for handling edge cases like empty tool inputs
// or missing schema properties that would otherwise cause API errors.
type CustomRoundTripper struct {
	// wrapped is the underlying HTTP transport to use for actual requests
	wrapped http.RoundTripper
}

// NewCustomChatModel creates a new custom Anthropic chat model.
// It wraps the standard eino-ext Claude model with additional request
// preprocessing to ensure compatibility with Anthropic's API requirements.
//
// Parameters:
//   - ctx: Context for the operation
//   - config: Configuration for the Claude model including API key, model name, and parameters
//
// Returns:
//   - *CustomChatModel: A wrapped Claude model with enhanced compatibility
//   - error: Returns an error if model creation fails
//
// The custom model automatically:
//   - Fixes malformed JSON in tool calls
//   - Ensures tool schemas have required properties
//   - Handles empty or missing input fields in function calls
func NewCustomChatModel(ctx context.Context, config *einoclaude.Config) (*CustomChatModel, error) {
	// Create a custom HTTP client that intercepts requests
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{}
	}

	// Wrap the transport with our custom round tripper
	if config.HTTPClient.Transport == nil {
		config.HTTPClient.Transport = http.DefaultTransport
	}
	config.HTTPClient.Transport = &CustomRoundTripper{
		wrapped: config.HTTPClient.Transport,
	}

	// Create the wrapped model
	wrapped, err := einoclaude.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}

	return &CustomChatModel{
		wrapped: wrapped,
	}, nil
}

// RoundTrip implements http.RoundTripper to intercept and fix requests.
// It preprocesses outgoing requests to the Anthropic API to ensure
// they meet the API's requirements for tool schemas and function calls.
//
// Parameters:
//   - req: The HTTP request to be sent to the Anthropic API
//
// Returns:
//   - *http.Response: The response from the Anthropic API
//   - error: Any error that occurred during the request
//
// The method performs the following fixes:
//   - Ensures tool input_schema properties are not null
//   - Fixes malformed JSON patterns in tool_use content
//   - Validates and corrects empty or invalid function call inputs
func (rt *CustomRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only process Anthropic API requests
	if !strings.Contains(req.URL.Host, "anthropic.com") {
		return rt.wrapped.RoundTrip(req)
	}

	// Read the request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	// Apply string-based fixes BEFORE JSON parsing for malformed patterns
	bodyStr := string(body)

	// Replace common malformed patterns - be more specific about context
	replacements := []struct {
		old string
		new string
	}{
		// Handle input field in tool_use objects
		{`"input":,"name"`, `"input":{},"name"`},
		{`"input":,"type"`, `"input":{},"type"`},
		{`"input":}`, `"input":{}}`},
		// Handle arguments field in function calls
		{`"arguments":,"name"`, `"arguments":"{}","name"`},
		{`"arguments":,"type"`, `"arguments":"{}","type"`},
		{`"arguments":}`, `"arguments":"{}"`},
		// Fallback patterns (less specific)
		{`"input":,`, `"input":{}`},
		{`"arguments":,`, `"arguments":"{}"`},
	}

	for _, r := range replacements {
		if strings.Contains(bodyStr, r.old) {
			bodyStr = strings.ReplaceAll(bodyStr, r.old, r.new)
		}
	}

	// Parse the JSON request (after string fixes)
	var requestData map[string]interface{}
	if err := json.Unmarshal([]byte(bodyStr), &requestData); err != nil {
		// Return the original request to avoid panic
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		return rt.wrapped.RoundTrip(req)
	}

	// Fix tool schemas if present
	if tools, ok := requestData["tools"].([]interface{}); ok {
		for _, tool := range tools {
			if toolMap, ok := tool.(map[string]interface{}); ok {
				if inputSchema, ok := toolMap["input_schema"].(map[string]interface{}); ok {
					// Ensure properties exists and is not null
					if properties, exists := inputSchema["properties"]; !exists || properties == nil {
						inputSchema["properties"] = map[string]interface{}{}
					} else if propertiesMap, ok := properties.(map[string]interface{}); ok {
						// Ensure each property has a type
						for _, propValue := range propertiesMap {
							if propMap, ok := propValue.(map[string]interface{}); ok {
								if _, hasType := propMap["type"]; !hasType {
									propMap["type"] = "string"
								}
							}
						}
					}
				}
			}
		}
	}

	// Fix tool_use content in messages if present
	if messages, ok := requestData["messages"].([]interface{}); ok {
		for _, message := range messages {
			if msgMap, ok := message.(map[string]interface{}); ok {
				if content, ok := msgMap["content"].([]interface{}); ok {
					for _, contentItem := range content {
						if contentMap, ok := contentItem.(map[string]interface{}); ok {
							if contentType, ok := contentMap["type"].(string); ok && contentType == "tool_use" {
								// Ensure tool_use input is valid JSON
								if input, exists := contentMap["input"]; exists {
									// If input is nil or empty, set it to an empty object
									if input == nil {
										contentMap["input"] = map[string]interface{}{}
									} else if inputBytes, ok := input.(json.RawMessage); ok {
										if len(inputBytes) == 0 {
											contentMap["input"] = map[string]interface{}{}
										} else {
											// Validate that it's valid JSON
											var temp interface{}
											if err := json.Unmarshal(inputBytes, &temp); err != nil {
												contentMap["input"] = map[string]interface{}{}
											}
										}
									} else if inputStr, ok := input.(string); ok {
										// Handle string inputs that might be empty or invalid JSON
										if inputStr == "" || inputStr == "{}" {
											contentMap["input"] = map[string]interface{}{}
										} else {
											// Try to parse as JSON
											var temp interface{}
											if err := json.Unmarshal([]byte(inputStr), &temp); err != nil {
												contentMap["input"] = map[string]interface{}{}
											}
										}
									}
								} else {
									// If input field doesn't exist, add it as empty object
									contentMap["input"] = map[string]interface{}{}
								}
							}
						}
					}
				}
			}
		}
	}

	// Marshal the fixed request back to JSON
	fixedBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	// Use the fixed body from JSON marshaling
	finalBodyStr := string(fixedBody)

	// Validate the final JSON
	var finalCheck interface{}
	if err := json.Unmarshal([]byte(finalBodyStr), &finalCheck); err != nil {
		return nil, err
	}

	// Create new request with fixed body
	req.Body = io.NopCloser(strings.NewReader(finalBodyStr))
	req.ContentLength = int64(len(finalBodyStr))
	// Make the actual request
	return rt.wrapped.RoundTrip(req)
}

// Generate implements the model.BaseChatModel interface.
// It generates a single response from the model based on the input messages.
//
// Parameters:
//   - ctx: Context for the operation, supporting cancellation and deadlines
//   - input: The conversation history as a slice of messages
//   - opts: Optional configuration options for the generation
//
// Returns:
//   - *schema.Message: The generated response message
//   - error: Any error that occurred during generation
func (m *CustomChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.wrapped.Generate(ctx, input, opts...)
}

// Stream implements the model.BaseChatModel interface.
// It generates a streaming response from the model, allowing incremental
// processing of the model's output as it's generated.
//
// Parameters:
//   - ctx: Context for the operation, supporting cancellation and deadlines
//   - input: The conversation history as a slice of messages
//   - opts: Optional configuration options for the generation
//
// Returns:
//   - *schema.StreamReader[*schema.Message]: A reader for the streaming response
//   - error: Any error that occurred during stream setup
func (m *CustomChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return m.wrapped.Stream(ctx, input, opts...)
}

// WithTools implements the model.ToolCallingChatModel interface.
// It creates a new model instance with the specified tools available for function calling.
// The original model instance remains unchanged.
//
// Parameters:
//   - tools: A slice of tool definitions that the model can use
//
// Returns:
//   - model.ToolCallingChatModel: A new model instance with tools enabled
//   - error: Returns an error if tool binding fails
func (m *CustomChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	wrappedWithTools, err := m.wrapped.WithTools(tools)
	if err != nil {
		return nil, err
	}

	return &CustomChatModel{
		wrapped: wrappedWithTools.(*einoclaude.ChatModel),
	}, nil
}
