package agent

import (
	"context"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// StreamToolCallChecker determines if streaming output contains tool calls
// Returns: hasToolCalls, content, error
type StreamToolCallChecker func(ctx context.Context, reader *schema.StreamReader[*schema.Message]) (bool, string, error)

// StreamingCallback is called for each chunk of streaming content
type StreamingCallback func(chunk string)

// StreamingConfig holds configuration for streaming behavior
type StreamingConfig struct {
	ToolCallChecker StreamToolCallChecker
	BufferSize      int
	UpdateInterval  int // milliseconds
}

// getProviderType determines the provider type from the stored provider type
func (a *Agent) getProviderType() string {
	return a.providerType
}

// getProviderToolCallChecker returns the appropriate tool call checker for the provider
func (a *Agent) getProviderToolCallChecker() StreamToolCallChecker {
	switch a.getProviderType() {
	case "anthropic":
		return anthropicStreamToolCallChecker
	case "openai", "azure":
		return openaiStreamToolCallChecker
	case "google", "gemini":
		return geminiStreamToolCallChecker
	case "ollama":
		return ollamaStreamToolCallChecker
	default:
		return defaultStreamToolCallChecker
	}
}

// anthropicStreamToolCallChecker handles Anthropic Claude's streaming pattern
// Claude typically outputs text content first, then tool calls later
func anthropicStreamToolCallChecker(ctx context.Context, reader *schema.StreamReader[*schema.Message]) (bool, string, error) {
	defer reader.Close()

	var fullContent strings.Builder
	var toolCallDetected bool

	for {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		default:
		}

		msg, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, "", err
		}

		fullContent.WriteString(msg.Content)

		// Claude typically outputs tool calls after text content
		if len(msg.ToolCalls) > 0 {
			toolCallDetected = true
			break
		}

		// Check for Claude-specific tool call patterns in accumulated content
		content := fullContent.String()
		if strings.Contains(content, "<function_calls>") ||
			strings.Contains(content, "I'll use the") ||
			strings.Contains(content, "Let me use") ||
			strings.Contains(content, "I need to use") {
			toolCallDetected = true
			break
		}
	}

	return toolCallDetected, fullContent.String(), nil
}

// openaiStreamToolCallChecker handles OpenAI's streaming pattern
// OpenAI outputs tool calls in early chunks, but we need to consume the full stream
func openaiStreamToolCallChecker(ctx context.Context, reader *schema.StreamReader[*schema.Message]) (bool, string, error) {
	defer reader.Close()

	var content strings.Builder
	var hasToolCalls bool

	for {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		default:
		}

		msg, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, "", err
		}

		content.WriteString(msg.Content)

		// OpenAI outputs tool calls in early chunks
		if len(msg.ToolCalls) > 0 {
			hasToolCalls = true
			break
		}
	}

	return hasToolCalls, content.String(), nil
}

// geminiStreamToolCallChecker handles Google Gemini's streaming pattern
// Similar to OpenAI - tool calls appear early, but we need to consume the full stream
func geminiStreamToolCallChecker(ctx context.Context, reader *schema.StreamReader[*schema.Message]) (bool, string, error) {
	defer reader.Close()

	var content strings.Builder
	var hasToolCalls bool

	for {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		default:
		}

		msg, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, "", err
		}

		content.WriteString(msg.Content)

		// Gemini outputs tool calls in early chunks
		if len(msg.ToolCalls) > 0 {
			hasToolCalls = true
			break
		}

		// Check for Gemini-specific patterns
		contentStr := content.String()
		if strings.Contains(contentStr, "function_call") {
			hasToolCalls = true
			break
		}
	}

	return hasToolCalls, content.String(), nil
}

// ollamaStreamToolCallChecker handles Ollama's streaming pattern
// Conservative approach - varies by model
func ollamaStreamToolCallChecker(ctx context.Context, reader *schema.StreamReader[*schema.Message]) (bool, string, error) {
	defer reader.Close()

	var content strings.Builder
	chunkCount := 0

	for {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		default:
		}

		msg, err := reader.Recv()
		if err == io.EOF {
			return false, content.String(), nil
		}
		if err != nil {
			return false, "", err
		}

		content.WriteString(msg.Content)
		chunkCount++

		// Check for tool calls
		if len(msg.ToolCalls) > 0 {
			return true, content.String(), nil
		}

		// For Ollama, be more conservative - collect more content before deciding
		// This is because different models have different patterns
		if chunkCount > 5 && len(content.String()) > 50 {
			return false, content.String(), nil
		}
	}
}

// defaultStreamToolCallChecker provides a conservative default implementation
func defaultStreamToolCallChecker(ctx context.Context, reader *schema.StreamReader[*schema.Message]) (bool, string, error) {
	defer reader.Close()

	var content strings.Builder
	var hasToolCalls bool

	for {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		default:
		}

		msg, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, "", err
		}

		content.WriteString(msg.Content)

		// Check for tool calls
		if len(msg.ToolCalls) > 0 {
			hasToolCalls = true
			break
		}
	}

	return hasToolCalls, content.String(), nil
}

// StreamWithCallback streams content with real-time callbacks
func StreamWithCallback(ctx context.Context, reader *schema.StreamReader[*schema.Message], callback StreamingCallback) (bool, string, error) {
	defer reader.Close()

	var content strings.Builder
	var hasToolCalls bool

	for {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		default:
		}

		msg, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, "", err
		}

		// Call callback for each chunk if provided
		if callback != nil && msg.Content != "" {
			callback(msg.Content)
		}

		content.WriteString(msg.Content)

		// Check for tool calls
		if len(msg.ToolCalls) > 0 {
			hasToolCalls = true
			break
		}
	}

	return hasToolCalls, content.String(), nil
}
