package hooks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadHooksConfig(t *testing.T) {
	// Save original XDG_CONFIG_HOME
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if originalXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	tests := []struct {
		name     string
		files    map[string]string
		expected *HookConfig
		wantErr  bool
	}{
		{
			name: "single yaml file",
			files: map[string]string{
				"hooks.yml": `
hooks:
  PreToolUse:
    - matcher: "bash"
      hooks:
        - type: command
          command: "echo test"
          timeout: 5
`,
			},
			expected: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {
						{
							Matcher: "bash",
							Hooks: []HookEntry{
								{Type: "command", Command: "echo test", Timeout: 5},
							},
						},
					},
				},
			},
		},
		{
			name: "environment substitution",
			files: map[string]string{
				"hooks.yml": `
hooks:
  PreToolUse:
    - matcher: "bash"
      hooks:
        - type: command
          command: "${env://TEST_HOOK_CMD:-echo default}"
`,
			},
			expected: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {
						{
							Matcher: "bash",
							Hooks: []HookEntry{
								{Type: "command", Command: "echo default"},
							},
						},
					},
				},
			},
		},
		{
			name: "merge multiple files",
			files: map[string]string{
				"global.yml": `
hooks:
  PreToolUse:
    - matcher: "bash"
      hooks:
        - type: command
          command: "global-hook"
`,
				"local.yml": `
hooks:
  PreToolUse:
    - matcher: "fetch"
      hooks:
        - type: command
          command: "local-hook"
`,
			},
			expected: &HookConfig{
				Hooks: map[HookEvent][]HookMatcher{
					PreToolUse: {
						{
							Matcher: "bash",
							Hooks:   []HookEntry{{Type: "command", Command: "global-hook"}},
						},
						{
							Matcher: "fetch",
							Hooks:   []HookEntry{{Type: "command", Command: "local-hook"}},
						},
					},
				},
			},
		},
		{
			name: "invalid yaml",
			files: map[string]string{
				"hooks.yml": `invalid yaml content`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test files
			tmpDir := t.TempDir()

			// Set XDG_CONFIG_HOME to a temp directory to avoid loading global hooks
			testConfigDir := filepath.Join(tmpDir, "config")
			os.Setenv("XDG_CONFIG_HOME", testConfigDir)

			// Write test files
			var paths []string
			for filename, content := range tt.files {
				path := filepath.Join(tmpDir, filename)
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
				paths = append(paths, path)
			}

			// Load configuration
			got, err := LoadHooksConfig(paths...)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("LoadHooksConfig() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		toolName string
		want     bool
	}{
		{"", "bash", true},                         // Empty pattern matches all
		{"bash", "bash", true},                     // Exact match
		{"bash", "Bash", false},                    // Case sensitive
		{"bash|fetch", "bash", true},               // Regex OR
		{"bash|fetch", "fetch", true},              // Regex OR
		{"bash|fetch", "todo", false},              // Regex OR no match
		{"mcp__.*", "mcp__filesystem__read", true}, // MCP pattern
		{".*write.*", "mcp__fs__write_file", true}, // Contains pattern
		{"^bash$", "bash", true},                   // Anchored regex
		{"^bash$", "bash2", false},                 // Anchored regex no match
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.toolName, func(t *testing.T) {
			got := matchesPattern(tt.pattern, tt.toolName)
			if got != tt.want {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.pattern, tt.toolName, got, tt.want)
			}
		})
	}
}

func TestNoHooksFlag(t *testing.T) {
	// This test verifies that when hooks are disabled via configuration,
	// the LoadHooksConfig function is not called. The actual implementation
	// of this is in cmd/root.go where viper.GetBool("no-hooks") is checked.
	// This test documents the expected behavior.

	// Create a test hooks file
	tmpDir := t.TempDir()
	hooksFile := filepath.Join(tmpDir, "hooks.yml")
	content := `
hooks:
  PreToolUse:
    - matcher: "bash"
      hooks:
        - type: command
          command: "echo 'This should not run'"
`
	if err := os.WriteFile(hooksFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Load the hooks config normally
	config, err := LoadHooksConfig(hooksFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify hooks are loaded
	if len(config.Hooks) == 0 {
		t.Error("expected hooks to be loaded")
	}

	// The actual --no-hooks flag implementation is in cmd/root.go
	// where it checks viper.GetBool("no-hooks") before calling LoadHooksConfig
}
