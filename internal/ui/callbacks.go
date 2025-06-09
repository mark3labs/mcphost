package ui

import (
	"context"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	utilCallbacks "github.com/cloudwego/eino/utils/callbacks"
)

// CreateCallbackHandler creates a callback handler using HandlerHelper
func (c *CLI) CreateCallbackHandler() callbacks.Handler {
	toolHandler := &utilCallbacks.ToolCallbackHandler{
		OnStart: func(ctx context.Context, runInfo *callbacks.RunInfo, input *tool.CallbackInput) context.Context {
			// Display the tool call message with the tool name and arguments
			c.DisplayToolCallMessage(runInfo.Name, input.ArgumentsInJSON)
			return ctx
		},
		OnEnd: func(ctx context.Context, runInfo *callbacks.RunInfo, output *tool.CallbackOutput) context.Context {
			// Tool execution completed - we could show results here if needed
			return ctx
		},
		OnEndWithStreamOutput: func(ctx context.Context, runInfo *callbacks.RunInfo, output *schema.StreamReader[*tool.CallbackOutput]) context.Context {
			return ctx
		},
		OnError: func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context {
			// Display error message
			c.DisplayError(err)
			return ctx
		},
	}
	
	return utilCallbacks.NewHandlerHelper().
		Tool(toolHandler).
		Handler()
}