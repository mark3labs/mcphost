package openai

type CreateRequest struct {
	Model       string         `json:"model"`
	Messages    []MessageParam `json:"messages"`
	Tools       []Tool         `json:"tools,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float32        `json:"temperature,omitempty"`
}

type MessageParam struct {
	Role             string        `json:"role"`
	Content          *string       `json:"content"`
	ReasoningContent *string       `json:"reasoning_content,omitempty"`
	FunctionCall     *FunctionCall `json:"function_call,omitempty"`
	ToolCalls        []ToolCall    `json:"tool_calls,omitempty"`
	Name             string        `json:"name,omitempty"`
	ToolCallID       string        `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type APIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Usage   Usage    `json:"usage"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Index        int          `json:"index"`
	Message      MessageParam `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
