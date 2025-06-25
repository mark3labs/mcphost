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

// Session represents a complete conversation session with metadata
type Session struct {
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Metadata  Metadata  `json:"metadata"`
	Messages  []Message `json:"messages"`
}

// Metadata contains session metadata
type Metadata struct {
	MCPHostVersion string `json:"mcphost_version"`
	Provider       string `json:"provider"`
	Model          string `json:"model"`
}

// Message represents a single message in the session
type Message struct {
	ID         string     `json:"id"`
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Timestamp  time.Time  `json:"timestamp"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // For tool result messages
}

// ToolCall represents a tool call within a message
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments any    `json:"arguments"`
}



// NewSession creates a new session with default values
func NewSession() *Session {
	return &Session{
		Version:   "1.0",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []Message{},
		Metadata:  Metadata{},
	}
}

// AddMessage adds a message to the session
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

// SetMetadata sets the session metadata
func (s *Session) SetMetadata(metadata Metadata) {
	s.Metadata = metadata
	s.UpdatedAt = time.Now()
}

// SaveToFile saves the session to a JSON file
func (s *Session) SaveToFile(filePath string) error {
	s.UpdatedAt = time.Now()
	
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %v", err)
	}
	
	return os.WriteFile(filePath, data, 0644)
}

// LoadFromFile loads a session from a JSON file
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

// ConvertFromSchemaMessage converts a schema.Message to a session Message
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

// ConvertToSchemaMessage converts a session Message to a schema.Message
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