package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SlashCommandField is a custom text field with slash command autocomplete
type SlashCommandField struct {
	textarea    textarea.Model
	commands    []SlashCommand
	showPopup   bool
	filtered    []FuzzyMatch
	selected    int
	width       int
	height      int
	lastValue   string
	popupHeight int
}

// NewSlashCommandField creates a new slash command field
func NewSlashCommandField(width int) *SlashCommandField {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 5000
	ta.SetWidth(width)
	ta.SetHeight(3)
	ta.Focus()

	// Apply default styles
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ta.FocusedStyle.Text = lipgloss.NewStyle()
	ta.FocusedStyle.Prompt = lipgloss.NewStyle()
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	return &SlashCommandField{
		textarea:    ta,
		commands:    SlashCommands,
		width:       width,
		height:      3,
		popupHeight: 5, // Show max 5 items
	}
}

// Init implements tea.Model
func (s *SlashCommandField) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model
func (s *SlashCommandField) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Check for exit keys first (when popup is not shown)
		if !s.showPopup {
			switch msg.String() {
			case "ctrl+c":
				return s, tea.Quit
			case "enter":
				// Only quit on enter if we're not in multiline mode
				if !strings.Contains(s.textarea.Value(), "\n") {
					return s, tea.Quit
				}
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
			case key.Matches(msg, key.NewBinding(key.WithKeys("tab", "enter"))):
				if s.selected < len(s.filtered) {
					// Complete with selected command
					s.textarea.SetValue(s.filtered[s.selected].Command.Name)
					s.showPopup = false
					s.selected = 0
					// Move cursor to end
					s.textarea.CursorEnd()
				}
				return s, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				s.showPopup = false
				s.selected = 0
				return s, nil
			}
		}

		// Let textarea handle the key first
		var cmd tea.Cmd
		s.textarea, cmd = s.textarea.Update(msg)
		cmds = append(cmds, cmd)

		// Check if we should show/update popup
		value := s.textarea.Value()
		if value != s.lastValue {
			s.lastValue = value
			if strings.HasPrefix(value, "/") && !strings.Contains(value, " ") && !strings.Contains(value, "\n") {
				// Show and update popup
				s.showPopup = true
				s.filtered = FuzzyMatchCommands(value, s.commands)
				s.selected = 0
			} else {
				// Hide popup
				s.showPopup = false
			}
		}

		return s, tea.Batch(cmds...)

	default:
		// Pass through other messages
		var cmd tea.Cmd
		s.textarea, cmd = s.textarea.Update(msg)
		return s, cmd
	}
}

// View implements tea.Model
func (s *SlashCommandField) View() string {
	// Get the textarea view
	textareaView := s.textarea.View()

	if !s.showPopup || len(s.filtered) == 0 {
		return textareaView
	}

	// Build popup view
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(s.width - 4)
	var items []string
	maxItems := min(len(s.filtered), s.popupHeight)

	for i := 0; i < maxItems; i++ {
		match := s.filtered[i]
		cmd := match.Command

		// Format item
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

		// Highlight selected item
		if i == s.selected {
			nameStyle = nameStyle.
				Background(lipgloss.Color("237")).
				Foreground(lipgloss.Color("15"))
			descStyle = descStyle.
				Background(lipgloss.Color("237")).
				Foreground(lipgloss.Color("15"))
		}
		// Truncate description if needed
		desc := cmd.Description
		maxDescLen := s.width - len(cmd.Name) - 10
		if len(desc) > maxDescLen && maxDescLen > 3 {
			desc = desc[:maxDescLen-3] + "..."
		}

		line := nameStyle.Render(cmd.Name) + " " + descStyle.Render(desc)
		items = append(items, line)
	}

	popup := popupStyle.Render(strings.Join(items, "\n"))

	// Combine textarea and popup
	return lipgloss.JoinVertical(lipgloss.Left, textareaView, popup)
}

// Value returns the current value of the field
func (s *SlashCommandField) Value() string {
	return s.textarea.Value()
}

// SetValue sets the value of the field
func (s *SlashCommandField) SetValue(value string) {
	s.textarea.SetValue(value)
}

// Focus focuses the field
func (s *SlashCommandField) Focus() tea.Cmd {
	return s.textarea.Focus()
}

// Blur blurs the field
func (s *SlashCommandField) Blur() {
	s.textarea.Blur()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
