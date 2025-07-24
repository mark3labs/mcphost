package hooks

// HookEvent represents a point in MCPHost's lifecycle where hooks can be executed
type HookEvent string

const (
	// PreToolUse fires before any tool execution
	PreToolUse HookEvent = "PreToolUse"

	// PostToolUse fires after tool execution completes
	PostToolUse HookEvent = "PostToolUse"

	// UserPromptSubmit fires when user submits a prompt
	UserPromptSubmit HookEvent = "UserPromptSubmit"

	// Stop fires when the main agent finishes responding
	Stop HookEvent = "Stop"
)

// IsValid returns true if the event is a valid hook event
func (e HookEvent) IsValid() bool {
	switch e {
	case PreToolUse, PostToolUse, UserPromptSubmit, Stop:
		return true
	}
	return false
}

// RequiresMatcher returns true if the event uses tool matchers
func (e HookEvent) RequiresMatcher() bool {
	return e == PreToolUse || e == PostToolUse
}
