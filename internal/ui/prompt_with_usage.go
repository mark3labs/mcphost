package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// PromptWithUsage is a Bubble Tea component that displays usage info and a prompt form
type PromptWithUsage struct {
	usageTracker *UsageTracker
	form         *huh.Form
	width        int
	compactMode  bool
	prompt       string
	submitted    bool
}

// NewPromptWithUsage creates a new prompt component with usage display
func NewPromptWithUsage(usageTracker *UsageTracker, width int, compactMode bool) *PromptWithUsage {
	p := &PromptWithUsage{
		usageTracker: usageTracker,
		width:        width,
		compactMode:  compactMode,
	}

	// Create form with reference to our prompt field
	p.form = huh.NewForm(huh.NewGroup(huh.NewText().
		Title("Enter your prompt (Type /help for commands, Ctrl+C to quit, ESC to cancel generation)").
		Value(&p.prompt).
		CharLimit(5000)),
	).WithWidth(width).
		WithTheme(huh.ThemeCharm())

	return p
}

// Init implements tea.Model
func (p *PromptWithUsage) Init() tea.Cmd {
	return p.form.Init()
}

// Update implements tea.Model
func (p *PromptWithUsage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return p, tea.Quit
		case "esc":
			p.prompt = ""
			p.submitted = true
			return p, tea.Quit
		}
	}

	// Update the form
	form, cmd := p.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		p.form = f
		if p.form.State == huh.StateCompleted {
			p.submitted = true
			return p, tea.Quit
		}
	}

	return p, cmd
}

// View implements tea.Model
func (p *PromptWithUsage) View() string {
	var view strings.Builder

	// Display usage info if available
	if p.usageTracker != nil {
		usageInfo := p.usageTracker.RenderUsageInfo()
		if usageInfo != "" {
			paddedUsage := lipgloss.NewStyle().
				PaddingLeft(2).
				PaddingTop(1).
				Render(usageInfo)
			view.WriteString(paddedUsage)
			view.WriteString("\n")
		}
	}

	// Display the form
	view.WriteString(p.form.View())

	return view.String()
}

// GetPrompt returns the entered prompt value
func (p *PromptWithUsage) GetPrompt() string {
	return p.prompt
}

// WasSubmitted returns whether the form was submitted (not cancelled)
func (p *PromptWithUsage) WasSubmitted() bool {
	return p.submitted && p.form.State == huh.StateCompleted
}
