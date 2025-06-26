package agent

import (
	"context"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// StreamWithCallback streams content with real-time callbacks
func StreamWithCallback(ctx context.Context, reader *schema.StreamReader[*schema.Message], callback func(string)) (bool, string, error) {
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
