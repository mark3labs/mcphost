package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Enhanced styling utilities and theme definitions

// Global theme instance
var currentTheme = DefaultTheme()

// GetTheme returns the currently active UI theme. The theme controls all color
// and styling decisions throughout the application's interface.
func GetTheme() Theme {
	return currentTheme
}

// SetTheme updates the global UI theme, affecting all subsequent rendering
// operations. This allows runtime theme switching for different visual preferences.
func SetTheme(theme Theme) {
	currentTheme = theme
}

// Theme defines a comprehensive color scheme for the application's UI, supporting
// both light and dark terminal modes through adaptive colors. It includes semantic
// colors for different message types and UI elements, based on the Catppuccin color palette.
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

// DefaultTheme creates and returns the default MCPHost theme based on the Catppuccin
// Mocha (dark) and Latte (light) color palettes. This theme provides a cohesive,
// pleasant visual experience with carefully selected colors for different UI elements.
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

// StyleCard creates a lipgloss style for card-like containers with rounded borders,
// padding, and appropriate width. Used for grouping related content in a visually
// distinct box.
func StyleCard(width int, theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(1, 2).
		MarginBottom(1)
}

// StyleHeader creates a lipgloss style for primary headers using the theme's
// primary color with bold text for emphasis and hierarchy.
func StyleHeader(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)
}

// StyleSubheader creates a lipgloss style for secondary headers using the theme's
// secondary color with bold text, providing visual hierarchy below primary headers.
func StyleSubheader(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Secondary).
		Bold(true)
}

// StyleMuted creates a lipgloss style for de-emphasized text using muted colors
// and italic formatting, suitable for supplementary or less important information.
func StyleMuted(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Muted).
		Italic(true)
}

// StyleSuccess creates a lipgloss style for success messages using green colors
// with bold text to indicate successful operations or positive outcomes.
func StyleSuccess(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Success).
		Bold(true)
}

// StyleError creates a lipgloss style for error messages using red colors
// with bold text to ensure visibility of problems or failures.
func StyleError(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Error).
		Bold(true)
}

// StyleWarning creates a lipgloss style for warning messages using yellow/amber
// colors with bold text to draw attention to potential issues or cautions.
func StyleWarning(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Warning).
		Bold(true)
}

// StyleInfo creates a lipgloss style for informational messages using blue colors
// with bold text for general notifications and status updates.
func StyleInfo(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Info).
		Bold(true)
}

// CreateSeparator generates a horizontal separator line with the specified width,
// character, and color. Useful for visually dividing sections of content in the UI.
func CreateSeparator(width int, char string, color lipgloss.AdaptiveColor) string {
	return lipgloss.NewStyle().
		Foreground(color).
		Width(width).
		Render(lipgloss.PlaceHorizontal(width, lipgloss.Center, char))
}

// CreateProgressBar generates a visual progress bar with filled and empty segments
// based on the percentage complete. The bar uses Unicode block characters for smooth
// appearance and theme colors to indicate progress.
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

// CreateBadge generates a styled badge or label with inverted colors (text on
// colored background) for highlighting important tags, statuses, or categories.
func CreateBadge(text string, color lipgloss.AdaptiveColor) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}).
		Background(color).
		Padding(0, 1).
		Bold(true).
		Render(text)
}

// CreateGradientText creates styled text with a gradient-like effect. Currently
// implements a simplified version using the start color only, as true gradients
// require more complex terminal capabilities.
func CreateGradientText(text string, startColor, endColor lipgloss.AdaptiveColor) string {
	// For now, just use the start color - true gradients would require more complex implementation
	return lipgloss.NewStyle().
		Foreground(startColor).
		Bold(true).
		Render(text)
}

// Compact styling utilities

// StyleCompactSymbol creates a lipgloss style for message type indicators in
// compact mode, using bold colored text to distinguish different message categories.
func StyleCompactSymbol(symbol string, color lipgloss.AdaptiveColor) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
}

// StyleCompactLabel creates a lipgloss style for message labels in compact mode
// with fixed width for alignment and bold colored text for readability.
func StyleCompactLabel(color lipgloss.AdaptiveColor) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Width(8)
}

// StyleCompactContent creates a simple lipgloss style for message content in
// compact mode, applying only color without additional formatting.
func StyleCompactContent(color lipgloss.AdaptiveColor) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(color)
}

// FormatCompactLine assembles a complete compact mode message line with consistent
// spacing and styling. Combines a symbol, fixed-width label, and content with their
// respective colors to create a uniform appearance across all message types.
func FormatCompactLine(symbol, label, content string, symbolColor, labelColor, contentColor lipgloss.AdaptiveColor) string {
	styledSymbol := StyleCompactSymbol(symbol, symbolColor).Render(symbol)
	styledLabel := StyleCompactLabel(labelColor).Render(label)
	styledContent := StyleCompactContent(contentColor).Render(content)

	return fmt.Sprintf("%s  %-8s %s", styledSymbol, styledLabel, styledContent)
}
