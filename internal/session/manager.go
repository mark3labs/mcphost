package session

import (
	"fmt"
	"sync"

	"github.com/cloudwego/eino/schema"
)

// Manager manages session state and auto-saving
type Manager struct {
	session  *Session
	filePath string
	mutex    sync.RWMutex
}

// NewManager creates a new session manager
func NewManager(filePath string) *Manager {
	return &Manager{
		session:  NewSession(),
		filePath: filePath,
	}
}

// NewManagerWithSession creates a new session manager with an existing session
func NewManagerWithSession(session *Session, filePath string) *Manager {
	return &Manager{
		session:  session,
		filePath: filePath,
	}
}

// AddMessage adds a message to the session and auto-saves
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

// AddMessages adds multiple messages to the session and auto-saves
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

// ReplaceAllMessages replaces all messages in the session with the provided messages
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



// SetMetadata sets the session metadata
func (m *Manager) SetMetadata(metadata Metadata) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.session.SetMetadata(metadata)
	
	if m.filePath != "" {
		return m.session.SaveToFile(m.filePath)
	}
	
	return nil
}

// GetMessages returns all messages as schema.Message slice
func (m *Manager) GetMessages() []*schema.Message {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	messages := make([]*schema.Message, len(m.session.Messages))
	for i, msg := range m.session.Messages {
		messages[i] = msg.ConvertToSchemaMessage()
	}
	
	return messages
}

// GetSession returns a copy of the current session
func (m *Manager) GetSession() *Session {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	// Return a copy to prevent external modification
	sessionCopy := *m.session
	sessionCopy.Messages = make([]Message, len(m.session.Messages))
	copy(sessionCopy.Messages, m.session.Messages)
	
	return &sessionCopy
}

// Save manually saves the session to file
func (m *Manager) Save() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if m.filePath == "" {
		return fmt.Errorf("no file path specified for session manager")
	}
	
	return m.session.SaveToFile(m.filePath)
}

// GetFilePath returns the file path for this session
func (m *Manager) GetFilePath() string {
	return m.filePath
}

// MessageCount returns the number of messages in the session
func (m *Manager) MessageCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return len(m.session.Messages)
}