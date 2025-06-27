package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptWithEnvAndArgsSubstitution(t *testing.T) {
	// Create a temporary script file with both env vars and script args
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "test-script.sh")

	scriptContent := `#!/usr/bin/env -S mcphost script
---
mcpServers:
  github:
    type: local
    command: ["gh", "api"]
    environment:
      GITHUB_TOKEN: "${env://GITHUB_TOKEN}"
      DEBUG: "${env://DEBUG:-false}"
  
  filesystem:
    type: builtin
    name: fs
    options:
      allowed_directories: ["${env://WORK_DIR:-/tmp}"]

model: "${env://MODEL:-anthropic:claude-sonnet-4-20250514}"
debug: ${env://DEBUG:-false}
---
List ${repo_type:-public} repositories for user ${username}.
Use the GitHub API to fetch ${count:-10} repositories.
Working directory is ${env://WORK_DIR:-/tmp}.
`

	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	// Set up environment variables
	os.Setenv("GITHUB_TOKEN", "ghp_test_token")
	os.Setenv("DEBUG", "true")
	os.Setenv("WORK_DIR", "/home/user/projects")
	defer func() {
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("DEBUG")
		os.Unsetenv("WORK_DIR")
	}()

	// Set up script arguments
	variables := map[string]string{
		"username":  "alice",
		"repo_type": "private",
	}

	// Parse the script
	scriptConfig, err := parseScriptFile(scriptPath, variables)
	if err != nil {
		t.Fatalf("Failed to parse script: %v", err)
	}

	// Verify environment variable substitution in MCP servers
	githubServer, exists := scriptConfig.MCPServers["github"]
	if !exists {
		t.Fatal("GitHub server not found in script config")
	}

	if githubServer.Environment["GITHUB_TOKEN"] != "ghp_test_token" {
		t.Errorf("Expected GITHUB_TOKEN=ghp_test_token, got %s", githubServer.Environment["GITHUB_TOKEN"])
	}
	if githubServer.Environment["DEBUG"] != "true" {
		t.Errorf("Expected DEBUG=true, got %s", githubServer.Environment["DEBUG"])
	}

	// Verify environment variable substitution in builtin server options
	fsServer, exists := scriptConfig.MCPServers["filesystem"]
	if !exists {
		t.Fatal("Filesystem server not found in script config")
	}

	allowedDirs, ok := fsServer.Options["allowed_directories"].([]interface{})
	if !ok {
		t.Fatal("allowed_directories should be an array")
	}
	if len(allowedDirs) != 1 || allowedDirs[0] != "/home/user/projects" {
		t.Errorf("Expected allowed_directories=[/home/user/projects], got %v", allowedDirs)
	}

	// Verify global config values
	if scriptConfig.Model != "anthropic:claude-sonnet-4-20250514" {
		t.Errorf("Expected model=anthropic:claude-sonnet-4-20250514, got %s", scriptConfig.Model)
	}
	if !scriptConfig.Debug {
		t.Error("Expected debug=true")
	}

	// Verify script args substitution in prompt
	expectedPrompt := `List private repositories for user alice.
Use the GitHub API to fetch 10 repositories.
Working directory is /home/user/projects.`

	if strings.TrimSpace(scriptConfig.Prompt) != strings.TrimSpace(expectedPrompt) {
		t.Errorf("Expected prompt:\n%s\nGot:\n%s", expectedPrompt, scriptConfig.Prompt)
	}
}

func TestScriptWithMissingRequiredEnvVar(t *testing.T) {
	// Create a script with a required environment variable
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "test-script.sh")

	scriptContent := `#!/usr/bin/env -S mcphost script
---
mcpServers:
  github:
    type: local
    command: ["gh", "api"]
    environment:
      GITHUB_TOKEN: "${env://REQUIRED_TOKEN}"
---
Test script with required env var.
`

	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	// Make sure the environment variable is not set
	os.Unsetenv("REQUIRED_TOKEN")

	// Parse the script - should fail
	variables := map[string]string{}
	_, err = parseScriptFile(scriptPath, variables)
	if err == nil {
		t.Fatal("Expected error for missing required environment variable")
	}

	if !strings.Contains(err.Error(), "required environment variable REQUIRED_TOKEN not set") {
		t.Errorf("Expected error about missing REQUIRED_TOKEN, got: %v", err)
	}
}

