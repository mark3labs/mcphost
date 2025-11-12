package hooks

// HookEvent represents a point in MCPHost's lifecycle where hooks can be executed.
// Events can be tool-related (requiring matchers) or lifecycle-related.
type HookEvent string

const (
	// PreToolUse fires before any tool execution, allowing pre-processing or validation
	PreToolUse HookEvent = "PreToolUse"

	// PostToolUse fires after tool execution completes, allowing post-processing or logging
	PostToolUse HookEvent = "PostToolUse"

	// UserPromptSubmit fires when user submits a prompt, before agent processing
	UserPromptSubmit HookEvent = "UserPromptSubmit"

	// Stop fires when the main agent finishes responding to a user prompt
	Stop HookEvent = "Stop"
)

// IsValid returns true if the event is a valid hook event.
// Valid events are PreToolUse, PostToolUse, UserPromptSubmit, and Stop.
func (e HookEvent) IsValid() bool {
	switch e {
	case PreToolUse, PostToolUse, UserPromptSubmit, Stop:
		return true
	}
	return false
}

// RequiresMatcher returns true if the event uses tool matchers.
// PreToolUse and PostToolUse events require matchers to determine which
// tools trigger the hooks. Other events apply globally without matchers.
func (e HookEvent) RequiresMatcher() bool {
	return e == PreToolUse || e == PostToolUse
}
