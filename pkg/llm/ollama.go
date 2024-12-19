package llm

import (
    "context"
    "encoding/json"
    "fmt"
    api "github.com/ollama/ollama/api"
    "strings"
)

// OllamaProvider implements the Provider interface for Ollama
type OllamaProvider struct {
	client *api.Client
	model  string
}

// OllamaMessage adapts Ollama's message format to our Message interface
type OllamaMessage struct {
    Message api.Message
    ToolCallID string // Store tool call ID separately since Ollama API doesn't have this field
}

func (m *OllamaMessage) GetRole() string {
    return m.Message.Role
}

func (m *OllamaMessage) GetContent() string {
    return strings.TrimSpace(m.Message.Content)
}

func (m *OllamaMessage) GetToolCalls() []ToolCall {
    var calls []ToolCall
    for _, call := range m.Message.ToolCalls {
        calls = append(calls, &OllamaToolCall{call})
    }
    return calls
}

func (m *OllamaMessage) GetUsage() (int, int) {
    return 0, 0 // Ollama doesn't provide token usage info
}

func (m *OllamaMessage) GetToolCallID() string {
    return m.ToolCallID
}

// OllamaToolCall adapts Ollama's tool call format
type OllamaToolCall struct {
    call api.ToolCall
}

func (t *OllamaToolCall) GetName() string {
    return t.call.Function.Name
}

func (t *OllamaToolCall) GetArguments() map[string]interface{} {
    return t.call.Function.Arguments
}

func (t *OllamaToolCall) GetID() string {
    return t.call.Function.Name // Use function name as ID since Ollama doesn't have tool call IDs
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(model string) (*OllamaProvider, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, err
	}
	return &OllamaProvider{
		client: client,
		model:  model,
	}, nil
}

func (p *OllamaProvider) CreateMessage(ctx context.Context, prompt string, messages []Message, tools []Tool) (Message, error) {
    // Convert generic messages to Ollama format
    ollamaMessages := make([]api.Message, 0, len(messages)+1)
    
    // Add existing messages, filtering out tool-related messages
    for _, msg := range messages {
        // Skip empty messages and tool responses
        if msg.GetContent() == "" || msg.GetRole() == "tool" {
            continue
        }
        
        ollamaMsg := api.Message{
            Role:    msg.GetRole(),
            Content: msg.GetContent(),
        }
        
        // Only add tool calls for assistant messages
        if msg.GetRole() == "assistant" {
            for _, call := range msg.GetToolCalls() {
                if call.GetName() != "" { // Skip empty tool calls
                    ollamaMsg.ToolCalls = append(ollamaMsg.ToolCalls, api.ToolCall{
                        Function: api.ToolCallFunction{
                            Name:      call.GetName(),
                            Arguments: call.GetArguments(),
                        },
                    })
                }
            }
        }
        
        ollamaMessages = append(ollamaMessages, ollamaMsg)
    }

    // Add the new prompt if not empty
    if prompt != "" {
        ollamaMessages = append(ollamaMessages, api.Message{
            Role:    "user",
            Content: prompt,
        })
    }

    // Ensure we have a system message at the start
    if len(ollamaMessages) == 0 || ollamaMessages[0].Role != "system" {
        systemMsg := api.Message{
            Role: "system",
            Content: `You are a helpful AI assistant. Respond directly and naturally to questions and requests.
Keep track of the conversation context and previous information shared by the user.
Only use tools when specifically needed to accomplish a task.`,
        }
        ollamaMessages = append([]api.Message{systemMsg}, ollamaMessages...)
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
                    Type       string                                                                           `json:"type"`
                    Required   []string                                                                         `json:"required"`
                    Properties map[string]struct{Type string `json:"type"`; Description string `json:"description"`; Enum []string `json:"enum,omitempty"`} `json:"properties"`
                }{
                    Type:       tool.InputSchema.Type,
                    Required:   tool.InputSchema.Required,
                    Properties: convertProperties(tool.InputSchema.Properties),
                },
            },
        }
    }

    // Only make the API call if we have messages
    if len(ollamaMessages) == 0 {
        return &OllamaMessage{
            Message: api.Message{
                Role:    "assistant",
                Content: "I don't have any context to respond to. Could you please provide a message or question?",
            },
        }, nil
    }

    var response api.Message
    err := p.client.Chat(ctx, &api.ChatRequest{
        Model:    p.model,
        Messages: ollamaMessages,
        Tools:    ollamaTools,
        Stream:   F(false), // Disable streaming
    }, func(r api.ChatResponse) error {
        if r.Done {
            response = r.Message
        }
        return nil
    })

    if err != nil {
        return nil, err
    }

    // Clean up any empty tool calls in the response
    var cleanToolCalls []api.ToolCall
    for _, call := range response.ToolCalls {
        if call.Function.Name != "" {
            cleanToolCalls = append(cleanToolCalls, call)
        }
    }
    response.ToolCalls = cleanToolCalls

    return &OllamaMessage{Message: response}, nil
}

func (p *OllamaProvider) SupportsTools() bool {
	// Check if model supports function calling
	resp, err := p.client.Show(context.Background(), &api.ShowRequest{
		Model: p.model,
	})
	if err != nil {
		return false
	}
	return strings.Contains(resp.Modelfile, "<tools>")
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

func (p *OllamaProvider) CreateToolResponse(toolCallID string, content interface{}) (Message, error) {
    // Convert content to string if needed
    var contentStr string
    switch v := content.(type) {
    case string:
        contentStr = v
    default:
        // Marshal other types to JSON
        bytes, err := json.Marshal(v)
        if err != nil {
            return nil, fmt.Errorf("error marshaling tool response: %w", err)
        }
        contentStr = string(bytes)
    }

    // Create a tool response message
    return &OllamaMessage{
        Message: api.Message{
            Role:    "tool",
            Content: contentStr,
        },
        ToolCallID: toolCallID,
    }, nil
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

// Helper function to get pointer to value
func F[T any](v T) *T {
	return &v
}
