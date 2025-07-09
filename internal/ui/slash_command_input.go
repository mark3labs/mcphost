package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SlashCommandInput is a custom input field with slash command autocomplete
type SlashCommandInput struct {
	textarea      textarea.Model
	commands      []SlashCommand
	showPopup     bool
	filtered      []FuzzyMatch
	selected      int
	width         int
	lastValue     string
	popupHeight   int
	title         string
	quitting      bool
	value         string
	submitNext    bool // Flag to submit on next update
	renderedLines int  // Track how many lines were rendered
}

// NewSlashCommandInput creates a new slash command input field
func NewSlashCommandInput(width int, title string) *SlashCommandInput {
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 5000
	ta.SetWidth(width - 6) // Account for border and padding
	ta.SetHeight(3)        // Default to 3 lines like huh
	ta.Focus()

	// Style the textarea to match huh theme
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	ta.FocusedStyle.Prompt = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	return &SlashCommandInput{
		textarea:    ta,
		commands:    SlashCommands,
		width:       width,
		popupHeight: 7,
		title:       title,
	}
}

// Init implements tea.Model
func (s *SlashCommandInput) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model
func (s *SlashCommandInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Check if we need to submit after updating the view
	if s.submitNext {
		s.value = s.textarea.Value()
		s.quitting = true
		return s, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg: // Check for quit keys first (when popup is not shown)
		if !s.showPopup {
			switch msg.String() {
			case "ctrl+c", "esc":
				s.quitting = true
				return s, tea.Quit
			case "ctrl+d": // Submit on Ctrl+D like huh
				s.value = s.textarea.Value()
				s.quitting = true
				return s, tea.Quit
			}

			// Check for submit keys (Ctrl+J or Alt+Enter for new line)
			if key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+j", "alt+enter"))) {
				// These are for new lines, let textarea handle them
			} else if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) && !strings.Contains(s.textarea.Value(), "\n") {
				// Submit on Enter only if it's single line
				s.value = s.textarea.Value()
				s.quitting = true
				return s, tea.Quit
			}
		}

		// Handle popup navigation
		if s.showPopup {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up"))):
				if s.selected > 0 {
					s.selected--
				}
				return s, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down"))):
				if s.selected < len(s.filtered)-1 && s.selected < s.popupHeight-1 {
					s.selected++
				}
				return s, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
				if s.selected < len(s.filtered) {
					// Complete with selected command
					s.textarea.SetValue(s.filtered[s.selected].Command.Name)
					s.showPopup = false
					s.selected = 0
					// Move cursor to end
					s.textarea.CursorEnd()
				}
				return s, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				if s.selected < len(s.filtered) {
					// Populate the field with the selected command
					s.textarea.SetValue(s.filtered[s.selected].Command.Name)
					s.textarea.CursorEnd()
					// Hide the popup
					s.showPopup = false
					s.selected = 0
					// Set flag to submit on next update (after view refresh)
					s.submitNext = true
					// Force a refresh
					return s, nil
				}
				return s, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				s.showPopup = false
				s.selected = 0
				return s, nil
			}
		}

		// Update textarea
		s.textarea, cmd = s.textarea.Update(msg)

		// Check if we should show/update popup
		value := s.textarea.Value()
		if value != s.lastValue {
			s.lastValue = value
			// Only show popup if we're on the first line and it starts with /
			lines := strings.Split(value, "\n")
			if len(lines) > 0 && strings.HasPrefix(lines[0], "/") && !strings.Contains(lines[0], " ") && len(lines) == 1 {
				// Show and update popup
				s.showPopup = true
				s.filtered = FuzzyMatchCommands(lines[0], s.commands)
				s.selected = 0
			} else {
				// Hide popup
				s.showPopup = false
			}
		}
		return s, cmd

	default:
		// Pass through other messages
		s.textarea, cmd = s.textarea.Update(msg)
		return s, cmd
	}
}

// View implements tea.Model
func (s *SlashCommandInput) View() string {
	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		MarginBottom(1)

	// Input box with huh-like styling
	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderRight(false).
		BorderTop(false).
		BorderBottom(false).
		BorderForeground(lipgloss.Color("39")).
		PaddingLeft(1).
		Width(s.width)

	// Build the view
	var view strings.Builder
	view.WriteString(titleStyle.Render(s.title))
	view.WriteString("\n")
	view.WriteString(inputBoxStyle.Render(s.textarea.View()))

	// Count rendered lines
	s.renderedLines = 2 + s.textarea.Height() // title + newline + textarea height

	// Add popup if visible
	if s.showPopup && len(s.filtered) > 0 {
		view.WriteString("\n")
		view.WriteString(s.renderPopup())
		// Add popup lines
		popupLines := min(len(s.filtered), s.popupHeight) + 5 // items + border + padding + footer
		s.renderedLines += 1 + popupLines                     // newline + popup
	}

	// Add help text at bottom
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MarginTop(1)

	// Show different help based on whether we have multiline content
	helpText := "enter submit"
	if strings.Contains(s.textarea.Value(), "\n") {
		helpText = "ctrl+d submit • enter new line"
	} else {
		helpText = "enter submit • ctrl+j / alt+enter new line"
	}

	view.WriteString("\n")
	view.WriteString(helpStyle.Render(helpText))
	s.renderedLines += 2 // newline + help text

	return view.String()
}

// renderPopup renders the autocomplete popup
func (s *SlashCommandInput) renderPopup() string {
	// Popup styling
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("236")).
		Padding(1, 2).
		Background(lipgloss.Color("234")).
		Width(s.width - 2).
		MarginLeft(2) // Align with input field

	var items []string
	maxItems := min(len(s.filtered), s.popupHeight)

	for i := 0; i < maxItems; i++ {
		match := s.filtered[i]
		cmd := match.Command

		// Create the selection indicator
		var indicator string
		if i == s.selected {
			indicator = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Render("> ")
		} else {
			indicator = "  "
		}

		// Format item
		nameStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

		descStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

		// Highlight selected item
		if i == s.selected {
			nameStyle = nameStyle.Foreground(lipgloss.Color("87"))
			descStyle = descStyle.Foreground(lipgloss.Color("250"))
		}

		// Format with proper spacing
		nameWidth := 15
		name := nameStyle.Width(nameWidth - 2).Render(cmd.Name)

		// Truncate description if needed
		desc := cmd.Description
		maxDescLen := s.width - nameWidth - 10
		if len(desc) > maxDescLen && maxDescLen > 3 {
			desc = desc[:maxDescLen-3] + "..."
		}

		line := indicator + name + descStyle.Render(desc)
		items = append(items, line)
	}

	// Join items
	content := strings.Join(items, "\n")

	// Add footer hint
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("238")).
		Italic(true)
	footer := footerStyle.Render("↑↓ navigate • tab complete • ↵ select • esc dismiss")

	// Combine content and footer
	popupContent := content + "\n\n" + footer

	return popupStyle.Render(popupContent)
}

// Value returns the final value
func (s *SlashCommandInput) Value() string {
	return s.value
}

// Cancelled returns true if the user cancelled
func (s *SlashCommandInput) Cancelled() bool {
	return s.quitting && s.value == ""
}

// RenderedLines returns how many lines were rendered
func (s *SlashCommandInput) RenderedLines() int {
	return s.renderedLines
}
