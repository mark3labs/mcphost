package ui

import (
	"fmt"
	
	"github.com/charmbracelet/lipgloss"
)

// Enhanced styling utilities and theme definitions

// Global theme instance
var currentTheme = DefaultTheme()

// GetTheme returns the current theme
func GetTheme() Theme {
	return currentTheme
}

// SetTheme sets the current theme
func SetTheme(theme Theme) {
	currentTheme = theme
}

// Theme represents a complete UI theme
type Theme struct {
	Primary     lipgloss.AdaptiveColor
	Secondary   lipgloss.AdaptiveColor
	Success     lipgloss.AdaptiveColor
	Warning     lipgloss.AdaptiveColor
	Error       lipgloss.AdaptiveColor
	Info        lipgloss.AdaptiveColor
	Text        lipgloss.AdaptiveColor
	Muted       lipgloss.AdaptiveColor
	VeryMuted   lipgloss.AdaptiveColor
	Background  lipgloss.AdaptiveColor
	Border      lipgloss.AdaptiveColor
	MutedBorder lipgloss.AdaptiveColor
	System      lipgloss.AdaptiveColor
	Tool        lipgloss.AdaptiveColor
	Accent      lipgloss.AdaptiveColor
	Highlight   lipgloss.AdaptiveColor
}

// DefaultTheme returns the default MCPHost theme (Catppuccin Mocha)
func DefaultTheme() Theme {
	return Theme{
		Primary: lipgloss.AdaptiveColor{
			Light: "#8839ef", // Latte Mauve
			Dark:  "#cba6f7", // Mocha Mauve
		},
		Secondary: lipgloss.AdaptiveColor{
			Light: "#04a5e5", // Latte Sky
			Dark:  "#89dceb", // Mocha Sky
		},
		Success: lipgloss.AdaptiveColor{
			Light: "#40a02b", // Latte Green
			Dark:  "#a6e3a1", // Mocha Green
		},
		Warning: lipgloss.AdaptiveColor{
			Light: "#df8e1d", // Latte Yellow
			Dark:  "#f9e2af", // Mocha Yellow
		},
		Error: lipgloss.AdaptiveColor{
			Light: "#d20f39", // Latte Red
			Dark:  "#f38ba8", // Mocha Red
		},
		Info: lipgloss.AdaptiveColor{
			Light: "#1e66f5", // Latte Blue
			Dark:  "#89b4fa", // Mocha Blue
		},
		Text: lipgloss.AdaptiveColor{
			Light: "#4c4f69", // Latte Text
			Dark:  "#cdd6f4", // Mocha Text
		},
		Muted: lipgloss.AdaptiveColor{
			Light: "#6c6f85", // Latte Subtext 0
			Dark:  "#a6adc8", // Mocha Subtext 0
		},
		VeryMuted: lipgloss.AdaptiveColor{
			Light: "#9ca0b0", // Latte Overlay 0
			Dark:  "#6c7086", // Mocha Overlay 0
		},
		Background: lipgloss.AdaptiveColor{
			Light: "#eff1f5", // Latte Base
			Dark:  "#1e1e2e", // Mocha Base
		},
		Border: lipgloss.AdaptiveColor{
			Light: "#acb0be", // Latte Surface 2
			Dark:  "#585b70", // Mocha Surface 2
		},
		MutedBorder: lipgloss.AdaptiveColor{
			Light: "#ccd0da", // Latte Surface 0
			Dark:  "#313244", // Mocha Surface 0
		},
		System: lipgloss.AdaptiveColor{
			Light: "#179299", // Latte Teal
			Dark:  "#94e2d5", // Mocha Teal
		},
		Tool: lipgloss.AdaptiveColor{
			Light: "#fe640b", // Latte Peach
			Dark:  "#fab387", // Mocha Peach
		},
		Accent: lipgloss.AdaptiveColor{
			Light: "#ea76cb", // Latte Pink
			Dark:  "#f5c2e7", // Mocha Pink
		},
		Highlight: lipgloss.AdaptiveColor{
			Light: "#df8e1d", // Latte Yellow (for highlights)
			Dark:  "#45475a", // Mocha Surface 1 (subtle highlight)
		},
	}
}

// StyleCard creates a styled card container
func StyleCard(width int, theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(1, 2).
		MarginBottom(1)
}

// StyleHeader creates a styled header
func StyleHeader(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)
}

// StyleSubheader creates a styled subheader
func StyleSubheader(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Secondary).
		Bold(true)
}

// StyleMuted creates muted text styling
func StyleMuted(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Muted).
		Italic(true)
}

// StyleSuccess creates success text styling
func StyleSuccess(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Success).
		Bold(true)
}

// StyleError creates error text styling
func StyleError(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Error).
		Bold(true)
}

// StyleWarning creates warning text styling
func StyleWarning(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Warning).
		Bold(true)
}

// StyleInfo creates info text styling
func StyleInfo(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Info).
		Bold(true)
}

// CreateSeparator creates a styled separator line
func CreateSeparator(width int, char string, color lipgloss.AdaptiveColor) string {
	return lipgloss.NewStyle().
		Foreground(color).
		Width(width).
		Render(lipgloss.PlaceHorizontal(width, lipgloss.Center, char))
}

// CreateProgressBar creates a simple progress bar
func CreateProgressBar(width int, percentage float64, theme Theme) string {
	filled := int(float64(width) * percentage / 100)
	empty := width - filled

	filledBar := lipgloss.NewStyle().
		Foreground(theme.Success).
		Render(lipgloss.PlaceHorizontal(filled, lipgloss.Left, "█"))

	emptyBar := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Render(lipgloss.PlaceHorizontal(empty, lipgloss.Left, "░"))

	return filledBar + emptyBar
}

// CreateBadge creates a styled badge
func CreateBadge(text string, color lipgloss.AdaptiveColor) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}).
		Background(color).
		Padding(0, 1).
		Bold(true).
		Render(text)
}

// CreateGradientText creates text with gradient-like effect using different shades
func CreateGradientText(text string, startColor, endColor lipgloss.AdaptiveColor) string {
	// For now, just use the start color - true gradients would require more complex implementation
	return lipgloss.NewStyle().
		Foreground(startColor).
		Bold(true).
		Render(text)
}

// Compact styling utilities

// StyleCompactSymbol creates a styled symbol for compact mode
func StyleCompactSymbol(symbol string, color lipgloss.AdaptiveColor) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
}

// StyleCompactLabel creates a styled label for compact mode
func StyleCompactLabel(color lipgloss.AdaptiveColor) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Width(8)
}

// StyleCompactContent creates basic content styling for compact mode
func StyleCompactContent(color lipgloss.AdaptiveColor) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(color)
}

// FormatCompactLine formats a complete compact line with consistent spacing
func FormatCompactLine(symbol, label, content string, symbolColor, labelColor, contentColor lipgloss.AdaptiveColor) string {
	styledSymbol := StyleCompactSymbol(symbol, symbolColor).Render(symbol)
	styledLabel := StyleCompactLabel(labelColor).Render(label)
	styledContent := StyleCompactContent(contentColor).Render(content)
	
	return fmt.Sprintf("%s  %-8s %s", styledSymbol, styledLabel, styledContent)
}
