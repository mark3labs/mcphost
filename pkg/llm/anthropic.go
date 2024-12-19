package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"
)

// AnthropicProvider implements the Provider interface for Anthropic's Claude
type AnthropicProvider struct {
    client *AnthropicClient
    model  string
}

// AnthropicClient handles API communication with Anthropic
type AnthropicClient struct {
    apiKey string
    client *http.Client
}

// AnthropicMessage adapts Anthropic's message format to our Message interface
type AnthropicMessage struct {
    Msg AnthropicAPIMessage
}

// AnthropicToolCall implements the ToolCall interface for Anthropic tool calls
type AnthropicToolCall struct {
    block AnthropicContent
}

func NewAnthropicToolCall(block AnthropicContent) *AnthropicToolCall {
    if block.ID == "" {
        block.ID = fmt.Sprintf("tc_%s_%d", block.Name, time.Now().UnixNano())
    }
    return &AnthropicToolCall{block: block}
}

func (t *AnthropicToolCall) GetName() string {
    return t.block.Name
}

func (t *AnthropicToolCall) GetArguments() map[string]interface{} {
    var args map[string]interface{}
    if err := json.Unmarshal(t.block.Input, &args); err != nil {
        return nil
    }
    return args
}

func (t *AnthropicToolCall) GetID() string {
    return t.block.ID
}

// Internal Anthropic API types
type AnthropicAPIMessage struct {
    ID           string              `json:"id"`
    Type         string              `json:"type"`
    Role         string              `json:"role"`
    Content      []AnthropicContent  `json:"content"`
    Model        string              `json:"model"`
    StopReason   *string             `json:"stop_reason"`
    StopSequence *string             `json:"stop_sequence"`
    Usage        AnthropicUsage      `json:"usage"`
}

type AnthropicUsage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}

type AnthropicContent struct {
    Type      string          `json:"type"`
    Text      string          `json:"text,omitempty"`
    ID        string          `json:"id,omitempty"`
    ToolUseID string          `json:"tool_use_id,omitempty"`
    Name      string          `json:"name,omitempty"`
    Input     json.RawMessage `json:"input,omitempty"`
    Content   interface{}     `json:"content,omitempty"`  // Can be string for tool results
}

type AnthropicMessageParam struct {
    Role    string             `json:"role"`
    Content []AnthropicContent `json:"content"`
}

type AnthropicCreateRequest struct {
    Model     string                 `json:"model"`
    Messages  []AnthropicMessageParam `json:"messages"`
    MaxTokens int                    `json:"max_tokens"`
    Tools     []AnthropicTool        `json:"tools,omitempty"`
}

type AnthropicTool struct {
    Name        string               `json:"name"`
    Description string               `json:"description,omitempty"`
    InputSchema AnthropicInputSchema `json:"input_schema"`
}

type AnthropicInputSchema struct {
    Type       string                 `json:"type"`
    Properties map[string]interface{} `json:"properties,omitempty"`
    Required   []string              `json:"required,omitempty"`
}

// Interface implementation methods
func (m *AnthropicMessage) GetRole() string {
    return m.Msg.Role
}

func (m *AnthropicMessage) GetContent() string {
    var content string
    for _, block := range m.Msg.Content {
        if block.Type == "text" {
            content += block.Text + "\n"
        }
    }
    return strings.TrimSpace(content)
}

func (m *AnthropicMessage) GetToolCalls() []ToolCall {
    var calls []ToolCall
    for _, block := range m.Msg.Content {
        if block.Type == "tool_use" {
            calls = append(calls, NewAnthropicToolCall(block))
        }
    }
    return calls
}

func (m *AnthropicMessage) GetUsage() (int, int) {
    return m.Msg.Usage.InputTokens, m.Msg.Usage.OutputTokens
}

func (m *AnthropicMessage) IsToolResponse() bool {
    for _, block := range m.Msg.Content {
        if block.Type == "tool_result" {
            return true
        }
    }
    return false
}

