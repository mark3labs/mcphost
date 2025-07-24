package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

// Executor handles hook execution
type Executor struct {
	config      *HookConfig
	sessionID   string
	transcript  string
	model       string
	interactive bool
	mu          sync.RWMutex
}

// NewExecutor creates a new hook executor
func NewExecutor(config *HookConfig, sessionID, transcriptPath string) *Executor {
	return &Executor{
		config:     config,
		sessionID:  sessionID,
		transcript: transcriptPath,
	}
}

// SetModel sets the model name for hook context
func (e *Executor) SetModel(model string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.model = model
}

// SetInteractive sets whether we're in interactive mode
func (e *Executor) SetInteractive(interactive bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.interactive = interactive
}

// PopulateCommonFields fills in the common fields for any hook input
func (e *Executor) PopulateCommonFields(event HookEvent) CommonInput {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cwd, _ := os.Getwd()
	return CommonInput{
		SessionID:      e.sessionID,
		TranscriptPath: e.transcript,
		CWD:            cwd,
		HookEventName:  event,
		Timestamp:      time.Now().Unix(),
		Model:          e.model,
		Interactive:    e.interactive,
	}
}

// ExecuteHooks runs all matching hooks for an event
func (e *Executor) ExecuteHooks(ctx context.Context, event HookEvent, input interface{}) (*HookOutput, error) {
	matchers, ok := e.config.Hooks[event]
	if !ok || len(matchers) == 0 {
		return nil, nil
	}

	// Get tool name if applicable
	toolName := ""
	if event.RequiresMatcher() {
		toolName = extractToolName(input)
	}

	// Find matching hooks
	var hooksToRun []HookEntry
	for _, matcher := range matchers {
		if matchesPattern(matcher.Matcher, toolName) {
			hooksToRun = append(hooksToRun, matcher.Hooks...)
		}
	}

	if len(hooksToRun) == 0 {
		return nil, nil
	}

	// Execute hooks in parallel
	results := make(chan *hookResult, len(hooksToRun))
	var wg sync.WaitGroup

	for _, hook := range hooksToRun {
		wg.Add(1)
		go func(h HookEntry) {
			defer wg.Done()
			result := e.executeHook(ctx, h, input)
			results <- result
		}(hook)
	}

	wg.Wait()
	close(results)

	// Process results
	return e.processResults(results)
}

// executeHook runs a single hook command
func (e *Executor) executeHook(ctx context.Context, hook HookEntry, input interface{}) *hookResult {
	// Prepare input JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return &hookResult{err: fmt.Errorf("marshaling input: %w", err)}
	}

	// Set timeout
	timeout := time.Duration(hook.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(ctx, "sh", "-c", hook.Command)
	cmd.Stdin = bytes.NewReader(inputJSON)
	cmd.Dir = getCurrentWorkingDir()

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	err = cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return &hookResult{
		exitCode: exitCode,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		err:      err,
	}
}

// matchesPattern checks if a tool name matches a pattern
func matchesPattern(pattern, toolName string) bool {
	if pattern == "" {
		return true // Empty pattern matches all
	}

	// Try exact match first
	if pattern == toolName {
		return true
	}

	// Try regex match
	matched, err := regexp.MatchString(pattern, toolName)
	if err != nil {
		// Invalid regex pattern, return false
		return false
	}

	return matched
}

// extractToolName gets the tool name from various input types
func extractToolName(input interface{}) string {
	switch v := input.(type) {
	case *PreToolUseInput:
		return v.ToolName
	case *PostToolUseInput:
		return v.ToolName
	default:
		return ""
	}
}

type hookResult struct {
	exitCode int
	stdout   string
	stderr   string
	err      error
}

// processResults combines results from multiple hooks
func (e *Executor) processResults(results <-chan *hookResult) (*HookOutput, error) {
	var finalOutput HookOutput

	for result := range results {
		if result.err != nil && result.exitCode != 2 {
			// Hook execution failed, skip this result
			continue
		}

		// Handle exit code 2 (blocking error)
		if result.exitCode == 2 {
			finalOutput.Decision = "block"
			finalOutput.Reason = result.stderr
			continueVal := false
			finalOutput.Continue = &continueVal
			return &finalOutput, nil
		}

		// Try to parse JSON output
		if result.stdout != "" {
			var output HookOutput
			if err := json.Unmarshal([]byte(result.stdout), &output); err == nil {
				// Merge outputs (later hooks can override)
				mergeHookOutputs(&finalOutput, &output)
			}
		}
	}

	return &finalOutput, nil
}

// mergeHookOutputs combines two hook outputs
func mergeHookOutputs(dst, src *HookOutput) {
	if src.Continue != nil {
		dst.Continue = src.Continue
	}
	if src.StopReason != "" {
		dst.StopReason = src.StopReason
	}
	if src.Decision != "" {
		dst.Decision = src.Decision
	}
	if src.Reason != "" {
		dst.Reason = src.Reason
	}
	if src.SuppressOutput {
		dst.SuppressOutput = true
	}
}

func getCurrentWorkingDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "/"
	}
	return cwd
}
