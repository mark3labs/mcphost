package google

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/mark3labs/mcphost/pkg/history"
	"github.com/mark3labs/mcphost/pkg/llm"
	"google.golang.org/api/option"
)

type Provider struct {
	client *genai.Client
	model  *genai.GenerativeModel
	chat   *genai.ChatSession
}

func NewProvider(ctx context.Context, apiKey string, model string) (*Provider, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	m := client.GenerativeModel(model)
	return &Provider{
		client: client,
		model:  m,
		chat:   m.StartChat(),
	}, nil
}

func (p *Provider) CreateMessage(ctx context.Context, prompt string, messages []llm.Message, tools []llm.Tool) (llm.Message, error) {
	var hist []*genai.Content
	for _, msg := range messages {
		for _, call := range msg.GetToolCalls() {
			hist = append(hist, &genai.Content{
				Role: msg.GetRole(),
				Parts: []genai.Part{
					genai.FunctionCall{
						Name: call.GetName(),
						Args: call.GetArguments(),
					},
				},
			})
		}

		if msg.IsToolResponse() {
			if historyMsg, ok := msg.(*history.HistoryMessage); ok {
				for _, block := range historyMsg.Content {
					hist = append(hist, &genai.Content{
						Role:  msg.GetRole(),
						Parts: []genai.Part{genai.Text(block.Text)},
					})
				}
			}
		}

		if text := strings.TrimSpace(msg.GetContent()); text != "" && !msg.IsToolResponse() && len(msg.GetToolCalls()) == 0 {
			hist = append(hist, &genai.Content{
				Role:  msg.GetRole(),
				Parts: []genai.Part{genai.Text(text)},
			})
		}
	}

	p.model.Tools = nil
	for _, tool := range tools {
		p.model.Tools = append(p.model.Tools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  convertSchema(tool.InputSchema),
				},
			},
		})
	}

	resp, err := p.chat.SendMessage(ctx, genai.Text(""))
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no response")
	}

	// We'll only work with the first candidate.
	// Depending on the generation config, there will only be 1 candidate anyway.
	return &Message{Candidate: resp.Candidates[0]}, nil
}

func convertSchema(schema llm.Schema) *genai.Schema {

	s := &genai.Schema{
		Type:       toType(schema.Type),
		Required:   schema.Required,
		Properties: make(map[string]*genai.Schema),
	}

	for name, prop := range schema.Properties {
		s.Properties[name] = propertyToSchema(prop.(map[string]any))
	}

	if len(s.Properties) == 0 {
		// No arguments, yet still described as an object.
		// Gemini doesn't like this:
		// Error: googleapi: Error 400: * GenerateContentRequest.tools[N].function_declarations[0].parameters.properties: should be non-empty for OBJECT type
		// Just pretend this has a fake argument.
		s.Nullable = true
		s.Properties["unused"] = &genai.Schema{
			Type:     genai.TypeInteger,
			Nullable: true,
		}
	}
	return s
}

func propertyToSchema(properties map[string]any) *genai.Schema {
	s := &genai.Schema{Type: toType(properties["type"].(string))}
	if desc, ok := properties["description"].(string); ok {
		s.Description = desc
	}
	if s.Type == genai.TypeObject {
		objectProperties := properties["properties"].(map[string]any)
		s.Properties = make(map[string]*genai.Schema)
		for name, prop := range objectProperties {
			s.Properties[name] = propertyToSchema(prop.(map[string]any))
		}
	} else if s.Type == genai.TypeArray {
		itemProperties := properties["items"].(map[string]any)
		s.Items = propertyToSchema(itemProperties)
	}
	return s
}

func toType(typ string) genai.Type {
	switch typ {
	case "string":
		return genai.TypeString
	case "boolean":
		return genai.TypeBoolean
	case "object":
		return genai.TypeObject
	case "array":
		return genai.TypeArray
	default:
		panic(fmt.Errorf("unknown type %v", typ))
	}
}

func (p *Provider) CreateToolResponse(toolCallID string, content any) (llm.Message, error) {
	// Unused??
	return nil, nil
}

func (p *Provider) SupportsTools() bool {
	// Unused??
	return true
}

func (p *Provider) Name() string {
	return "Google"
}