func (m *AnthropicMessage) GetToolResponseID() string {
    for _, block := range m.Msg.Content {
        if block.Type == "tool_result" {
            return block.ToolUseID
        }
    }
    return ""
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey string) *AnthropicProvider {
    return &AnthropicProvider{
        client: &AnthropicClient{
            apiKey: apiKey,
            client: &http.Client{},
        },
        model: "claude-3-5-sonnet-20240620",
    }
}

func (p *AnthropicProvider) CreateMessage(ctx context.Context, prompt string, messages []Message, tools []Tool) (Message, error) {
    // Convert generic messages to Anthropic format
    anthropicMessages := make([]AnthropicMessageParam, 0, len(messages))
    
    for _, msg := range messages {
        content := []AnthropicContent{{
            Type: "text",
            Text: strings.TrimSpace(msg.GetContent()),
        }}
        
        // Add tool calls if present
        for _, call := range msg.GetToolCalls() {
            input, _ := json.Marshal(call.GetArguments())
            content = append(content, AnthropicContent{
                Type:  "tool_use",
                ID:    call.GetID(),
                Name:  call.GetName(),
                Input: input,
            })
        }

        // Handle tool responses
        if msg.IsToolResponse() {
            content = []AnthropicContent{{
                Type:      "tool_result",
                ToolUseID: msg.GetToolResponseID(),
                Content: []AnthropicContent{{
                    Type: "text",
                    Text: msg.GetContent(),
                }},
            }}
        }

        if len(content) > 0 {
            anthropicMessages = append(anthropicMessages, AnthropicMessageParam{
                Role:    msg.GetRole(),
                Content: content,
            })
        }
    }

    // Add the new prompt if provided
    if prompt != "" {
        anthropicMessages = append(anthropicMessages, AnthropicMessageParam{
            Role: "user",
            Content: []AnthropicContent{{
                Type: "text",
                Text: prompt,
            }},
        })
    }

    // Convert tools to Anthropic format
    anthropicTools := make([]AnthropicTool, len(tools))
    for i, tool := range tools {
        anthropicTools[i] = AnthropicTool{
            Name:        tool.Name,
            Description: tool.Description,
            InputSchema: AnthropicInputSchema{
                Type:       tool.InputSchema.Type,
                Properties: tool.InputSchema.Properties,
                Required:   tool.InputSchema.Required,
            },
        }
    }

    // Make the API call
    resp, err := p.client.createMessage(ctx, AnthropicCreateRequest{
        Model:     p.model,
        Messages:  anthropicMessages,
        MaxTokens: 4096,
        Tools:     anthropicTools,
    })
    if err != nil {
        return nil, err
    }

    return &AnthropicMessage{Msg: *resp}, nil
}

func (p *AnthropicProvider) SupportsTools() bool {
    return true
}

func (p *AnthropicProvider) Name() string {
    return "anthropic"
}

func (p *AnthropicProvider) CreateToolResponse(toolCallID string, content interface{}) (Message, error) {
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

    return &AnthropicMessage{
        Msg: AnthropicAPIMessage{
            Role: "assistant",
            Content: []AnthropicContent{{
                Type:      "tool_result",
                ToolUseID: toolCallID,
                Content: []AnthropicContent{{
                    Type: "text",
                    Text: contentStr,
                }},
            }},
        },
    }, nil
}

// Internal API methods
func (c *AnthropicClient) createMessage(ctx context.Context, req AnthropicCreateRequest) (*AnthropicAPIMessage, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("error marshaling request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("error creating request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("X-Api-Key", c.apiKey)
    httpReq.Header.Set("anthropic-version", "2023-06-01")

    resp, err := c.client.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("error making request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        var errResp struct {
            Error struct {
                Type    string `json:"type"`
                Message string `json:"message"`
            } `json:"error"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
            return nil, fmt.Errorf("error response with status %d", resp.StatusCode)
        }

        if errResp.Error.Type == "overloaded_error" {
            return nil, fmt.Errorf("overloaded_error: %s", errResp.Error.Message)
        }

        return nil, fmt.Errorf("%s: %s", errResp.Error.Type, errResp.Error.Message)
    }

    var message AnthropicAPIMessage
    if err := json.NewDecoder(resp.Body).Decode(&message); err != nil {
        return nil, fmt.Errorf("error decoding response: %w", err)
    }

    return &message, nil
}
