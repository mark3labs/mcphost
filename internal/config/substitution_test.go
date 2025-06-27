package config

import (
	"os"
	"testing"
)

func TestParseVariableWithDefault(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		expectedVar        string
		expectedDefault    string
		expectedHasDefault bool
	}{
		{
			name:               "variable without default",
			input:              "GITHUB_TOKEN",
			expectedVar:        "GITHUB_TOKEN",
			expectedDefault:    "",
			expectedHasDefault: false,
		},
		{
			name:               "variable with default",
			input:              "DEBUG:-false",
			expectedVar:        "DEBUG",
			expectedDefault:    "false",
			expectedHasDefault: true,
		},
		{
			name:               "variable with empty default",
			input:              "OPTIONAL:-",
			expectedVar:        "OPTIONAL",
			expectedDefault:    "",
			expectedHasDefault: true,
		},
		{
			name:               "variable with complex default",
			input:              "DATABASE_URL:-sqlite:///tmp/default.db",
			expectedVar:        "DATABASE_URL",
			expectedDefault:    "sqlite:///tmp/default.db",
			expectedHasDefault: true,
		},
		{
			name:               "variable with default containing colon",
			input:              "URL:-https://api.example.com:8080/path",
			expectedVar:        "URL",
			expectedDefault:    "https://api.example.com:8080/path",
			expectedHasDefault: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varName, defaultValue, hasDefault := parseVariableWithDefault(tt.input)

			if varName != tt.expectedVar {
				t.Errorf("Expected var name %s, got %s", tt.expectedVar, varName)
			}
			if defaultValue != tt.expectedDefault {
				t.Errorf("Expected default value %s, got %s", tt.expectedDefault, defaultValue)
			}
			if hasDefault != tt.expectedHasDefault {
				t.Errorf("Expected hasDefault %v, got %v", tt.expectedHasDefault, hasDefault)
			}
		})
	}
}

