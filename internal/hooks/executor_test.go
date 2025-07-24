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
		a.Reason == b.Reason
}
