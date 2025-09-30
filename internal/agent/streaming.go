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
	// Close reader when done
	// StreamReader.Close() doesn't return an error, but underlying operations might panic
	// We wrap in recover to handle any panic from already-closed streams gracefully
	defer func() {
		if r := recover(); r != nil {
			// Suppress panic from closing already-closed stream
			// This can happen when providers close the stream themselves after EOF
		}
		reader.Close()
	}()

	var content strings.Builder
	var accumulatedToolCalls map[string]*schema.ToolCall // Track tool calls by ID to handle incremental updates
	var streamComplete bool
	var finalResponseMeta *schema.ResponseMeta // Accumulate response metadata from all chunks

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

		// Accumulate response metadata - merge from multiple chunks for accuracy
		if msg.ResponseMeta != nil {
			if finalResponseMeta == nil {
				// First metadata we've seen - use as base
				finalResponseMeta = &schema.ResponseMeta{}
				if msg.ResponseMeta.Usage != nil {
					finalResponseMeta.Usage = &schema.TokenUsage{}
				}
			}

			// Merge metadata intelligently to handle Anthropic's streaming behavior
			if msg.ResponseMeta.Usage != nil && finalResponseMeta.Usage != nil {
				usage := msg.ResponseMeta.Usage

				// Take PromptTokens from first chunk that has them (usually non-zero)
				if finalResponseMeta.Usage.PromptTokens == 0 && usage.PromptTokens > 0 {
					finalResponseMeta.Usage.PromptTokens = usage.PromptTokens
				}

				// Always take the latest CompletionTokens (accumulates over chunks)
				if usage.CompletionTokens > 0 {
					finalResponseMeta.Usage.CompletionTokens = usage.CompletionTokens
				}

				// Calculate TotalTokens from the components
				finalResponseMeta.Usage.TotalTokens = finalResponseMeta.Usage.PromptTokens + finalResponseMeta.Usage.CompletionTokens
			}

			// Preserve other metadata fields from the latest chunk
			if msg.ResponseMeta.FinishReason != "" {
				finalResponseMeta.FinishReason = msg.ResponseMeta.FinishReason
			}
		}

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

	// Return complete message with all content, final tool calls, and preserved metadata
	return &schema.Message{
		Role:         schema.Assistant,
		Content:      content.String(),
		ToolCalls:    finalToolCalls,
		ResponseMeta: finalResponseMeta, // Preserve usage and other metadata from streaming
	}, nil
}
