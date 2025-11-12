package tools

import (
	"sync"
)

// BufferedDebugLogger implements DebugLogger by storing debug messages in memory
// until they can be retrieved and displayed. This is useful when debug output
// needs to be deferred or batch-processed rather than immediately displayed.
// All methods are thread-safe for concurrent use.
type BufferedDebugLogger struct {
	enabled  bool
	messages []string
	mu       sync.Mutex
}

// NewBufferedDebugLogger creates a new buffered debug logger instance.
// The enabled parameter determines whether debug messages will be stored.
// If enabled is false, all LogDebug calls become no-ops for performance.
func NewBufferedDebugLogger(enabled bool) *BufferedDebugLogger {
	return &BufferedDebugLogger{
		enabled:  enabled,
		messages: make([]string, 0),
	}
}

// LogDebug stores a debug message in the internal buffer if debug logging is enabled.
// Messages are appended to the buffer and retained until GetMessages is called.
// If debug logging is disabled, this method is a no-op.
// Thread-safe for concurrent calls.
func (l *BufferedDebugLogger) LogDebug(message string) {
	if !l.enabled {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, message)
}

// IsDebugEnabled returns whether debug logging is enabled for this logger.
// This can be used to conditionally execute expensive debug operations
// only when debugging is actually enabled.
func (l *BufferedDebugLogger) IsDebugEnabled() bool {
	return l.enabled
}

// GetMessages returns all buffered debug messages and clears the internal buffer.
// The returned slice contains all messages logged since the last call to GetMessages.
// After this call, the internal buffer is empty and ready to accumulate new messages.
// Thread-safe for concurrent calls.
func (l *BufferedDebugLogger) GetMessages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	messages := make([]string, len(l.messages))
	copy(messages, l.messages)
	l.messages = l.messages[:0] // Clear the buffer
	return messages
}
