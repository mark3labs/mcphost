package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExecuteHooks(t *testing.T) {
	// Create test scripts
	tmpDir := t.TempDir()

	// Simple echo script
	echoScript := filepath.Join(tmpDir, "echo.sh")
	if err := os.WriteFile(echoScript, []byte(`#!/bin/bash
cat
`), 0755); err != nil {
		t.Fatalf("failed to create echo script: %v", err)
	}

	// Blocking script (exit code 2)
	blockScript := filepath.Join(tmpDir, "block.sh")
	if err := os.WriteFile(blockScript, []byte(`#!/bin/bash
echo "Blocked by policy" >&2
exit 2
`), 0755); err != nil {
		t.Fatalf("failed to create block script: %v", err)
	}

	// JSON output script
	jsonScript := filepath.Join(tmpDir, "json.sh")
	if err := os.WriteFile(jsonScript, []byte(`#!/bin/bash
echo '{"decision": "approve", "reason": "Approved by test"}'
`), 0755); err != nil {
		t.Fatalf("failed to create json script: %v", err)
	}

	tests := []struct {
		name     string
		config   *HookConfig
		event    HookEvent
		input    interface{}
		expected *HookOutput
		wantErr  bool
	}{
		{
			name: "simple command execution",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {{
						Matcher: "bash",
						Hooks: []HookEntry{{
							Type:    "command",
							Command: echoScript,
						}},
					}},
				},
			},
			event: PreToolUse,
			input: &PreToolUseInput{
				CommonInput: CommonInput{HookEventName: PreToolUse},
				ToolName:    "bash",
			},
			expected: &HookOutput{},
		},
		{
			name: "blocking hook",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {{
						Matcher: "bash",
						Hooks: []HookEntry{{
							Type:    "command",
							Command: blockScript,
						}},
					}},
				},
			},
			event: PreToolUse,
			input: &PreToolUseInput{
				CommonInput: CommonInput{HookEventName: PreToolUse},
				ToolName:    "bash",
			},
			expected: &HookOutput{
				Decision: "block",
				Reason:   "Blocked by policy\n",
				Continue: boolPtr(false),
			},
		},
		{
			name: "JSON output parsing",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {{
						Matcher: "bash",
						Hooks: []HookEntry{{
							Type:    "command",
							Command: jsonScript,
						}},
					}},
				},
			},
			event: PreToolUse,
			input: &PreToolUseInput{
				CommonInput: CommonInput{HookEventName: PreToolUse},
				ToolName:    "bash",
			},
			expected: &HookOutput{
				Decision: "approve",
				Reason:   "Approved by test",
			},
		},
		{
			name: "timeout handling",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {{
						Matcher: "bash",
						Hooks: []HookEntry{{
							Type:    "command",
							Command: "sleep 10",
							Timeout: 1,
						}},
					}},
				},
			},
			event: PreToolUse,
			input: &PreToolUseInput{
				CommonInput: CommonInput{HookEventName: PreToolUse},
				ToolName:    "bash",
			},
			expected: &HookOutput{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewExecutor(tt.config, "test-session", "/tmp/test.jsonl")

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			got, err := executor.ExecuteHooks(ctx, tt.event, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Compare outputs
			if !compareHookOutputs(got, tt.expected) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				expectedJSON, _ := json.MarshalIndent(tt.expected, "", "  ")
				t.Errorf("ExecuteHooks() output mismatch:\ngot:\n%s\nwant:\n%s", gotJSON, expectedJSON)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func compareHookOutputs(a, b *HookOutput) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare Continue pointers
	if (a.Continue == nil) != (b.Continue == nil) {
		return false
	}
	if a.Continue != nil && *a.Continue != *b.Continue {
		return false
	}

	return a.StopReason == b.StopReason &&
		a.SuppressOutput == b.SuppressOutput &&
		a.Decision == b.Decision &&
		a.Reason == b.Reason &&
		a.Feedback == b.Feedback &&
		a.Context == b.Context &&
		a.SystemPrompt == b.SystemPrompt &&
		a.ModifyInput == b.ModifyInput &&
		a.ModifyOutput == b.ModifyOutput
}

func TestToolBlocking(t *testing.T) {
	// Create test script that blocks bash tool
	tmpDir := t.TempDir()

	blockBashScript := filepath.Join(tmpDir, "block_bash.sh")
	if err := os.WriteFile(blockBashScript, []byte(`#!/bin/bash
echo '{"decision": "block", "reason": "Bash commands are not allowed for security reasons"}'
`), 0755); err != nil {
		t.Fatalf("failed to create block bash script: %v", err)
	}

	config := &HookConfig{
		Hooks: map[HookEvent][]HookMatcher{
			PreToolUse: {{
				Matcher: "bash",
				Hooks: []HookEntry{{
					Type:    "command",
					Command: blockBashScript,
				}},
			}},
		},
	}

	executor := NewExecutor(config, "test-session", "/tmp/test.jsonl")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	input := &PreToolUseInput{
		CommonInput: CommonInput{HookEventName: PreToolUse},
		ToolName:    "bash",
		ToolInput:   json.RawMessage(`{"command": "ls -la"}`),
	}

	got, err := executor.ExecuteHooks(ctx, PreToolUse, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the hook blocked the tool
	if got == nil {
		t.Fatal("expected hook output, got nil")
	}

	if got.Decision != "block" {
		t.Errorf("expected decision 'block', got '%s'", got.Decision)
	}

	if got.Reason != "Bash commands are not allowed for security reasons" {
		t.Errorf("unexpected reason: %s", got.Reason)
	}

	// Continue field is optional for JSON output (only set for exit code 2)
}

func TestHookOutputLLMFeedback(t *testing.T) {
	// Create test scripts
	tmpDir := t.TempDir()

	// Script with feedback and context
	feedbackScript := filepath.Join(tmpDir, "feedback.sh")
	if err := os.WriteFile(feedbackScript, []byte(`#!/bin/bash
echo '{
	"feedback": "Tool execution was slow, consider optimization",
	"context": "Performance warning",
	"continue": true
}'
`), 0755); err != nil {
		t.Fatalf("failed to create feedback script: %v", err)
	}

	// Script with output modification
	modifyScript := filepath.Join(tmpDir, "modify.sh")
	if err := os.WriteFile(modifyScript, []byte(`#!/bin/bash
echo '{
	"modifyOutput": "{\"result\": \"sanitized output\"}",
	"suppressOutput": true
}'
`), 0755); err != nil {
		t.Fatalf("failed to create modify script: %v", err)
	}

	// Script with system prompt modification
	systemPromptScript := filepath.Join(tmpDir, "systemprompt.sh")
	if err := os.WriteFile(systemPromptScript, []byte(`#!/bin/bash
echo '{
	"systemPrompt": "Additional context: User is working on sensitive data",
	"context": "Security context applied"
}'
`), 0755); err != nil {
		t.Fatalf("failed to create system prompt script: %v", err)
	}

	tests := []struct {
		name     string
		config   *HookConfig
		event    HookEvent
		input    interface{}
		expected *HookOutput
	}{
		{
			name: "feedback and context",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PostToolUse: {{
						Matcher: "bash",
						Hooks: []HookEntry{{
							Type:    "command",
							Command: feedbackScript,
						}},
					}},
				},
			},
			event: PostToolUse,
			input: &PostToolUseInput{
				CommonInput:  CommonInput{HookEventName: PostToolUse},
				ToolName:     "bash",
				ToolResponse: json.RawMessage(`{"output": "test"}`),
			},
			expected: &HookOutput{
				Feedback: "Tool execution was slow, consider optimization",
				Context:  "Performance warning",
				Continue: boolPtr(true),
			},
		},
		{
			name: "modify output",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PostToolUse: {{
						Matcher: "bash",
						Hooks: []HookEntry{{
							Type:    "command",
							Command: modifyScript,
						}},
					}},
				},
			},
			event: PostToolUse,
			input: &PostToolUseInput{
				CommonInput:  CommonInput{HookEventName: PostToolUse},
				ToolName:     "bash",
				ToolResponse: json.RawMessage(`{"output": "sensitive data"}`),
			},
			expected: &HookOutput{
				ModifyOutput:   `{"result": "sanitized output"}`,
				SuppressOutput: true,
			},
		},
		{
			name: "system prompt modification",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					UserPromptSubmit: {{
						Matcher: ".*",
						Hooks: []HookEntry{{
							Type:    "command",
							Command: systemPromptScript,
						}},
					}},
				},
			},
			event: UserPromptSubmit,
			input: &UserPromptSubmitInput{
				CommonInput: CommonInput{HookEventName: UserPromptSubmit},
				Prompt:      "help me with this task",
			},
			expected: &HookOutput{
				SystemPrompt: "Additional context: User is working on sensitive data",
				Context:      "Security context applied",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewExecutor(tt.config, "test-session", "/tmp/test.jsonl")

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			got, err := executor.ExecuteHooks(ctx, tt.event, tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Compare outputs
			if !compareHookOutputs(got, tt.expected) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				expectedJSON, _ := json.MarshalIndent(tt.expected, "", "  ")
				t.Errorf("ExecuteHooks() output mismatch:\ngot:\n%s\nwant:\n%s", gotJSON, expectedJSON)
			}
		})
	}
}