func TestEnvSubstituter_SubstituteEnvVars(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		envVars     map[string]string
		expected    string
		expectError bool
	}{
		{
			name:     "basic env substitution",
			input:    `{"token": "${env://GITHUB_TOKEN}"}`,
			envVars:  map[string]string{"GITHUB_TOKEN": "ghp_123"},
			expected: `{"token": "ghp_123"}`,
		},
		{
			name:     "env with default value used",
			input:    `{"debug": "${env://DEBUG:-false}"}`,
			envVars:  map[string]string{},
			expected: `{"debug": "false"}`,
		},
		{
			name:     "env with default value overridden",
			input:    `{"debug": "${env://DEBUG:-false}"}`,
			envVars:  map[string]string{"DEBUG": "true"},
			expected: `{"debug": "true"}`,
		},
		{
			name:     "env with empty default",
			input:    `{"optional": "${env://OPTIONAL:-}"}`,
			envVars:  map[string]string{},
			expected: `{"optional": ""}`,
		},
		{
			name:     "multiple env vars in same string",
			input:    `{"url": "${env://HOST:-localhost}:${env://PORT:-8080}"}`,
			envVars:  map[string]string{"HOST": "example.com"},
			expected: `{"url": "example.com:8080"}`,
		},
		{
			name:     "mixed env and script args (env processed first)",
			input:    `{"token": "${env://TOKEN:-default}", "name": "${username}"}`,
			envVars:  map[string]string{},
			expected: `{"token": "default", "name": "${username}"}`,
		},
		{
			name:     "complex default with special characters",
			input:    `{"db": "${env://DATABASE_URL:-sqlite:///tmp/default.db?cache=shared&mode=rwc}"}`,
			envVars:  map[string]string{},
			expected: `{"db": "sqlite:///tmp/default.db?cache=shared&mode=rwc"}`,
		},
		{
			name:     "no env vars in content",
			input:    `{"normal": "value", "script": "${arg}"}`,
			envVars:  map[string]string{},
			expected: `{"normal": "value", "script": "${arg}"}`,
		},
		{
			name:        "missing required env var",
			input:       `{"token": "${env://REQUIRED_TOKEN}"}`,
			envVars:     map[string]string{},
			expectError: true,
		},
		{
			name:        "multiple missing required env vars",
			input:       `{"token": "${env://TOKEN1}", "key": "${env://TOKEN2}"}`,
			envVars:     map[string]string{},
			expectError: true,
		},
		{
			name:     "yaml format",
			input:    "token: ${env://GITHUB_TOKEN:-default}\ndebug: ${env://DEBUG:-false}",
			envVars:  map[string]string{"GITHUB_TOKEN": "ghp_456"},
			expected: "token: ghp_456\ndebug: false",
		},
		{
			name:     "env var with underscores and numbers",
			input:    `{"var": "${env://MY_VAR_123}"}`,
			envVars:  map[string]string{"MY_VAR_123": "test_value"},
			expected: `{"var": "test_value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			originalEnv := make(map[string]string)
			for k, v := range tt.envVars {
				originalEnv[k] = os.Getenv(k)
				os.Setenv(k, v)
			}

			// Clean up environment variables after test
			defer func() {
				for k := range tt.envVars {
					if originalValue, existed := originalEnv[k]; existed {
						os.Setenv(k, originalValue)
					} else {
						os.Unsetenv(k)
					}
				}
			}()

			substituter := &EnvSubstituter{}
			result, err := substituter.SubstituteEnvVars(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
				}
			}
		})
	}
}

func TestArgsSubstituter_SubstituteArgs(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		args        map[string]string
		expected    string
		expectError bool
	}{
		{
			name:     "basic args substitution",
			input:    `{"name": "${username}"}`,
			args:     map[string]string{"username": "john"},
			expected: `{"name": "john"}`,
		},
		{
			name:     "args with default value used",
			input:    `{"type": "${repo_type:-public}"}`,
			args:     map[string]string{},
			expected: `{"type": "public"}`,
		},
		{
			name:     "args with default value overridden",
			input:    `{"type": "${repo_type:-public}"}`,
			args:     map[string]string{"repo_type": "private"},
			expected: `{"type": "private"}`,
		},
		{
			name:     "args with empty default",
			input:    `{"optional": "${optional_arg:-}"}`,
			args:     map[string]string{},
			expected: `{"optional": ""}`,
		},
		{
			name:     "multiple args in same string",
			input:    `{"message": "Hello ${name:-World}, you have ${count:-0} messages"}`,
			args:     map[string]string{"name": "Alice"},
			expected: `{"message": "Hello Alice, you have 0 messages"}`,
		},
		{
			name:     "no args in content",
			input:    `{"normal": "value", "env": "${env://TOKEN}"}`,
			args:     map[string]string{},
			expected: `{"normal": "value", "env": "${env://TOKEN}"}`,
		},
		{
			name:        "missing required arg",
			input:       `{"name": "${required_name}"}`,
			args:        map[string]string{},
			expectError: true,
		},
		{
			name:        "multiple missing required args",
			input:       `{"name": "${name}", "id": "${id}"}`,
			args:        map[string]string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			substituter := NewArgsSubstituter(tt.args)
			result, err := substituter.SubstituteArgs(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
				}
			}
		})
	}
}

func TestHasEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "has env vars",
			content:  `{"token": "${env://GITHUB_TOKEN}"}`,
			expected: true,
		},
		{
			name:     "has env vars with default",
			content:  `{"debug": "${env://DEBUG:-false}"}`,
			expected: true,
		},
		{
			name:     "no env vars",
			content:  `{"name": "${username}", "normal": "value"}`,
			expected: false,
		},
		{
			name:     "empty content",
			content:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasEnvVars(tt.content)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHasScriptArgs(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "has script args",
			content:  `{"name": "${username}"}`,
			expected: true,
		},
		{
			name:     "has script args with default",
			content:  `{"type": "${repo_type:-public}"}`,
			expected: true,
		},
		{
			name:     "no script args",
			content:  `{"token": "${env://GITHUB_TOKEN}", "normal": "value"}`,
			expected: false,
		},
		{
			name:     "empty content",
			content:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasScriptArgs(tt.content)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIntegrationEnvAndArgsSubstitution(t *testing.T) {
	// Test that env substitution and args substitution work together correctly
	input := `{
		"token": "${env://GITHUB_TOKEN:-default_token}",
		"user": "${username}",
		"repo": "${repo_name:-my-repo}",
		"debug": "${env://DEBUG:-false}"
	}`

	// Set up environment
	os.Setenv("GITHUB_TOKEN", "ghp_real_token")
	defer os.Unsetenv("GITHUB_TOKEN")

	// Step 1: Apply env substitution
	envSubstituter := &EnvSubstituter{}
	afterEnv, err := envSubstituter.SubstituteEnvVars(input)
	if err != nil {
		t.Fatalf("Env substitution failed: %v", err)
	}

	expectedAfterEnv := `{
		"token": "ghp_real_token",
		"user": "${username}",
		"repo": "${repo_name:-my-repo}",
		"debug": "false"
	}`

	if afterEnv != expectedAfterEnv {
		t.Errorf("After env substitution, expected:\n%s\nGot:\n%s", expectedAfterEnv, afterEnv)
	}

	// Step 2: Apply args substitution
	args := map[string]string{"username": "alice"}
	argsSubstituter := NewArgsSubstituter(args)
	final, err := argsSubstituter.SubstituteArgs(afterEnv)
	if err != nil {
		t.Fatalf("Args substitution failed: %v", err)
	}

	expectedFinal := `{
		"token": "ghp_real_token",
		"user": "alice",
		"repo": "my-repo",
		"debug": "false"
	}`

	if final != expectedFinal {
		t.Errorf("After args substitution, expected:\n%s\nGot:\n%s", expectedFinal, final)
	}
}
