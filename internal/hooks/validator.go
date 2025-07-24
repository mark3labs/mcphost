package hooks

import (
	"fmt"
	"regexp"
	"strings"
)

// Security patterns to detect potentially dangerous commands
var (
	commandInjectionPattern    = regexp.MustCompile(`[;&|]|\$\(|` + "`")
	pathTraversalPattern       = regexp.MustCompile(`\.\.\/`)
	commandSubstitutionPattern = regexp.MustCompile(`\$\([^)]+\)|` + "`" + `[^` + "`" + `]+` + "`")
)

// validateHookCommand validates a hook command for security issues
func validateHookCommand(command string) error {
	if command == "" {
		return fmt.Errorf("empty command")
	}

	// Check for command injection attempts
	if commandInjectionPattern.MatchString(command) {
		// Allow simple pipes and redirects, but check for dangerous patterns
		if containsDangerousPattern(command) {
			return fmt.Errorf("potential command injection detected")
		}
	}

	// Check for path traversal
	if pathTraversalPattern.MatchString(command) {
		return fmt.Errorf("path traversal detected")
	}

	// Check for command substitution
	if commandSubstitutionPattern.MatchString(command) {
		return fmt.Errorf("command substitution detected")
	}

	return nil
}

// containsDangerousPattern checks for specific dangerous command patterns
func containsDangerousPattern(command string) bool {
	dangerousPatterns := []string{
		"; rm ",
		"&& rm ",
		"| rm ",
		"; dd ",
		"&& dd ",
		"| dd ",
		"/dev/null 2>&1",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(command, pattern) {
			return true
		}
	}

	// Check for multiple command separators which might indicate injection
	separatorCount := 0
	for _, sep := range []string{";", "&&", "||", "|"} {
		separatorCount += strings.Count(command, sep)
	}

	// Allow up to 2 separators for reasonable command chaining
	return separatorCount > 2
}

// ValidateHookConfig validates the entire hook configuration
func ValidateHookConfig(config *HookConfig) error {
	if config == nil {
		return fmt.Errorf("nil configuration")
	}

	for event, matchers := range config.Hooks {
		if !event.IsValid() {
			return fmt.Errorf("invalid event: %s", event)
		}

		for i, matcher := range matchers {
			// Validate regex pattern if provided
			if matcher.Matcher != "" {
				if _, err := regexp.Compile(matcher.Matcher); err != nil {
					return fmt.Errorf("invalid regex pattern in matcher %d for event %s: %w", i, event, err)
				}
			}

			// Validate hooks
			if len(matcher.Hooks) == 0 {
				return fmt.Errorf("no hooks defined for matcher %d in event %s", i, event)
			}

			for j, hook := range matcher.Hooks {
				if err := validateHookEntry(hook); err != nil {
					return fmt.Errorf("invalid hook %d in matcher %d for event %s: %w", j, i, event, err)
				}
			}
		}
	}

	return nil
}

// validateHookEntry validates a single hook entry
func validateHookEntry(hook HookEntry) error {
	if hook.Type != "command" {
		return fmt.Errorf("invalid hook type: %s (only 'command' is supported)", hook.Type)
	}

	if hook.Command == "" {
		return fmt.Errorf("empty command")
	}

	// Basic security validation
	if err := validateHookCommand(hook.Command); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	if hook.Timeout < 0 {
		return fmt.Errorf("negative timeout: %d", hook.Timeout)
	}

	if hook.Timeout > 600 { // 10 minutes max
		return fmt.Errorf("timeout too large: %d (max 600 seconds)", hook.Timeout)
	}

	return nil
}
