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

// WithFullWidth returns a renderingOption that configures the block renderer
// to expand to the full available width of its container. When enabled, the
// block will fill the entire horizontal space rather than sizing to its content.
func WithFullWidth() renderingOption {
	return func(c *blockRenderer) {
		c.fullWidth = true
	}
}

// WithAlign returns a renderingOption that sets the horizontal alignment
// of the block content within its container. The align parameter accepts
// lipgloss.Left, lipgloss.Center, or lipgloss.Right positions.
func WithAlign(align lipgloss.Position) renderingOption {
	return func(c *blockRenderer) {
		c.align = &align
	}
}

// WithBorderColor returns a renderingOption that sets the border color
// for the block. The color parameter uses lipgloss.AdaptiveColor to support
// both light and dark terminal themes automatically.
func WithBorderColor(color lipgloss.AdaptiveColor) renderingOption {
	return func(c *blockRenderer) {
		c.borderColor = &color
	}
}

// WithMarginTop returns a renderingOption that sets the top margin
// for the block. The margin is specified in number of lines and adds
// vertical space above the block.
func WithMarginTop(margin int) renderingOption {
	return func(c *blockRenderer) {
		c.marginTop = margin
	}
}

// WithMarginBottom returns a renderingOption that sets the bottom margin
// for the block. The margin is specified in number of lines and adds
// vertical space below the block.
func WithMarginBottom(margin int) renderingOption {
	return func(c *blockRenderer) {
		c.marginBottom = margin
	}
}

// WithPaddingLeft returns a renderingOption that sets the left padding
// for the block content. The padding is specified in number of characters
// and adds horizontal space between the left border and the content.
func WithPaddingLeft(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingLeft = padding
	}
}

// WithPaddingRight returns a renderingOption that sets the right padding
// for the block content. The padding is specified in number of characters
// and adds horizontal space between the content and the right border.
func WithPaddingRight(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingRight = padding
	}
}

// WithPaddingTop returns a renderingOption that sets the top padding
// for the block content. The padding is specified in number of lines
// and adds vertical space between the top border and the content.
func WithPaddingTop(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingTop = padding
	}
}

// WithPaddingBottom returns a renderingOption that sets the bottom padding
// for the block content. The padding is specified in number of lines
// and adds vertical space between the content and the bottom border.
func WithPaddingBottom(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingBottom = padding
	}
}

// WithWidth returns a renderingOption that sets a specific width for the block
// in characters. This overrides the default container width and allows precise
// control over the block's horizontal dimensions.
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
