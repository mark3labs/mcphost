package hooks

import (
	"strings"
	"testing"
)

func TestValidateHookCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "simple command",
			command: "echo hello",
			wantErr: false,
		},
		{
			name:    "absolute path",
			command: "/usr/local/bin/validator.py",
			wantErr: false,
		},
		{
			name:    "command injection attempt",
			command: "echo test; rm -rf /",
			wantErr: true,
			errMsg:  "potential command injection",
		},
		{
			name:    "path traversal",
			command: "cat ../../../etc/passwd",
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "command substitution",
			command: "echo $(/bin/sh -c 'malicious')",
			wantErr: true,
			errMsg:  "command substitution detected",
		},
		{
			name:    "backtick substitution",
			command: "echo `whoami`",
			wantErr: true,
			errMsg:  "command substitution detected",
		},
		{
			name:    "empty command",
			command: "",
			wantErr: true,
			errMsg:  "empty command",
		},
		{
			name:    "simple pipe allowed",
			command: "ps aux | grep process",
			wantErr: false,
		},
		{
			name:    "too many command separators",
			command: "cmd1 | cmd2 && cmd3 ; cmd4",
			wantErr: true,
			errMsg:  "potential command injection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHookCommand(tt.command)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message %q does not contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateHookConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *HookConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {
						{
							Matcher: "bash",
							Hooks: []HookEntry{
								{Type: "command", Command: "echo test"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "nil configuration",
		},
		{
			name: "invalid event",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					"InvalidEvent": {
						{
							Hooks: []HookEntry{
								{Type: "command", Command: "echo test"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid event",
		},
		{
			name: "invalid regex pattern",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {
						{
							Matcher: "[invalid",
							Hooks: []HookEntry{
								{Type: "command", Command: "echo test"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid regex pattern",
		},
		{
			name: "no hooks defined",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {
						{
							Matcher: "bash",
							Hooks:   []HookEntry{},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "no hooks defined",
		},
		{
			name: "invalid hook type",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {
						{
							Hooks: []HookEntry{
								{Type: "invalid", Command: "echo test"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid hook type",
		},
		{
			name: "empty command",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {
						{
							Hooks: []HookEntry{
								{Type: "command", Command: ""},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "empty command",
		},
		{
			name: "negative timeout",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {
						{
							Hooks: []HookEntry{
								{Type: "command", Command: "echo test", Timeout: -1},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "negative timeout",
		},
		{
			name: "timeout too large",
			config: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {
						{
							Hooks: []HookEntry{
								{Type: "command", Command: "echo test", Timeout: 700},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "timeout too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHookConfig(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message %q does not contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
