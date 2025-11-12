package session

import (
	"fmt"
	"sync"

	"github.com/cloudwego/eino/schema"
)

// Manager manages session state and auto-saving functionality.
// It provides thread-safe operations for managing a conversation session,
// including automatic persistence to disk after each modification.
// The Manager ensures that all session operations are synchronized and
// that the session file is kept up-to-date with any changes.
type Manager struct {
	session  *Session
	filePath string
	mutex    sync.RWMutex
}

// NewManager creates a new session manager with a fresh session.
// The filePath parameter specifies where the session will be auto-saved.
// If filePath is empty, the session will not be automatically saved to disk.
// Returns a Manager instance ready to track conversation messages.
func NewManager(filePath string) *Manager {
	return &Manager{
		session:  NewSession(),
		filePath: filePath,
	}
}

// NewManagerWithSession creates a new session manager with an existing session.
// This is useful when loading a session from a file and wanting to continue
// managing it with auto-save functionality.
// The session parameter is the existing session to manage.
// The filePath parameter specifies where the session will be auto-saved.
func NewManagerWithSession(session *Session, filePath string) *Manager {
	return &Manager{
		session:  session,
		filePath: filePath,
	}
}

// AddMessage adds a message to the session and auto-saves.
// The message is converted from schema.Message format to the internal
// session Message format before being added. If a filePath was specified
// when creating the Manager, the session is automatically saved to disk.
// This operation is thread-safe.
// Returns an error if auto-saving fails, nil otherwise.
func (m *Manager) AddMessage(msg *schema.Message) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	sessionMsg := ConvertFromSchemaMessage(msg)
	m.session.AddMessage(sessionMsg)

	if m.filePath != "" {
		return m.session.SaveToFile(m.filePath)
	}

	return nil
}

// AddMessages adds multiple messages to the session and auto-saves.
// All messages are added in order and then the session is saved once.
// This is more efficient than calling AddMessage multiple times when
// adding several messages at once. The operation is thread-safe.
// Returns an error if auto-saving fails, nil otherwise.
func (m *Manager) AddMessages(msgs []*schema.Message) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, msg := range msgs {
		sessionMsg := ConvertFromSchemaMessage(msg)
		m.session.AddMessage(sessionMsg)
	}

	if m.filePath != "" {
		return m.session.SaveToFile(m.filePath)
	}

	return nil
}

// ReplaceAllMessages replaces all messages in the session with the provided messages.
// This method completely clears the existing message history and replaces it with
// the new set of messages. Useful for resetting a conversation or loading a
// different conversation context. The operation is thread-safe and triggers
// an auto-save if a filePath is configured.
// Returns an error if auto-saving fails, nil otherwise.
func (m *Manager) ReplaceAllMessages(msgs []*schema.Message) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Clear existing messages
	m.session.Messages = []Message{}

	// Add all new messages
	for _, msg := range msgs {
		sessionMsg := ConvertFromSchemaMessage(msg)
		m.session.AddMessage(sessionMsg)
	}

	if m.filePath != "" {
		return m.session.SaveToFile(m.filePath)
	}

	return nil
}

// SetMetadata sets the session metadata.
// This updates the session's metadata with information about the provider,
// model, and MCPHost version. The operation is thread-safe and triggers
// an auto-save if a filePath is configured.
// Returns an error if auto-saving fails, nil otherwise.
func (m *Manager) SetMetadata(metadata Metadata) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.session.SetMetadata(metadata)

	if m.filePath != "" {
		return m.session.SaveToFile(m.filePath)
	}

	return nil
}

// GetMessages returns all messages as a schema.Message slice.
// This method converts all stored session messages to the schema format
// used by LLM providers. The returned slice is a new allocation, so
// modifications to it won't affect the stored session. This operation
// is thread-safe for concurrent reads.
func (m *Manager) GetMessages() []*schema.Message {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	messages := make([]*schema.Message, len(m.session.Messages))
	for i, msg := range m.session.Messages {
		messages[i] = msg.ConvertToSchemaMessage()
	}

	return messages
}

// GetSession returns a copy of the current session.
// The returned session is a deep copy, including all messages, so
// modifications to it won't affect the managed session. This is useful
// for safely inspecting the session state without risk of concurrent
// modification. This operation is thread-safe for concurrent reads.
func (m *Manager) GetSession() *Session {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return a copy to prevent external modification
	sessionCopy := *m.session
	sessionCopy.Messages = make([]Message, len(m.session.Messages))
	copy(sessionCopy.Messages, m.session.Messages)

	return &sessionCopy
}

// Save manually saves the session to file.
// This forces a save operation even if no changes have been made.
// Useful for ensuring the session is persisted at specific points.
// Returns an error if no filePath was specified when creating the
// Manager, or if the save operation fails.
func (m *Manager) Save() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.filePath == "" {
		return fmt.Errorf("no file path specified for session manager")
	}

	return m.session.SaveToFile(m.filePath)
}

// GetFilePath returns the file path for this session.
// Returns the path where the session is being auto-saved, or an
// empty string if no auto-save path was configured.
func (m *Manager) GetFilePath() string {
	return m.filePath
}

// MessageCount returns the number of messages in the session.
// This provides a quick way to check the conversation length without
// retrieving all messages. This operation is thread-safe for concurrent reads.
func (m *Manager) MessageCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.session.Messages)
}
