package models

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiToolCallingModel implements the ToolCallingChatModel interface for Gemini
type GeminiToolCallingModel struct {
	client    *genai.Client
	modelName string
	tools     []*schema.ToolInfo
}

// NewGeminiToolCallingModel creates a new custom Gemini tool calling model
func NewGeminiToolCallingModel(ctx context.Context, apiKey, modelName string) (*GeminiToolCallingModel, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key required for Gemini model")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiToolCallingModel{
		client:    client,
		modelName: modelName,
		tools:     nil,
	}, nil
}

// Generate implements the BaseChatModel interface
func (g *GeminiToolCallingModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	parts, err := g.convertMessagesToParts(input)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	model := g.client.GenerativeModel(g.modelName)
	
	// Add tools if available
	if len(g.tools) > 0 {
		geminiTools, err := g.convertToolsToGemini(g.tools)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tools: %w", err)
		}
		model.Tools = geminiTools
		model.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingUnspecified,
			},
		}
	}

	resp, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	return g.convertResponseToMessage(resp)
}

// Stream implements the BaseChatModel interface
func (g *GeminiToolCallingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	parts, err := g.convertMessagesToParts(input)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	model := g.client.GenerativeModel(g.modelName)
	
	// Add tools if available
	if len(g.tools) > 0 {
		geminiTools, err := g.convertToolsToGemini(g.tools)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tools: %w", err)
		}
		model.Tools = geminiTools
		model.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingUnspecified,
			},
		}
	}

	iter := model.GenerateContentStream(ctx, parts...)
	return g.createStreamReader(iter), nil
}

// WithTools implements the ToolCallingChatModel interface
func (g *GeminiToolCallingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	// Create a new instance with the tools bound
	newModel := &GeminiToolCallingModel{
		client:    g.client,
		modelName: g.modelName,
		tools:     make([]*schema.ToolInfo, len(tools)),
	}
	copy(newModel.tools, tools)
	return newModel, nil
}

// convertMessagesToParts converts eino messages to Gemini parts
func (g *GeminiToolCallingModel) convertMessagesToParts(messages []*schema.Message) ([]genai.Part, error) {
	var parts []genai.Part

	for _, msg := range messages {
		// Handle text content
		if msg.Content != "" {
			parts = append(parts, genai.Text(msg.Content))
		}

		// Handle tool calls
		if len(msg.ToolCalls) > 0 {
			for _, toolCall := range msg.ToolCalls {
				// Convert tool call arguments to map
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					return nil, fmt.Errorf("failed to unmarshal tool call args: %w", err)
				}

				parts = append(parts, genai.FunctionCall{
					Name: toolCall.Function.Name,
					Args: args,
				})
			}
		}

		// Handle tool responses (messages with ToolCallID)
		if msg.ToolCallID != "" && msg.Content != "" {
			parts = append(parts, genai.FunctionResponse{
				Name: msg.ToolName,
				Response: map[string]interface{}{
					"result": msg.Content,
				},
			})
		}
	}

	return parts, nil
}

// convertToolsToGemini converts eino tools to Gemini tools
func (g *GeminiToolCallingModel) convertToolsToGemini(tools []*schema.ToolInfo) ([]*genai.Tool, error) {
	var geminiTools []*genai.Tool

	functionDeclarations := []*genai.FunctionDeclaration{}

	for _, tool := range tools {
		// Convert the tool schema to Gemini schema
		var geminiSchema *genai.Schema
		
		if tool.ParamsOneOf != nil {
			openAPISchema, err := tool.ParamsOneOf.ToOpenAPIV3()
			if err != nil {
				return nil, fmt.Errorf("failed to convert tool params to OpenAPI for tool %s: %w", tool.Name, err)
			}
			
			// Convert OpenAPI schema to map for our converter
			schemaBytes, err := json.Marshal(openAPISchema)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal OpenAPI schema for tool %s: %w", tool.Name, err)
			}
			
			var schemaMap map[string]interface{}
			if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal OpenAPI schema for tool %s: %w", tool.Name, err)
			}
			
			geminiSchema, err = g.convertSchemaToGemini(schemaMap)
			if err != nil {
				return nil, fmt.Errorf("failed to convert schema for tool %s: %w", tool.Name, err)
			}
		}

		functionDecl := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Desc,
			Parameters:  geminiSchema,
		}

		functionDeclarations = append(functionDeclarations, functionDecl)
	}

	if len(functionDeclarations) > 0 {
		geminiTools = append(geminiTools, &genai.Tool{
			FunctionDeclarations: functionDeclarations,
		})
	}

	return geminiTools, nil
}

