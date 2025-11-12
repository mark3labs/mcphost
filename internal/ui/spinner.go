package ui

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Spinner provides an animated loading indicator that displays while long-running
// operations are in progress. It wraps the bubbles spinner component and manages
// its lifecycle through a tea.Program for proper terminal handling.
type Spinner struct {
	model  spinner.Model
	done   chan struct{}
	prog   *tea.Program
	ctx    context.Context
	cancel context.CancelFunc
}

// spinnerModel is the tea.Model for the spinner
type spinnerModel struct {
	spinner  spinner.Model
	message  string
	quitting bool
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.quitting = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case quitMsg:
		m.quitting = true
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m spinnerModel) View() string {
	if m.quitting {
		return ""
	}

	// Enhanced spinner display with better styling
	baseStyle := lipgloss.NewStyle()
	theme := GetTheme()

	spinnerStyle := baseStyle.
		Foreground(theme.Primary).
		Bold(true)

	messageStyle := baseStyle.
		Foreground(theme.Text).
		Italic(true)

	return fmt.Sprintf(" %s %s",
		spinnerStyle.Render(m.spinner.View()),
		messageStyle.Render(m.message))
}

// quitMsg is sent when we want to quit the spinner
type quitMsg struct{}

// NewSpinner creates a new animated spinner with the specified message. The spinner
// uses the theme's primary color and a modern animation style. It runs in a separate
// tea.Program to avoid interfering with other terminal operations.
func NewSpinner(message string) *Spinner {
	s := spinner.New()
	s.Spinner = spinner.Points // More modern spinner style
	theme := GetTheme()
	s.Style = s.Style.Foreground(theme.Primary)

	ctx, cancel := context.WithCancel(context.Background())

	model := spinnerModel{
		spinner: s,
		message: message,
	}

	prog := tea.NewProgram(model, tea.WithOutput(os.Stderr), tea.WithoutCatchPanics())

	return &Spinner{
		model:  s,
		done:   make(chan struct{}),
		prog:   prog,
		ctx:    ctx,
		cancel: cancel,
	}
}

// NewThemedSpinner creates a new animated spinner with custom color styling.
// This allows for different spinner colors based on the operation type or status.
// The spinner runs independently in its own tea.Program.
func NewThemedSpinner(message string, color lipgloss.AdaptiveColor) *Spinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = s.Style.Foreground(color)

	ctx, cancel := context.WithCancel(context.Background())

	model := spinnerModel{
		spinner: s,
		message: message,
	}

	prog := tea.NewProgram(model, tea.WithOutput(os.Stderr), tea.WithoutCatchPanics())

	return &Spinner{
		model:  s,
		done:   make(chan struct{}),
		prog:   prog,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the spinner animation in a separate goroutine. The spinner will
// continue animating until Stop is called. The animation runs in a separate
// tea.Program to maintain smooth animation independent of other operations.
func (s *Spinner) Start() {
	go func() {
		defer close(s.done)
		go func() {
			<-s.ctx.Done()
			s.prog.Send(quitMsg{})
		}()
		_, err := s.prog.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running spinner: %v\n", err)
		}
	}()
}

// Stop halts the spinner animation and cleans up resources. This method blocks
// until the spinner has fully stopped and the terminal state is restored.
func (s *Spinner) Stop() {
	s.cancel()
	<-s.done
}
