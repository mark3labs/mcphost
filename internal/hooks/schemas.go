package hooks

import (
	"encoding/json"
)

// CommonInput contains fields common to all hook inputs, providing context
// information that is available to every hook regardless of the event type.
// These fields help hooks understand the execution environment and session state.
type CommonInput struct {
	SessionID      string    `json:"session_id"`      // Unique session identifier
	TranscriptPath string    `json:"transcript_path"` // Path to transcript file (if enabled)
	CWD            string    `json:"cwd"`             // Current working directory
	HookEventName  HookEvent `json:"hook_event_name"` // The hook event type
	Timestamp      int64     `json:"timestamp"`       // Unix timestamp when hook fired
	Model          string    `json:"model"`           // AI model being used
	Interactive    bool      `json:"interactive"`     // Whether in interactive mode
}

// PreToolUseInput is passed to PreToolUse hooks before a tool is executed.
// It contains the tool name and input parameters, allowing hooks to validate,
// modify, or block tool execution.
type PreToolUseInput struct {
	CommonInput
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// PostToolUseInput is passed to PostToolUse hooks after a tool has been executed.
// It contains the tool name, input parameters, and the tool's response, allowing
// hooks to log, analyze, or react to tool execution results.
type PostToolUseInput struct {
	CommonInput
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

// UserPromptSubmitInput is passed to UserPromptSubmit hooks when a user submits
// a prompt. It contains the user's input text, allowing hooks to validate,
// modify, or log user interactions before processing.
type UserPromptSubmitInput struct {
	CommonInput
	Prompt string `json:"prompt"`
}

// StopInput is passed to Stop hooks when the agent finishes responding to a prompt.
// It contains the final response, completion reason, and optional metadata about
// the interaction, allowing hooks to perform cleanup or logging operations.
type StopInput struct {
	CommonInput
	StopHookActive bool            `json:"stop_hook_active"`
	Response       string          `json:"response"`       // The agent's final response
	StopReason     string          `json:"stop_reason"`    // "completed", "cancelled", "error"
	Meta           json.RawMessage `json:"meta,omitempty"` // Additional metadata (e.g., token usage, model info)
}

// HookOutput represents the JSON output from a hook that controls MCPHost behavior.
// Hooks can decide whether to continue execution, provide reasons for stopping,
// suppress output, or block tool execution. The Decision field can be "approve",
// "block", or empty (default behavior).
type HookOutput struct {
	Continue       *bool  `json:"continue,omitempty"`
	StopReason     string `json:"stopReason,omitempty"`
	SuppressOutput bool   `json:"suppressOutput,omitempty"`
	Decision       string `json:"decision,omitempty"` // "approve", "block", or ""
	Reason         string `json:"reason,omitempty"`
}
