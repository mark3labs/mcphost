package hooks

import (
	"encoding/json"
)

// CommonInput contains fields common to all hook inputs
type CommonInput struct {
	SessionID      string    `json:"session_id"`      // Unique session identifier
	TranscriptPath string    `json:"transcript_path"` // Path to transcript file (if enabled)
	CWD            string    `json:"cwd"`             // Current working directory
	HookEventName  HookEvent `json:"hook_event_name"` // The hook event type
	Timestamp      int64     `json:"timestamp"`       // Unix timestamp when hook fired
	Model          string    `json:"model"`           // AI model being used
	Interactive    bool      `json:"interactive"`     // Whether in interactive mode
}

// PreToolUseInput is passed to PreToolUse hooks
type PreToolUseInput struct {
	CommonInput
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// PostToolUseInput is passed to PostToolUse hooks
type PostToolUseInput struct {
	CommonInput
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

// UserPromptSubmitInput is passed to UserPromptSubmit hooks
type UserPromptSubmitInput struct {
	CommonInput
	Prompt string `json:"prompt"`
}

// StopInput is passed to Stop hooks
type StopInput struct {
	CommonInput
	StopHookActive bool            `json:"stop_hook_active"`
	Response       string          `json:"response"`       // The agent's final response
	StopReason     string          `json:"stop_reason"`    // "completed", "cancelled", "error"
	Meta           json.RawMessage `json:"meta,omitempty"` // Additional metadata (e.g., token usage, model info)
}

// HookOutput represents the JSON output from a hook
type HookOutput struct {
	Continue       *bool  `json:"continue,omitempty"`
	StopReason     string `json:"stopReason,omitempty"`
	SuppressOutput bool   `json:"suppressOutput,omitempty"`
	Decision       string `json:"decision,omitempty"` // "approve", "block", or ""
	Reason         string `json:"reason,omitempty"`
}
