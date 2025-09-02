package sdk_test

import (
	"context"
	"testing"

	"github.com/mark3labs/mcphost/sdk"
)

func TestNew(t *testing.T) {
	ctx := context.Background()

	// Test default initialization
	host, err := sdk.New(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to create MCPHost with defaults: %v", err)
	}
	defer host.Close()

	if host.GetModelString() == "" {
		t.Error("Model string should not be empty")
	}
}

func TestNewWithOptions(t *testing.T) {
	ctx := context.Background()

	opts := &sdk.Options{
		Model:    "anthropic:claude-3-haiku-20240307",
		MaxSteps: 5,
		Quiet:    true,
	}

	host, err := sdk.New(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to create MCPHost with options: %v", err)
	}
	defer host.Close()

	if host.GetModelString() != opts.Model {
		t.Errorf("Expected model %s, got %s", opts.Model, host.GetModelString())
	}
}

func TestSessionManagement(t *testing.T) {
	ctx := context.Background()

	host, err := sdk.New(ctx, &sdk.Options{Quiet: true})
	if err != nil {
		t.Fatalf("Failed to create MCPHost: %v", err)
	}
	defer host.Close()

	// Test clear session
	host.ClearSession()
	mgr := host.GetSessionManager()
	if mgr.MessageCount() != 0 {
		t.Error("Session should be empty after clear")
	}

	// Test save/load session (would need actual implementation)
	tempFile := t.TempDir() + "/session.json"

	// Add a message first
	_, err = host.Prompt(ctx, "test message")
	if err == nil { // Only if we have a working model
		if err := host.SaveSession(tempFile); err != nil {
			t.Errorf("Failed to save session: %v", err)
		}

		// Clear and reload
		host.ClearSession()
		if err := host.LoadSession(tempFile); err != nil {
			t.Errorf("Failed to load session: %v", err)
		}
	}
}
