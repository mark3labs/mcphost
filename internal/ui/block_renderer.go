package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// blockRenderer handles rendering of content blocks with configurable options
type blockRenderer struct {
	align         *lipgloss.Position
	borderColor   *lipgloss.AdaptiveColor
	fullWidth     bool
	paddingTop    int
	paddingBottom int
	paddingLeft   int
	paddingRight  int
	marginTop     int
	marginBottom  int
	width         int
}

// renderingOption configures block rendering
type renderingOption func(*blockRenderer)

// WithFullWidth makes the block take full available width
func WithFullWidth() renderingOption {
	return func(c *blockRenderer) {
		c.fullWidth = true
	}
}

// WithAlign sets the horizontal alignment of the block
func WithAlign(align lipgloss.Position) renderingOption {
	return func(c *blockRenderer) {
		c.align = &align
	}
}

// WithBorderColor sets the border color
func WithBorderColor(color lipgloss.AdaptiveColor) renderingOption {
	return func(c *blockRenderer) {
		c.borderColor = &color
	}
}

// WithMarginTop sets the top margin
func WithMarginTop(margin int) renderingOption {
	return func(c *blockRenderer) {
		c.marginTop = margin
	}
}

// WithMarginBottom sets the bottom margin
func WithMarginBottom(margin int) renderingOption {
	return func(c *blockRenderer) {
		c.marginBottom = margin
	}
}

// WithPaddingLeft sets the left padding
func WithPaddingLeft(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingLeft = padding
	}
}

// WithPaddingRight sets the right padding
func WithPaddingRight(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingRight = padding
	}
}

// WithPaddingTop sets the top padding
func WithPaddingTop(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingTop = padding
	}
}

// WithPaddingBottom sets the bottom padding
func WithPaddingBottom(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingBottom = padding
	}
}

// WithWidth sets a specific width for the block
func WithWidth(width int) renderingOption {
	return func(c *blockRenderer) {
		c.width = width
	}
}

// renderContentBlock renders content with configurable styling options
func renderContentBlock(content string, containerWidth int, options ...renderingOption) string {
	renderer := &blockRenderer{
		fullWidth:     false,
		paddingTop:    1,
		paddingBottom: 1,
		paddingLeft:   2,
		paddingRight:  2,
		width:         containerWidth,
	}

	for _, option := range options {
		option(renderer)
	}

	theme := GetTheme()
	style := lipgloss.NewStyle().
		PaddingTop(renderer.paddingTop).
		PaddingBottom(renderer.paddingBottom).
		PaddingLeft(renderer.paddingLeft).
		PaddingRight(renderer.paddingRight).
		Foreground(theme.Text).
		BorderStyle(lipgloss.ThickBorder())

	align := lipgloss.Left
	if renderer.align != nil {
		align = *renderer.align
	}

	// Default to transparent/no border color
	borderColor := lipgloss.AdaptiveColor{Light: "", Dark: ""}
	if renderer.borderColor != nil {
		borderColor = *renderer.borderColor
	}

	// Very muted color for the opposite border
	mutedOppositeBorder := lipgloss.AdaptiveColor{
		Light: "#F3F4F6", // Very light gray, barely visible
		Dark:  "#1F2937", // Very dark gray, barely visible
	}

	switch align {
	case lipgloss.Left:
		style = style.
			BorderLeft(true).
			BorderRight(true).
			AlignHorizontal(align).
			BorderLeftForeground(borderColor).
			BorderRightForeground(mutedOppositeBorder)
	case lipgloss.Right:
		style = style.
			BorderRight(true).
			BorderLeft(true).
			AlignHorizontal(align).
			BorderRightForeground(borderColor).
			BorderLeftForeground(mutedOppositeBorder)
	}

	if renderer.fullWidth {
		style = style.Width(renderer.width)
	}

	content = style.Render(content)

	// Place the content horizontally with proper background
	content = lipgloss.PlaceHorizontal(
		renderer.width,
		align,
		content,
	)

	// Add margins
	if renderer.marginTop > 0 {
		for range renderer.marginTop {
			content = "\n" + content
		}
	}
	if renderer.marginBottom > 0 {
		for range renderer.marginBottom {
			content = content + "\n"
		}
	}

	return content
}
