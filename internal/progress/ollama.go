package progress

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render

const (
	padding  = 2
	maxWidth = 80
)

// OllamaPullProgress represents the progress information from Ollama pull API
type OllamaPullProgress struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

// progressMsg represents progress updates
type progressMsg struct {
	percent float64
	status  string
}

// progressErrMsg represents errors during progress
type progressErrMsg struct{ err error }

// progressCompleteMsg indicates completion
type progressCompleteMsg struct{}

// ProgressModel represents the progress bar model
type ProgressModel struct {
	progress progress.Model
	status   string
	err      error
	complete bool
}

// NewProgressModel creates a new progress model
func NewProgressModel() ProgressModel {
	return ProgressModel{
		progress: progress.New(progress.WithDefaultGradient()),
		status:   "Initializing...",
	}
}

// Init initializes the progress model
func (m ProgressModel) Init() tea.Cmd {
	return nil
}

// Update handles progress updates
func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		return m, nil

	case progressErrMsg:
		m.err = msg.err
		return m, tea.Quit

	case progressCompleteMsg:
		m.complete = true
		return m, tea.Quit

	case progressMsg:
		var cmds []tea.Cmd
		m.status = msg.status

		if msg.percent >= 1.0 {
			m.complete = true
			cmds = append(cmds, tea.Quit)
		}

		cmds = append(cmds, m.progress.SetPercent(msg.percent))
		return m, tea.Batch(cmds...)

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	default:
		return m, nil
	}
}

// View renders the progress bar
func (m ProgressModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %s\n", m.err.Error())
	}

	if m.complete {
		return fmt.Sprintf("\n%s%s\n\n%sComplete!\n",
			strings.Repeat(" ", padding),
			m.progress.View(),
			strings.Repeat(" ", padding))
	}

	pad := strings.Repeat(" ", padding)
	return fmt.Sprintf("\n%s%s\n%s%s\n\n%s",
		pad, m.progress.View(),
		pad, m.status,
		pad+helpStyle("Press 'q' or Ctrl+C to cancel"))
}

// ProgressReader wraps an io.Reader to provide progress updates for Ollama pull operations
type ProgressReader struct {
	reader   io.Reader
	program  *tea.Program
	model    ProgressModel
	lastLine string
	done     chan struct{}
	wg       sync.WaitGroup
}

// NewProgressReader creates a new progress reader for Ollama pull operations
func NewProgressReader(reader io.Reader) *ProgressReader {
	model := NewProgressModel()
	// Create program with standard settings
	program := tea.NewProgram(model)

	pr := &ProgressReader{
		reader:  reader,
		program: program,
		model:   model,
		done:    make(chan struct{}),
	}

	// Start the TUI in a goroutine
	pr.wg.Add(1)
	go func() {
		defer pr.wg.Done()
		if _, err := program.Run(); err != nil {
			// Handle error silently for now
		}
		close(pr.done)
	}()

	return pr
}

// Read implements io.Reader and parses Ollama streaming responses
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	if n > 0 {
		// Parse the JSON lines for progress information
		data := string(p[:n])
		pr.lastLine += data

		// Process complete lines
		for {
			lineEnd := strings.Index(pr.lastLine, "\n")
			if lineEnd == -1 {
				break
			}

			line := strings.TrimSpace(pr.lastLine[:lineEnd])
			pr.lastLine = pr.lastLine[lineEnd+1:]

			if line != "" {
				pr.parseProgressLine(line)
			}
		}
	}

	if err == io.EOF {
		// Send completion message and ensure program quits
		pr.program.Send(progressCompleteMsg{})
	}

	return n, err
}

// parseProgressLine parses a single JSON line from Ollama pull response
func (pr *ProgressReader) parseProgressLine(line string) {
	var progress OllamaPullProgress
	if err := json.Unmarshal([]byte(line), &progress); err != nil {
		return // Ignore malformed JSON
	}

	var percent float64
	status := progress.Status

	// Calculate progress percentage if we have total and completed
	if progress.Total > 0 && progress.Completed >= 0 {
		percent = float64(progress.Completed) / float64(progress.Total)

		// Format status with progress info
		if progress.Digest != "" {
			status = fmt.Sprintf("%s (%s)", progress.Status, progress.Digest[:12])
		}

		// Add size information
		if progress.Total > 0 {
			totalMB := float64(progress.Total) / (1024 * 1024)
			completedMB := float64(progress.Completed) / (1024 * 1024)
			status = fmt.Sprintf("%s - %.1f/%.1f MB", status, completedMB, totalMB)
		}
	} else {
		// For status-only updates (like "pulling manifest"), show indeterminate progress
		if strings.Contains(strings.ToLower(progress.Status), "pulling") ||
			strings.Contains(strings.ToLower(progress.Status), "downloading") {
			// Keep current progress or show small progress for activity
			percent = 0.1
		} else if strings.Contains(strings.ToLower(progress.Status), "success") ||
			strings.Contains(strings.ToLower(progress.Status), "complete") {
			percent = 1.0
		}
	}

	pr.program.Send(progressMsg{
		percent: percent,
		status:  status,
	})
}

// Close stops the progress display and waits for cleanup
func (pr *ProgressReader) Close() error {
	// Send completion message to trigger quit
	pr.program.Send(progressCompleteMsg{})

	// Wait for the program to finish with timeout
	done := make(chan struct{})
	go func() {
		pr.wg.Wait()
		close(done)
	}()

	// Wait for completion or timeout after 2 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case <-done:
		// Program finished normally
	case <-ctx.Done():
		// Timeout - force kill the program
		pr.program.Kill()
	}

	return nil
}
