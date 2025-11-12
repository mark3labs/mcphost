package sdk

import (
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcphost/internal/session"
)

// Message is an alias for session.Message providing SDK users with access
// to message structures for conversation history and tool interactions.
type Message = session.Message

// ToolCall is an alias for session.ToolCall representing a tool invocation
// with its name, arguments, and result within a conversation.
type ToolCall = session.ToolCall

// ConvertToSchemaMessage converts an SDK message to the underlying schema message
// format used by the agent for LLM interactions.
func ConvertToSchemaMessage(msg *Message) *schema.Message {
	return msg.ConvertToSchemaMessage()
}

// ConvertFromSchemaMessage converts a schema message from the agent to an SDK
// message format for use in the SDK API.
func ConvertFromSchemaMessage(msg *schema.Message) Message {
	return session.ConvertFromSchemaMessage(msg)
}