func TestScriptWithMissingRequiredArg(t *testing.T) {
	// Create a script with a required script argument
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "test-script.sh")

	scriptContent := `#!/usr/bin/env -S mcphost script
---
mcpServers:
  github:
    type: local
    command: ["gh", "api"]
    environment:
      GITHUB_TOKEN: "${env://GITHUB_TOKEN:-default_token}"
---
List repositories for user ${required_username}.
`

	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	// Parse the script without providing the required argument
	variables := map[string]string{}
	_, err = parseScriptFile(scriptPath, variables)
	if err == nil {
		t.Fatal("Expected error for missing required script argument")
	}

	if !strings.Contains(err.Error(), "required_username") {
		t.Errorf("Expected error about missing required_username, got: %v", err)
	}
}

func TestScriptProcessingOrder(t *testing.T) {
	// Test that env substitution happens before args substitution
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "test-script.sh")

	// This script tests that env vars are processed first, then script args
	scriptContent := `#!/usr/bin/env -S mcphost script
---
mcpServers:
  test:
    type: local
    command: ["echo", "${env://BASE_PATH:-/tmp}/${path_suffix}"]
---
Base path is ${env://BASE_PATH:-/tmp} and suffix is ${path_suffix:-default}.
`

	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	// Set environment variable
	os.Setenv("BASE_PATH", "/home/user")
	defer os.Unsetenv("BASE_PATH")

	// Set script argument
	variables := map[string]string{
		"path_suffix": "documents",
	}

	// Parse the script
	scriptConfig, err := parseScriptFile(scriptPath, variables)
	if err != nil {
		t.Fatalf("Failed to parse script: %v", err)
	}

	// Verify that both substitutions worked correctly
	testServer := scriptConfig.MCPServers["test"]
	expectedCommand := []string{"echo", "/home/user/documents"}

	if len(testServer.Command) != len(expectedCommand) {
		t.Errorf("Expected command length %d, got %d", len(expectedCommand), len(testServer.Command))
	}

	for i, expected := range expectedCommand {
		if i < len(testServer.Command) && testServer.Command[i] != expected {
			t.Errorf("Expected command[%d] = %s, got %s", i, expected, testServer.Command[i])
		}
	}

	// Verify prompt substitution
	expectedPrompt := "Base path is /home/user and suffix is documents."
	if strings.TrimSpace(scriptConfig.Prompt) != expectedPrompt {
		t.Errorf("Expected prompt: %s\nGot: %s", expectedPrompt, strings.TrimSpace(scriptConfig.Prompt))
	}
}

func TestScriptBackwardCompatibility(t *testing.T) {
	// Test that existing scripts without env vars still work
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "test-script.sh")

	scriptContent := `#!/usr/bin/env -S mcphost script
---
mcpServers:
  filesystem:
    type: builtin
    name: fs
    options:
      allowed_directories: ["/tmp"]

model: "anthropic:claude-sonnet-4-20250514"
---
List files in ${directory:-/tmp} for user ${username}.
`

	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	// Set script arguments
	variables := map[string]string{
		"username": "bob",
	}

	// Parse the script
	scriptConfig, err := parseScriptFile(scriptPath, variables)
	if err != nil {
		t.Fatalf("Failed to parse script: %v", err)
	}

	// Verify that script args substitution still works
	expectedPrompt := "List files in /tmp for user bob."
	if strings.TrimSpace(scriptConfig.Prompt) != expectedPrompt {
		t.Errorf("Expected prompt: %s\nGot: %s", expectedPrompt, strings.TrimSpace(scriptConfig.Prompt))
	}

	// Verify that config is unchanged
	if scriptConfig.Model != "anthropic:claude-sonnet-4-20250514" {
		t.Errorf("Expected model unchanged, got %s", scriptConfig.Model)
	}
}
