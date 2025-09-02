package sdk

import (
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcphost/internal/session"
)

// Message is an alias for session.Message for SDK users
type Message = session.Message

// ToolCall is an alias for session.ToolCall
type ToolCall = session.ToolCall

// ConvertToSchemaMessage converts SDK message to schema message
func ConvertToSchemaMessage(msg *Message) *schema.Message {
	return msg.ConvertToSchemaMessage()
}

// ConvertFromSchemaMessage converts schema message to SDK message
func ConvertFromSchemaMessage(msg *schema.Message) Message {
	return session.ConvertFromSchemaMessage(msg)
}