// convertSchemaToGemini converts a JSON schema to Gemini schema
func (g *GeminiToolCallingModel) convertSchemaToGemini(schemaMap map[string]interface{}) (*genai.Schema, error) {
	schema := &genai.Schema{}

	if typeVal, ok := schemaMap["type"].(string); ok {
		switch typeVal {
		case "object":
			schema.Type = genai.TypeObject
		case "string":
			schema.Type = genai.TypeString
		case "number":
			schema.Type = genai.TypeNumber
		case "integer":
			schema.Type = genai.TypeInteger
		case "boolean":
			schema.Type = genai.TypeBoolean
		case "array":
			schema.Type = genai.TypeArray
		default:
			schema.Type = genai.TypeString
		}
	}

	if desc, ok := schemaMap["description"].(string); ok {
		schema.Description = desc
	}

	if props, ok := schemaMap["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for propName, propSchema := range props {
			if propSchemaMap, ok := propSchema.(map[string]interface{}); ok {
				convertedProp, err := g.convertSchemaToGemini(propSchemaMap)
				if err != nil {
					return nil, fmt.Errorf("failed to convert property %s: %w", propName, err)
				}
				schema.Properties[propName] = convertedProp
			}
		}
	}

	if required, ok := schemaMap["required"].([]interface{}); ok {
		schema.Required = make([]string, len(required))
		for i, req := range required {
			if reqStr, ok := req.(string); ok {
				schema.Required[i] = reqStr
			}
		}
	}

	return schema, nil
}

// convertResponseToMessage converts Gemini response to eino message
func (g *GeminiToolCallingModel) convertResponseToMessage(resp *genai.GenerateContentResponse) (*schema.Message, error) {
	message := &schema.Message{
		Role: schema.Assistant,
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return nil, fmt.Errorf("no content in candidate")
	}

	var textParts []string
	var toolCalls []schema.ToolCall

	for _, part := range candidate.Content.Parts {
		switch p := part.(type) {
		case genai.Text:
			textParts = append(textParts, string(p))
		case genai.FunctionCall:
			argsBytes, err := json.Marshal(p.Args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal function call args: %w", err)
			}

			toolCall := schema.ToolCall{
				ID: fmt.Sprintf("call_%s", p.Name),
				Function: schema.FunctionCall{
					Name:      p.Name,
					Arguments: string(argsBytes),
				},
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	if len(textParts) > 0 {
		message.Content = textParts[0] // Take the first text part
	}

	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	return message, nil
}

// createStreamReader creates a stream reader for Gemini streaming responses
func (g *GeminiToolCallingModel) createStreamReader(iter *genai.GenerateContentResponseIterator) *schema.StreamReader[*schema.Message] {
	reader, writer := schema.Pipe[*schema.Message](1)

	go func() {
		defer writer.Close()

		for {
			resp, err := iter.Next()
			if err != nil {
				writer.Send(nil, err)
				return
			}

			if resp == nil {
				return
			}

			message, err := g.convertResponseToMessage(resp)
			if err != nil {
				writer.Send(nil, err)
				return
			}

			writer.Send(message, nil)
		}
	}()

	return reader
}