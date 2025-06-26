package agent

import (
	"context"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// StreamWithCallback streams content with real-time callbacks and returns complete response
// IMPORTANT: Tool calls are only processed after EOF is reached to ensure we have the complete
// and final tool call information. This prevents premature tool execution on partial data.
// Handles different provider streaming patterns:
// - Anthropic: Text content first, then tool calls streamed incrementally
// - OpenAI/Others: Tool calls first or alone
// - Mixed: Tool calls and content interleaved
func StreamWithCallback(ctx context.Context, reader *schema.StreamReader[*schema.Message], callback func(string)) (*schema.Message, error) {
	defer reader.Close()

	var content strings.Builder
	var accumulatedToolCalls map[string]*schema.ToolCall // Track tool calls by ID to handle incremental updates
	var streamComplete bool

	accumulatedToolCalls = make(map[string]*schema.ToolCall)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		msg, err := reader.Recv()
		if err == io.EOF {
			// Stream is complete - now we can safely process tool calls
			streamComplete = true
			break
		}
		if err != nil {
			return nil, err
		}

		// Call callback for each chunk if provided (for real-time display)
		if callback != nil && msg.Content != "" {
			callback(msg.Content)
		}

		// Accumulate content from all chunks
		content.WriteString(msg.Content)

		// Accumulate tool calls incrementally - Anthropic streams them piece by piece
		// NOTE: We don't process these tool calls until EOF is reached
		if len(msg.ToolCalls) > 0 {
			for _, toolCall := range msg.ToolCalls {
				// Use tool call ID as key, but handle cases where ID might be empty in partial chunks
				key := toolCall.ID
				if key == "" {
					// For chunks without ID, try to find existing tool call or create a temporary key
					if len(accumulatedToolCalls) == 1 {
						// If we have exactly one tool call being built, assume this chunk belongs to it
						for existingKey := range accumulatedToolCalls {
							key = existingKey
							break
						}
					} else {
						// Create a temporary key for this tool call
						key = "temp_" + toolCall.Function.Name
					}
				}

				existing := accumulatedToolCalls[key]
				if existing == nil {
					// First time seeing this tool call
					accumulatedToolCalls[key] = &schema.ToolCall{
						ID:       toolCall.ID,
						Function: toolCall.Function,
					}
				} else {
					// Update existing tool call with new information
					// Preserve non-empty values, accumulate arguments
					if toolCall.ID != "" {
						existing.ID = toolCall.ID
					}
					if toolCall.Function.Name != "" {
						existing.Function.Name = toolCall.Function.Name
					}
					// Accumulate arguments (they come in pieces)
					existing.Function.Arguments += toolCall.Function.Arguments
				}
			}
		}
	}

	// Only process tool calls after EOF - ensures we have complete information
	var finalToolCalls []schema.ToolCall
	if streamComplete && len(accumulatedToolCalls) > 0 {
		finalToolCalls = make([]schema.ToolCall, 0, len(accumulatedToolCalls))
		for _, toolCall := range accumulatedToolCalls {
			finalToolCalls = append(finalToolCalls, *toolCall)
		}
	}

	// Return complete message with all content and final tool calls
	return &schema.Message{
		Role:      schema.Assistant,
		Content:   content.String(),
		ToolCalls: finalToolCalls,
	}, nil
}