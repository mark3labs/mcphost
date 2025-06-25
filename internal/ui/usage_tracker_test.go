package ui

import (
	"testing"

	"github.com/mark3labs/mcphost/internal/models"
)

func TestUsageTracker_OAuthCosts(t *testing.T) {
	// Create a mock model info with costs
	modelInfo := &models.ModelInfo{
		ID:   "claude-3-5-sonnet-20241022",
		Name: "Claude 3.5 Sonnet v2",
		Cost: models.Cost{
			Input:  3.0,
			Output: 15.0,
		},
	}

	// Test with regular API key (costs should be calculated)
	regularTracker := NewUsageTracker(modelInfo, "anthropic", 80, false)
	regularTracker.UpdateUsage(1000, 500, 0, 0) // 1000 input, 500 output tokens

	stats := regularTracker.GetLastRequestStats()
	if stats == nil {
		t.Fatal("Expected stats to be non-nil")
	}

	// Check that costs are calculated for regular API key
	expectedInputCost := float64(1000) * 3.0 / 1000000  // $0.003
	expectedOutputCost := float64(500) * 15.0 / 1000000 // $0.0075
	expectedTotalCost := expectedInputCost + expectedOutputCost // $0.0105

	if stats.InputCost != expectedInputCost {
		t.Errorf("Expected input cost %f, got %f", expectedInputCost, stats.InputCost)
	}
	if stats.OutputCost != expectedOutputCost {
		t.Errorf("Expected output cost %f, got %f", expectedOutputCost, stats.OutputCost)
	}
	if stats.TotalCost != expectedTotalCost {
		t.Errorf("Expected total cost %f, got %f", expectedTotalCost, stats.TotalCost)
	}

	// Test with OAuth credentials (costs should be $0)
	oauthTracker := NewUsageTracker(modelInfo, "anthropic", 80, true)
	oauthTracker.UpdateUsage(1000, 500, 0, 0) // Same token usage

	oauthStats := oauthTracker.GetLastRequestStats()
	if oauthStats == nil {
		t.Fatal("Expected OAuth stats to be non-nil")
	}

	// Check that all costs are $0 for OAuth
	if oauthStats.InputCost != 0.0 {
		t.Errorf("Expected OAuth input cost to be $0, got %f", oauthStats.InputCost)
	}
	if oauthStats.OutputCost != 0.0 {
		t.Errorf("Expected OAuth output cost to be $0, got %f", oauthStats.OutputCost)
	}
	if oauthStats.TotalCost != 0.0 {
		t.Errorf("Expected OAuth total cost to be $0, got %f", oauthStats.TotalCost)
	}

	// Verify token counts are still tracked correctly for OAuth
	if oauthStats.InputTokens != 1000 {
		t.Errorf("Expected OAuth input tokens to be 1000, got %d", oauthStats.InputTokens)
	}
	if oauthStats.OutputTokens != 500 {
		t.Errorf("Expected OAuth output tokens to be 500, got %d", oauthStats.OutputTokens)
	}
}

func TestUsageTracker_OAuthSessionStats(t *testing.T) {
	// Create a mock model info with costs
	modelInfo := &models.ModelInfo{
		ID:   "claude-3-5-sonnet-20241022",
		Name: "Claude 3.5 Sonnet v2",
		Cost: models.Cost{
			Input:  3.0,
			Output: 15.0,
		},
	}

	// Test OAuth session stats accumulation
	oauthTracker := NewUsageTracker(modelInfo, "anthropic", 80, true)
	
	// Make multiple requests
	oauthTracker.UpdateUsage(1000, 500, 0, 0)
	oauthTracker.UpdateUsage(2000, 1000, 0, 0)

	sessionStats := oauthTracker.GetSessionStats()

	// Check that tokens are accumulated correctly
	if sessionStats.TotalInputTokens != 3000 {
		t.Errorf("Expected total input tokens to be 3000, got %d", sessionStats.TotalInputTokens)
	}
	if sessionStats.TotalOutputTokens != 1500 {
		t.Errorf("Expected total output tokens to be 1500, got %d", sessionStats.TotalOutputTokens)
	}

	// Check that total cost remains $0 for OAuth
	if sessionStats.TotalCost != 0.0 {
		t.Errorf("Expected OAuth session total cost to be $0, got %f", sessionStats.TotalCost)
	}

	// Check request count
	if sessionStats.RequestCount != 2 {
		t.Errorf("Expected request count to be 2, got %d", sessionStats.RequestCount)
	}
}