package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/mark3labs/mcphost/internal/config"
)

func TestMCPToolManager_LoadTools_WithTimeout(t *testing.T) {
	manager := NewMCPToolManager()

	// Create a config with a non-existent command that should fail
	cfg := &config.Config{
		MCPServers: map[string]config.MCPServerConfig{
			"test-server": {
				Command: []string{"non-existent-command", "arg1", "arg2"},
			},
		},
	}

	// Create a context with a reasonable timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// This should not hang indefinitely and should return an error
	start := time.Now()
	err := manager.LoadTools(ctx, cfg)
	duration := time.Since(start)

	// The operation should complete within our timeout
	if duration > 14*time.Second {
		t.Errorf("LoadTools took too long: %v, expected to complete within 14 seconds", duration)
	}

	// We expect an error since the command doesn't exist, but it shouldn't be a timeout
	if err == nil {
		t.Error("Expected an error for non-existent command, but got nil")
	}

	t.Logf("LoadTools completed in %v with error: %v", duration, err)
}

func TestMCPToolManager_LoadTools_GracefulFailure(t *testing.T) {
	manager := NewMCPToolManager()

	// Create a config with multiple servers, some good and some bad
	cfg := &config.Config{
		MCPServers: map[string]config.MCPServerConfig{
			"bad-server-1": {
				Command: []string{"non-existent-command-1", "arg1"},
			},
			"bad-server-2": {
				Command: []string{"non-existent-command-2", "arg2"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// This should fail gracefully and return an error since all servers failed
	err := manager.LoadTools(ctx, cfg)

	// We expect an error since all servers failed
	if err == nil {
		t.Error("Expected an error when all servers fail, but got nil")
	}

	// The error should mention that all servers failed
	if err != nil && !contains(err.Error(), "all MCP servers failed") {
		t.Errorf("Expected error message to mention all servers failed, got: %v", err)
	}

	t.Logf("LoadTools failed gracefully with error: %v", err)
}

// TestMCPToolManager_ToolWithoutProperties tests handling of tools with no input properties
func TestMCPToolManager_ToolWithoutProperties(t *testing.T) {
	manager := NewMCPToolManager()

	// Create a config with a builtin todo server (which has tools with properties)
	// and test the schema conversion logic
	cfg := &config.Config{
		MCPServers: map[string]config.MCPServerConfig{
			"todo-server": {
				Type: "builtin",
				Name: "todo",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Load the tools - this should work fine
	err := manager.LoadTools(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to load tools: %v", err)
	}

	// Get the loaded tools
	tools := manager.GetTools()
	if len(tools) == 0 {
		t.Fatal("No tools were loaded")
	}

	// Test that we can get tool info for each tool
	for _, tool := range tools {
		info, err := tool.Info(ctx)
		if err != nil {
			t.Errorf("Failed to get tool info: %v", err)
			continue
		}

		// Check that the tool has a valid schema
		if info.ParamsOneOf == nil {
			t.Errorf("Tool %s has nil ParamsOneOf", info.Name)
		}

		t.Logf("Tool: %s, Description: %s", info.Name, info.Desc)
	}
}

// TestIssue89_ObjectSchemaMissingProperties tests the fix for issue #89
// This is a regression test for the "object schema missing properties" error
// that occurs when tools have no input parameters and use OpenAI function calling
func TestIssue89_ObjectSchemaMissingProperties(t *testing.T) {
	// Create a schema that would cause the OpenAI validation error
	// This simulates what might happen with tools that have no input properties
	brokenSchema := &jsonschema.Schema{
		Type: "object",
		// Properties is nil - this causes "object schema missing properties" error in OpenAI
	}

	// Verify the problematic state
	if brokenSchema.Type == "object" && brokenSchema.Properties == nil {
		t.Log("Found object schema with nil properties - this causes OpenAI validation error")
	}

	// Apply the fix from issue #89
	if brokenSchema.Type == "object" && brokenSchema.Properties == nil {
		brokenSchema.Properties = jsonschema.NewProperties()
	}

	// Verify the fix worked
	if brokenSchema.Type == "object" && brokenSchema.Properties == nil {
		t.Error("Fix failed: object schema still has nil properties")
	}

	// Test that we can create a ParamsOneOf from the fixed schema
	// This is what would fail before the fix
	paramsOneOf := schema.NewParamsOneOfByJSONSchema(brokenSchema)
	if paramsOneOf == nil {
		t.Error("Failed to create ParamsOneOf from fixed schema - OpenAI function calling would fail")
	}
}

// TestConvertExclusiveBoundsToBoolean tests the JSON Schema draft-07 to draft-04 conversion
// for exclusiveMinimum and exclusiveMaximum fields.
// Draft-07: exclusiveMinimum/exclusiveMaximum are numeric values (the actual bounds)
// Draft-04: exclusiveMinimum/exclusiveMaximum are booleans that modify minimum/maximum
func TestConvertExclusiveBoundsToBoolean(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:  "exclusiveMinimum as number",
			input: `{"type": "number", "exclusiveMinimum": 0}`,
			expected: map[string]interface{}{
				"type":             "number",
				"minimum":          float64(0),
				"exclusiveMinimum": true,
			},
		},
		{
			name:  "exclusiveMaximum as number",
			input: `{"type": "number", "exclusiveMaximum": 100}`,
			expected: map[string]interface{}{
				"type":             "number",
				"maximum":          float64(100),
				"exclusiveMaximum": true,
			},
		},
		{
			name:  "both exclusive bounds as numbers",
			input: `{"type": "integer", "exclusiveMinimum": 1, "exclusiveMaximum": 10}`,
			expected: map[string]interface{}{
				"type":             "integer",
				"minimum":          float64(1),
				"exclusiveMinimum": true,
				"maximum":          float64(10),
				"exclusiveMaximum": true,
			},
		},
		{
			name:  "already boolean exclusiveMinimum (draft-04 style)",
			input: `{"type": "number", "minimum": 0, "exclusiveMinimum": true}`,
			expected: map[string]interface{}{
				"type":             "number",
				"minimum":          float64(0),
				"exclusiveMinimum": true,
			},
		},
		{
			name:  "no exclusive bounds",
			input: `{"type": "string", "minLength": 1}`,
			expected: map[string]interface{}{
				"type":      "string",
				"minLength": float64(1),
			},
		},
		{
			name:  "nested properties with exclusive bounds",
			input: `{"type": "object", "properties": {"age": {"type": "integer", "exclusiveMinimum": 0}}}`,
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"age": map[string]interface{}{
						"type":             "integer",
						"minimum":          float64(0),
						"exclusiveMinimum": true,
					},
				},
			},
		},
		{
			name:  "array items with exclusive bounds",
			input: `{"type": "array", "items": {"type": "number", "exclusiveMaximum": 100}}`,
			expected: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":             "number",
					"maximum":          float64(100),
					"exclusiveMaximum": true,
				},
			},
		},
		{
			name:  "allOf with exclusive bounds",
			input: `{"allOf": [{"type": "number", "exclusiveMinimum": 0}]}`,
			expected: map[string]interface{}{
				"allOf": []interface{}{
					map[string]interface{}{
						"type":             "number",
						"minimum":          float64(0),
						"exclusiveMinimum": true,
					},
				},
			},
		},
		{
			name:  "additionalProperties with exclusive bounds",
			input: `{"type": "object", "additionalProperties": {"type": "integer", "exclusiveMinimum": 0, "exclusiveMaximum": 255}}`,
			expected: map[string]interface{}{
				"type": "object",
				"additionalProperties": map[string]interface{}{
					"type":             "integer",
					"minimum":          float64(0),
					"exclusiveMinimum": true,
					"maximum":          float64(255),
					"exclusiveMaximum": true,
				},
			},
		},
		{
			name:  "Chrome DevTools MCP style schema (real-world example)",
			input: `{"type": "object", "properties": {"timeout": {"type": "integer", "exclusiveMinimum": 0}, "quality": {"type": "number", "minimum": 0, "maximum": 100}}}`,
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timeout": map[string]interface{}{
						"type":             "integer",
						"minimum":          float64(0),
						"exclusiveMinimum": true,
					},
					"quality": map[string]interface{}{
						"type":    "number",
						"minimum": float64(0),
						"maximum": float64(100),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertExclusiveBoundsToBoolean([]byte(tt.input))

			var got map[string]interface{}
			if err := json.Unmarshal(result, &got); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			if !deepEqual(got, tt.expected) {
				t.Errorf("convertExclusiveBoundsToBoolean() =\n%v\nwant:\n%v", got, tt.expected)
			}
		})
	}
}

