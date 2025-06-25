package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Enhanced styling utilities and theme definitions

// Theme represents a complete UI theme
type Theme struct {
	Primary    lipgloss.AdaptiveColor
	Secondary  lipgloss.AdaptiveColor
	Success    lipgloss.AdaptiveColor
	Warning    lipgloss.AdaptiveColor
	Error      lipgloss.AdaptiveColor
	Info       lipgloss.AdaptiveColor
	Text       lipgloss.AdaptiveColor
	Muted      lipgloss.AdaptiveColor
	Background lipgloss.AdaptiveColor
	Border     lipgloss.AdaptiveColor
}

// DefaultTheme returns the default MCPHost theme
func DefaultTheme() Theme {
	return Theme{
		Primary: lipgloss.AdaptiveColor{
			Light: "#7C3AED", // Purple
			Dark:  "#A855F7",
		},
		Secondary: lipgloss.AdaptiveColor{
			Light: "#06B6D4", // Cyan
			Dark:  "#22D3EE",
		},
		Success: lipgloss.AdaptiveColor{
			Light: "#059669", // Green
			Dark:  "#10B981",
		},
		Warning: lipgloss.AdaptiveColor{
			Light: "#D97706", // Orange
			Dark:  "#F59E0B",
		},
		Error: lipgloss.AdaptiveColor{
			Light: "#DC2626", // Red
			Dark:  "#F87171",
		},
		Info: lipgloss.AdaptiveColor{
			Light: "#2563EB", // Blue
			Dark:  "#60A5FA",
		},
		Text: lipgloss.AdaptiveColor{
			Light: "#1F2937", // Dark gray
			Dark:  "#F9FAFB", // Light gray
		},
		Muted: lipgloss.AdaptiveColor{
			Light: "#6B7280", // Medium gray
			Dark:  "#9CA3AF",
		},
		Background: lipgloss.AdaptiveColor{
			Light: "#FFFFFF", // White
			Dark:  "#111827", // Dark gray
		},
		Border: lipgloss.AdaptiveColor{
			Light: "#E5E7EB", // Light gray
			Dark:  "#374151", // Medium gray
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
