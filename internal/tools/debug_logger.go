package tools

// DebugLogger defines the interface for debug logging in the MCP tools package.
// Implementations can provide different strategies for handling debug output,
// such as immediate console output, buffering, or file logging.
// All implementations must be thread-safe for concurrent use.
type DebugLogger interface {
	// LogDebug logs a debug message. Implementations determine how the message is handled.
	LogDebug(message string)
	// IsDebugEnabled returns true if debug logging is enabled, allowing callers
	// to skip expensive debug operations when debugging is disabled.
	IsDebugEnabled() bool
}

// SimpleDebugLogger provides a minimal implementation of the DebugLogger interface.
// It is intentionally silent by default to prevent duplicate or unstyled debug output
// during initialization. Debug messages are only displayed when using the CLI debug logger
// which provides proper formatting and styling.
type SimpleDebugLogger struct {
	enabled bool
}

// NewSimpleDebugLogger creates a new simple debug logger instance.
// The enabled parameter determines whether IsDebugEnabled will return true.
// Note that LogDebug is intentionally a no-op to avoid unstyled output;
// actual debug output is handled by the CLI's debug logger.
func NewSimpleDebugLogger(enabled bool) *SimpleDebugLogger {
	return &SimpleDebugLogger{enabled: enabled}
}

// LogDebug is intentionally a no-op in SimpleDebugLogger.
// Debug messages are only displayed when using the CLI debug logger which provides
// proper formatting and styling. This prevents duplicate or unstyled debug output
// during initialization and ensures consistent debug output presentation.
func (l *SimpleDebugLogger) LogDebug(message string) {
	// Silent by default - messages will only appear when using CLI debug logger
	// This prevents duplicate or unstyled debug output during initialization
}

// IsDebugEnabled returns whether debug logging is enabled for this logger.
// This allows code to conditionally execute expensive debug operations
// only when debugging is active, improving performance in production.
func (l *SimpleDebugLogger) IsDebugEnabled() bool {
	return l.enabled
}
