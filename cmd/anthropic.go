package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mark3labs/mcphost/pkg/llm"
)

type AnthropicClient struct {
	apiKey string
	client *http.Client
}

func NewAnthropicClient(apiKey string) *AnthropicClient {
	return &AnthropicClient{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

type Message struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// For tool use blocks
	ID        string          `json:"id,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	// For tool result blocks
	Content interface{} `json:"content,omitempty"` // Can be string or []ContentBlock
}

type MessageParam struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type CreateMessageRequest struct {
	Model     string         `json:"model"`
	Messages  []MessageParam `json:"messages"`
	MaxTokens int            `json:"max_tokens"`
	Tools     []llm.Tool     `json:"tools,omitempty"`
}

func (c *AnthropicClient) CreateMessage(ctx context.Context, req CreateMessageRequest) (*Message, error) {
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

		// Special handling for overloaded error
		if errResp.Error.Type == "overloaded_error" {
			return nil, fmt.Errorf("overloaded_error: %s", errResp.Error.Message)
		}

		return nil, fmt.Errorf("%s: %s", errResp.Error.Type, errResp.Error.Message)
	}

	var message Message
	if err := json.NewDecoder(resp.Body).Decode(&message); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &message, nil
}
