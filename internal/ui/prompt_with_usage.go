package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// InteractivePrompt is a comprehensive Bubble Tea component that handles:
// - Usage info display
// - Loading spinner during processing
// - Input form for user prompts
type InteractivePrompt struct {
	usageTracker *UsageTracker
	form         *huh.Form
	spinner      spinner.Model
	width        int
	compactMode  bool
	prompt       string
	submitted    bool
	showSpinner  bool
	spinnerMsg   string
	showForm     bool
}

// NewInteractivePrompt creates a new comprehensive prompt component
func NewInteractivePrompt(usageTracker *UsageTracker, width int, compactMode bool) *InteractivePrompt {
	p := &InteractivePrompt{
		usageTracker: usageTracker,
		width:        width,
		compactMode:  compactMode,
		showForm:     true, // Start with form visible
	}

	// Create form with reference to our prompt field
	p.form = huh.NewForm(huh.NewGroup(huh.NewText().
		Title("Enter your prompt (Type /help for commands, Ctrl+C to quit, ESC to cancel generation)").
		Value(&p.prompt).
		CharLimit(5000)),
	).WithWidth(width).
		WithTheme(huh.ThemeCharm())

	// Initialize spinner
	p.spinner = spinner.New()
	p.spinner.Spinner = spinner.Points
	theme := GetTheme()
	p.spinner.Style = p.spinner.Style.Foreground(theme.Primary)

	return p
}

// ShowSpinner shows the spinner with a message and hides the form
func (p *InteractivePrompt) ShowSpinner(message string) tea.Cmd {
	p.showSpinner = true
	p.spinnerMsg = message
	p.showForm = false
	return p.spinner.Tick
}

// HideSpinner hides the spinner and shows the form again
func (p *InteractivePrompt) HideSpinner() {
	p.showSpinner = false
	p.showForm = true
}

// Init implements tea.Model
func (p *InteractivePrompt) Init() tea.Cmd {
	if p.showForm {
		return p.form.Init()
	}
	return nil
}

// Update implements tea.Model
func (p *InteractivePrompt) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if p.showSpinner {
			// During spinner, only allow ESC to cancel
			switch msg.String() {
			case "esc":
				return p, tea.Quit
			case "ctrl+c":
				return p, tea.Quit
			}
			return p, nil
		}

		// Handle form input
		switch msg.String() {
		case "ctrl+c":
			return p, tea.Quit
		case "esc":
			p.prompt = ""
			p.submitted = true
			return p, tea.Quit
		}

	case spinner.TickMsg:
		if p.showSpinner {
			var cmd tea.Cmd
			p.spinner, cmd = p.spinner.Update(msg)
			return p, cmd
		}
	}

	// Update the form if it's visible
	if p.showForm {
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

	return p, nil
}

// View implements tea.Model
func (p *InteractivePrompt) View() string {
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

	// Display spinner or form
	if p.showSpinner {
		// Enhanced spinner display
		baseStyle := lipgloss.NewStyle()
		theme := GetTheme()

		spinnerStyle := baseStyle.
			Foreground(theme.Primary).
			Bold(true)

		messageStyle := baseStyle.
			Foreground(theme.Text).
			Italic(true)

		spinnerView := lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingTop(1).
			Render(spinnerStyle.Render(p.spinner.View()) + " " + messageStyle.Render(p.spinnerMsg))

		view.WriteString(spinnerView)
	} else if p.showForm {
		// Display the form
		view.WriteString(p.form.View())
	}

	return view.String()
}

// GetPrompt returns the entered prompt value
func (p *InteractivePrompt) GetPrompt() string {
	return p.prompt
}

// WasSubmitted returns whether the form was submitted (not cancelled)
func (p *InteractivePrompt) WasSubmitted() bool {
	return p.submitted && p.form.State == huh.StateCompleted
}

// Legacy compatibility - keep the old name for now
type PromptWithUsage = InteractivePrompt

// NewPromptWithUsage creates a new prompt component (legacy compatibility)
func NewPromptWithUsage(usageTracker *UsageTracker, width int, compactMode bool) *PromptWithUsage {
	return NewInteractivePrompt(usageTracker, width, compactMode)
}
