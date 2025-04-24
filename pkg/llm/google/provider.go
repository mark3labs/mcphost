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

	toolCallID int
}

func NewProvider(ctx context.Context, apiKey, model, systemPrompt string) (*Provider, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	m := client.GenerativeModel(model)
	// If systemPrompt is provided, set the system prompt for the model.
	if systemPrompt != "" {
		m.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	}
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
				Role: mappingRole(msg.GetRole()),
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
					if block.Type == "tool_result" {
						hist = append(hist, &genai.Content{
							Role:  mappingRole(msg.GetRole()),
							Parts: []genai.Part{genai.Text(block.Text)},
						})
					}
				}
			}
		}

		if text := strings.TrimSpace(msg.GetContent()); text != "" {
			hist = append(hist, &genai.Content{
				Role:  mappingRole(msg.GetRole()),
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
					Parameters:  translateToGoogleSchema(tool.InputSchema),
				},
			},
		})
	}

	p.chat.History = hist
	// The provided messages slice (and thus history) already includes the new prompt,
	// so we just call SendMessage with an empty string that will be trimmed by the server.
	resp, err := p.chat.SendMessage(ctx, genai.Text(""))
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no response from model")
	}

	// The library enforces a generation config with 1 candidate.
	m := &Message{
		Candidate:  resp.Candidates[0],
		toolCallID: p.toolCallID,
	}

	p.toolCallID += len(m.Candidate.FunctionCalls())
	return m, nil
}

func (p *Provider) CreateToolResponse(toolCallID string, content any) (llm.Message, error) {
	// UNUSED: Nothing in root.go calls this.
	return nil, nil
}

func (p *Provider) SupportsTools() bool {
	// UNUSED: Nothing in root.go calls this.
	return true
}

func (p *Provider) Name() string {
	return "Google"
}

func translateToGoogleSchema(schema llm.Schema) *genai.Schema {
	s := &genai.Schema{
		Type:       toType(schema.Type),
		Required:   schema.Required,
		Properties: make(map[string]*genai.Schema),
	}

	for name, prop := range schema.Properties {
		m, ok := prop.(map[string]any)
		if !ok || len(m) == 0 {
			continue
		}
		s.Properties[name] = propertyToGoogleSchema(m)
	}

	if len(s.Properties) == 0 {
		// Functions that don't take any arguments have an object-type schema with 0 properties.
		// Google/Gemini does not like that: Error 400: * GenerateContentRequest properties: should be non-empty for OBJECT type.
		// To work around this issue, we'll just inject some unused, nullable property with a primitive type.
		s.Nullable = true
		s.Properties["unused"] = &genai.Schema{
			Type:     genai.TypeInteger,
			Nullable: true,
		}
	}
	return s
}

func propertyToGoogleSchema(properties map[string]any) *genai.Schema {
	typ, ok := properties["type"].(string)
	if !ok {
		return nil
	}
	s := &genai.Schema{Type: toType(typ)}
	if desc, ok := properties["description"].(string); ok {
		s.Description = desc
	}

	// Objects and arrays need to have their properties recursively mapped.
	if s.Type == genai.TypeObject {
		objectProperties := properties["properties"].(map[string]any)
		s.Properties = make(map[string]*genai.Schema)
		for name, prop := range objectProperties {
			s.Properties[name] = propertyToGoogleSchema(prop.(map[string]any))
		}
	} else if s.Type == genai.TypeArray {
		itemProperties := properties["items"].(map[string]any)
		s.Items = propertyToGoogleSchema(itemProperties)
	}

	return s
}

func toType(typ string) genai.Type {
	switch typ {
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	case "object":
		return genai.TypeObject
	case "array":
		return genai.TypeArray
	default:
		return genai.TypeUnspecified
	}
}

const (
	roleUser  = "user"
	roleModel = "model"
)

var roleMap = map[string]string{
	roleUser:  roleUser,
	roleModel: roleModel,
}

func mappingRole(role string) string {
	v, ok := roleMap[role]
	if !ok {
		return roleUser
	}
	return v
}
