package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cloudwego/eino/schema"
)

// Session represents a complete conversation session with metadata.
// It stores all messages exchanged during a conversation along with
// contextual information about the session such as the provider, model,
// and timestamps. Sessions can be saved to and loaded from JSON files
// for persistence across program runs.
type Session struct {
	// Version indicates the session format version for compatibility
	Version string `json:"version"`
	// CreatedAt is the timestamp when the session was first created
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the session was last modified
	UpdatedAt time.Time `json:"updated_at"`
	// Metadata contains contextual information about the session
	Metadata Metadata `json:"metadata"`
	// Messages is the ordered list of all messages in this session
	Messages []Message `json:"messages"`
}

// Metadata contains session metadata that provides context about the
// environment and configuration used during the conversation. This helps
// with debugging and understanding the session's context when reviewing
// conversation history.
type Metadata struct {
	// MCPHostVersion is the version of MCPHost used for this session
	MCPHostVersion string `json:"mcphost_version"`
	// Provider is the LLM provider used (e.g., "anthropic", "openai", "gemini")
	Provider string `json:"provider"`
	// Model is the specific model identifier used for the conversation
	Model string `json:"model"`
}

// Message represents a single message in the conversation session.
// Messages can be from different roles (user, assistant, tool) and may
// include tool calls for assistant messages or tool results for tool messages.
type Message struct {
	// ID is a unique identifier for this message, auto-generated if not provided
	ID string `json:"id"`
	// Role indicates who sent the message ("user", "assistant", "tool", or "system")
	Role string `json:"role"`
	// Content is the text content of the message
	Content string `json:"content"`
	// Timestamp is when the message was created
	Timestamp time.Time `json:"timestamp"`
	// ToolCalls contains any tool invocations made by the assistant in this message
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID links a tool result message to its corresponding tool call
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool invocation within an assistant message.
// When the assistant decides to use a tool, it creates a ToolCall with
// the necessary information to execute that tool.
type ToolCall struct {
	// ID is a unique identifier for this tool call, used to link results
	ID string `json:"id"`
	// Name is the name of the tool being invoked
	Name string `json:"name"`
	// Arguments contains the parameters passed to the tool, typically as JSON
	Arguments any `json:"arguments"`
}

// NewSession creates a new session with default values.
// It initializes a session with version 1.0, current timestamps,
// empty message list, and empty metadata. The returned session
// is ready to receive messages and can be saved to a file.
func NewSession() *Session {
	return &Session{
		Version:   "1.0",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []Message{},
		Metadata:  Metadata{},
	}
}

// AddMessage adds a message to the session.
// If the message doesn't have an ID, one will be auto-generated.
// If the message doesn't have a timestamp, the current time will be used.
// The session's UpdatedAt timestamp is automatically updated.
func (s *Session) AddMessage(msg Message) {
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// SetMetadata sets the session metadata.
// This replaces the existing metadata with the provided metadata
// and updates the session's UpdatedAt timestamp. Use this to record
// information about the provider, model, and MCPHost version.
func (s *Session) SetMetadata(metadata Metadata) {
	s.Metadata = metadata
	s.UpdatedAt = time.Now()
}

// SaveToFile saves the session to a JSON file.
// The session is serialized as indented JSON for readability.
// The UpdatedAt timestamp is automatically updated before saving.
// The file is created with 0644 permissions if it doesn't exist,
// or overwritten if it does exist.
// Returns an error if marshaling fails or file writing fails.
func (s *Session) SaveToFile(filePath string) error {
	s.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %v", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadFromFile loads a session from a JSON file.
// It reads the file at the specified path and deserializes it into
// a Session struct. This is useful for resuming previous conversations
// or reviewing session history.
// Returns the loaded session on success, or an error if the file
// cannot be read or the JSON is invalid.
func LoadFromFile(filePath string) (*Session, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %v", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %v", err)
	}

	return &session, nil
}

// ConvertFromSchemaMessage converts a schema.Message to a session Message.
// This function bridges between the eino schema message format and the
// session's internal message format. It preserves role, content, and
// tool-related information while adding a timestamp.
// Tool calls from assistant messages and tool call IDs from tool messages
// are properly converted and preserved.
func ConvertFromSchemaMessage(msg *schema.Message) Message {
	sessionMsg := Message{
		Role:      string(msg.Role),
		Content:   msg.Content,
		Timestamp: time.Now(),
	}

	// Convert tool calls if present (for assistant messages)
	if len(msg.ToolCalls) > 0 {
		sessionMsg.ToolCalls = make([]ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			sessionMsg.ToolCalls[i] = ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}

	// Handle tool result messages - extract tool call ID from ToolCallID field
	if msg.Role == schema.Tool && msg.ToolCallID != "" {
		sessionMsg.ToolCallID = msg.ToolCallID
	}

	return sessionMsg
}

// ConvertToSchemaMessage converts a session Message to a schema.Message.
// This method bridges between the session's internal message format and
// the eino schema message format used by the LLM providers.
// It properly handles tool calls for assistant messages and tool call IDs
// for tool result messages. Arguments are converted to string format as
// required by the schema.
func (m *Message) ConvertToSchemaMessage() *schema.Message {
	msg := &schema.Message{
		Role:    schema.RoleType(m.Role),
		Content: m.Content,
	}

	// Convert tool calls if present (for assistant messages)
	if len(m.ToolCalls) > 0 {
		msg.ToolCalls = make([]schema.ToolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			// Arguments are already stored as a string, use them directly
			var argsStr string
			if str, ok := tc.Arguments.(string); ok {
				argsStr = str
			} else {
				// Fallback: marshal to JSON if not a string
				if argBytes, err := json.Marshal(tc.Arguments); err == nil {
					argsStr = string(argBytes)
				}
			}

			msg.ToolCalls[i] = schema.ToolCall{
				ID: tc.ID,
				Function: schema.FunctionCall{
					Name:      tc.Name,
					Arguments: argsStr,
				},
			}
		}
	}

	// Handle tool result messages - set the tool call ID
	if m.Role == "tool" && m.ToolCallID != "" {
		msg.ToolCallID = m.ToolCallID
	}

	return msg
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "msg_" + hex.EncodeToString(bytes)
}
