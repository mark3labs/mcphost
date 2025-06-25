package ui

import (
	"fmt"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/tokens"
)

// UsageStats represents token and cost information for a single request/response
type UsageStats struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	InputCost        float64
	OutputCost       float64
	CacheReadCost    float64
	CacheWriteCost   float64
	TotalCost        float64
}

// SessionStats represents cumulative stats for the entire session
type SessionStats struct {
	TotalInputTokens      int
	TotalOutputTokens     int
	TotalCacheReadTokens  int
	TotalCacheWriteTokens int
	TotalCost             float64
	RequestCount          int
}

// UsageTracker tracks token usage and costs for LLM interactions
type UsageTracker struct {
	mu           sync.RWMutex
	modelInfo    *models.ModelInfo
	provider     string
	sessionStats SessionStats
	lastRequest  *UsageStats
	width        int
	isOAuth      bool // Whether OAuth credentials are being used (costs should be $0)
}

// NewUsageTracker creates a new usage tracker for the given model
func NewUsageTracker(modelInfo *models.ModelInfo, provider string, width int, isOAuth bool) *UsageTracker {
	return &UsageTracker{
		modelInfo: modelInfo,
		provider:  provider,
		width:     width,
		isOAuth:   isOAuth,
	}
}

// EstimateTokens provides a rough estimate of tokens in text
// This is a simple approximation - real token counting would require the actual tokenizer
func EstimateTokens(text string) int {
	// Rough approximation: ~4 characters per token for most models
	// This is not accurate but gives a reasonable estimate
	return len(text) / 4
}

// UpdateUsage updates the tracker with new usage information
func (ut *UsageTracker) UpdateUsage(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	// Calculate costs based on model pricing
	// For OAuth credentials, costs are $0 for usage tracking purposes
	var inputCost, outputCost, cacheReadCost, cacheWriteCost, totalCost float64

	if !ut.isOAuth {
		inputCost = float64(inputTokens) * ut.modelInfo.Cost.Input / 1000000 // Cost is per million tokens
		outputCost = float64(outputTokens) * ut.modelInfo.Cost.Output / 1000000

		if ut.modelInfo.Cost.CacheRead != nil {
			cacheReadCost = float64(cacheReadTokens) * (*ut.modelInfo.Cost.CacheRead) / 1000000
		}
		if ut.modelInfo.Cost.CacheWrite != nil {
			cacheWriteCost = float64(cacheWriteTokens) * (*ut.modelInfo.Cost.CacheWrite) / 1000000
		}

		totalCost = inputCost + outputCost + cacheReadCost + cacheWriteCost
	}
	// If OAuth, all costs remain 0.0

	// Update last request stats
	ut.lastRequest = &UsageStats{
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		CacheReadTokens:  cacheReadTokens,
		CacheWriteTokens: cacheWriteTokens,
		InputCost:        inputCost,
		OutputCost:       outputCost,
		CacheReadCost:    cacheReadCost,
		CacheWriteCost:   cacheWriteCost,
		TotalCost:        totalCost,
	}

	// Update session stats
	ut.sessionStats.TotalInputTokens += inputTokens
	ut.sessionStats.TotalOutputTokens += outputTokens
	ut.sessionStats.TotalCacheReadTokens += cacheReadTokens
	ut.sessionStats.TotalCacheWriteTokens += cacheWriteTokens
	ut.sessionStats.TotalCost += totalCost
	ut.sessionStats.RequestCount++
}

// EstimateAndUpdateUsage estimates tokens from text and updates usage
func (ut *UsageTracker) EstimateAndUpdateUsage(inputText, outputText string) {
	inputTokens := tokens.EstimateTokens(inputText)
	outputTokens := tokens.EstimateTokens(outputText)
	ut.UpdateUsage(inputTokens, outputTokens, 0, 0)
}

// EstimateAndUpdateUsageFromText estimates tokens from text and updates usage
func (ut *UsageTracker) EstimateAndUpdateUsageFromText(inputText, outputText string) {
	inputTokens := tokens.EstimateTokens(inputText)
	outputTokens := tokens.EstimateTokens(outputText)
	ut.UpdateUsage(inputTokens, outputTokens, 0, 0)
}

// RenderUsageInfo renders enhanced usage information with better styling
func (ut *UsageTracker) RenderUsageInfo() string {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	if ut.sessionStats.RequestCount == 0 {
		return ""
	}

	// Import lipgloss for styling
	baseStyle := lipgloss.NewStyle()

	// Calculate total tokens
	totalTokens := ut.sessionStats.TotalInputTokens + ut.sessionStats.TotalOutputTokens

	// Format tokens with K/M suffix for better readability
	var tokenStr string
	if totalTokens >= 1000000 {
		tokenStr = fmt.Sprintf("%.1fM", float64(totalTokens)/1000000)
	} else if totalTokens >= 1000 {
		tokenStr = fmt.Sprintf("%.1fK", float64(totalTokens)/1000)
	} else {
		tokenStr = fmt.Sprintf("%d", totalTokens)
	}

	// Calculate percentage based on context limit with color coding
	var percentageStr string
	var percentageColor lipgloss.AdaptiveColor
	if ut.modelInfo.Limit.Context > 0 {
		percentage := float64(totalTokens) / float64(ut.modelInfo.Limit.Context) * 100

		// Color code based on usage percentage
		theme := GetTheme()
		if percentage >= 80 {
			percentageColor = theme.Error // Red
		} else if percentage >= 60 {
			percentageColor = theme.Warning // Orange
		} else {
			percentageColor = theme.Success // Green
		}

		percentageStr = baseStyle.
			Foreground(percentageColor).
			Render(fmt.Sprintf(" (%.0f%%)", percentage))
	}

	// Format cost with appropriate styling
	theme := GetTheme()
	var costStr string
	if ut.isOAuth {
		costStr = baseStyle.
			Foreground(theme.Primary).
			Render("$0.00")
	} else {
		costStr = baseStyle.
			Foreground(theme.Primary).
			Render(fmt.Sprintf("$%.4f", ut.sessionStats.TotalCost))
	}

	// Create styled components
	tokensLabel := baseStyle.
		Foreground(theme.Muted).
		Render("Tokens: ")

	tokensValue := baseStyle.
		Foreground(theme.Text).
		Bold(true).
		Render(tokenStr)

	costLabel := baseStyle.
		Foreground(theme.Muted).
		Render(" | Cost: ")

	// Build the enhanced display
	return fmt.Sprintf("%s%s%s%s%s\n",
		tokensLabel, tokensValue, percentageStr, costLabel, costStr)
}

// GetSessionStats returns a copy of the current session statistics
func (ut *UsageTracker) GetSessionStats() SessionStats {
	ut.mu.RLock()
	defer ut.mu.RUnlock()
	return ut.sessionStats
}

// GetLastRequestStats returns a copy of the last request statistics
func (ut *UsageTracker) GetLastRequestStats() *UsageStats {
	ut.mu.RLock()
	defer ut.mu.RUnlock()
	if ut.lastRequest == nil {
		return nil
	}
	stats := *ut.lastRequest
	return &stats
}

// Reset clears all usage statistics
func (ut *UsageTracker) Reset() {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.sessionStats = SessionStats{}
	ut.lastRequest = nil
}

// SetWidth updates the display width for rendering
func (ut *UsageTracker) SetWidth(width int) {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.width = width
}