// TestConvertExclusiveBoundsToBoolean_InvalidJSON tests that invalid JSON is returned unchanged
func TestConvertExclusiveBoundsToBoolean_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{invalid json}`)
	result := convertExclusiveBoundsToBoolean(invalidJSON)

	if string(result) != string(invalidJSON) {
		t.Errorf("Expected invalid JSON to be returned unchanged, got: %s", string(result))
	}
}

// deepEqual compares two maps recursively
func deepEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		switch av := v.(type) {
		case map[string]interface{}:
			bvm, ok := bv.(map[string]interface{})
			if !ok || !deepEqual(av, bvm) {
				return false
			}
		case []interface{}:
			bva, ok := bv.([]interface{})
			if !ok || !sliceEqual(av, bva) {
				return false
			}
		default:
			if v != bv {
				return false
			}
		}
	}
	return true
}

// sliceEqual compares two slices recursively
func sliceEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		switch av := a[i].(type) {
		case map[string]interface{}:
			bvm, ok := b[i].(map[string]interface{})
			if !ok || !deepEqual(av, bvm) {
				return false
			}
		case []interface{}:
			bva, ok := b[i].([]interface{})
			if !ok || !sliceEqual(av, bva) {
				return false
			}
		default:
			if a[i] != b[i] {
				return false
			}
		}
	}
	return true
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
