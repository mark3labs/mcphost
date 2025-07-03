package ui

import (
	"io"
	"os"
	"strings"
	"sync"

	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// TerminalRenderer provides flicker-free terminal rendering using termenv
type TerminalRenderer struct {
	output       *termenv.Output
	width        int
	height       int
	cursorRow    int
	cursorCol    int
	screenBuffer []string // Track current screen content
	mutex        sync.RWMutex
}

// NewTerminalRenderer creates a new terminal renderer
func NewTerminalRenderer(writer io.Writer) *TerminalRenderer {
	output := termenv.NewOutput(writer)

	// Get terminal size
	width, height := getTerminalSize()

	return &TerminalRenderer{
		output:       output,
		width:        width,
		height:       height,
		cursorRow:    0,
		cursorCol:    0,
		screenBuffer: make([]string, height),
	}
}

// MoveTo moves cursor to specific row and column (0-indexed)
func (tr *TerminalRenderer) MoveTo(row, col int) {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()

	tr.output.MoveCursor(row+1, col+1) // termenv uses 1-indexed
	tr.cursorRow = row
	tr.cursorCol = col
}

// WriteAt writes content at specific position without affecting other content
func (tr *TerminalRenderer) WriteAt(row, col int, content string) {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()

	// Move to position
	tr.output.MoveCursor(row+1, col+1)

	// Write content
	tr.output.WriteString(content)

	// Update internal tracking
	tr.cursorRow = row
	tr.cursorCol = col + len(content)

	// Update screen buffer
	if row < len(tr.screenBuffer) {
		if col == 0 {
			tr.screenBuffer[row] = content
		} else {
			// Insert content at specific column
			existing := tr.screenBuffer[row]
			if col >= len(existing) {
				tr.screenBuffer[row] = existing + strings.Repeat(" ", col-len(existing)) + content
			} else {
				tr.screenBuffer[row] = existing[:col] + content + existing[col+len(content):]
			}
		}
	}
}

// ClearLines clears specified number of lines starting from current position
func (tr *TerminalRenderer) ClearLines(count int) {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()

	for i := 0; i < count; i++ {
		tr.output.ClearLine()
		if i < count-1 {
			tr.output.CursorDown(1)
		}
	}

	// Update screen buffer
	for i := tr.cursorRow; i < tr.cursorRow+count && i < len(tr.screenBuffer); i++ {
		tr.screenBuffer[i] = ""
	}
}

// ClearFromCursor clears from cursor to end of screen
func (tr *TerminalRenderer) ClearFromCursor() {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()

	// Clear current line from cursor
	tr.output.WriteString("\033[K")

	// Clear all lines below
	for i := tr.cursorRow + 1; i < tr.height; i++ {
		tr.output.MoveCursor(i+1, 1)
		tr.output.ClearLine()
		if i < len(tr.screenBuffer) {
			tr.screenBuffer[i] = ""
		}
	}

	// Return to original position
	tr.output.MoveCursor(tr.cursorRow+1, tr.cursorCol+1)
}

// HideCursor hides the terminal cursor
func (tr *TerminalRenderer) HideCursor() {
	tr.output.HideCursor()
}

// ShowCursor shows the terminal cursor
func (tr *TerminalRenderer) ShowCursor() {
	tr.output.ShowCursor()
}

// UpdateSize updates terminal dimensions
func (tr *TerminalRenderer) UpdateSize() {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()

	width, height := getTerminalSize()
	tr.width = width
	tr.height = height

	// Resize screen buffer
	if height > len(tr.screenBuffer) {
		tr.screenBuffer = append(tr.screenBuffer, make([]string, height-len(tr.screenBuffer))...)
	} else if height < len(tr.screenBuffer) {
		tr.screenBuffer = tr.screenBuffer[:height]
	}
}

// GetSize returns current terminal dimensions
func (tr *TerminalRenderer) GetSize() (int, int) {
	tr.mutex.RLock()
	defer tr.mutex.RUnlock()
	return tr.width, tr.height
}

// GetCursorPosition returns current cursor position
func (tr *TerminalRenderer) GetCursorPosition() (int, int) {
	tr.mutex.RLock()
	defer tr.mutex.RUnlock()
	return tr.cursorRow, tr.cursorCol
}

// Helper function to get terminal size
func getTerminalSize() (int, int) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80, 24 // fallback
	}
	return width, height
}
