package ui

import (
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
)

const defaultMargin = 1

// Helper functions for style pointers
func boolPtr(b bool) *bool       { return &b }
func stringPtr(s string) *string { return &s }
func uintPtr(u uint) *uint       { return &u }

// BaseStyle returns a basic lipgloss style
func BaseStyle() lipgloss.Style {
	return lipgloss.NewStyle()
}

// GetMarkdownRenderer returns a glamour TermRenderer configured for our use
func GetMarkdownRenderer(width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(generateMarkdownStyleConfig()),
		glamour.WithWordWrap(width),
	)
	return r
}

// generateMarkdownStyleConfig creates an ansi.StyleConfig for markdown rendering
func generateMarkdownStyleConfig() ansi.StyleConfig {
	// Define adaptive colors based on terminal background
	var textColor, mutedColor string
	if lipgloss.HasDarkBackground() {
		textColor = "#F9FAFB"  // Light text for dark backgrounds
		mutedColor = "#9CA3AF" // Light muted for dark backgrounds
	} else {
		textColor = "#1F2937"  // Dark text for light backgrounds
		mutedColor = "#6B7280" // Dark muted for light backgrounds
	}
	var headingColor, emphColor, strongColor, linkColor, codeColor, errorColor, keywordColor, stringColor, numberColor, commentColor string
	if lipgloss.HasDarkBackground() {
		// Dark background colors
		headingColor = "#22D3EE" // Cyan
		emphColor = "#FDE047"    // Yellow
		strongColor = "#F9FAFB"  // Light gray
		linkColor = "#60A5FA"    // Blue
		codeColor = "#D1D5DB"    // Light gray
		errorColor = "#F87171"   // Red
		keywordColor = "#C084FC" // Purple
		stringColor = "#34D399"  // Green
		numberColor = "#FBBF24"  // Orange
		commentColor = "#9CA3AF" // Muted gray
	} else {
		// Light background colors
		headingColor = "#0891B2" // Dark cyan
		emphColor = "#D97706"    // Orange
		strongColor = "#1F2937"  // Dark gray
		linkColor = "#2563EB"    // Blue
		codeColor = "#374151"    // Dark gray
		errorColor = "#DC2626"   // Red
		keywordColor = "#7C3AED" // Purple
		stringColor = "#059669"  // Green
		numberColor = "#D97706"  // Orange
		commentColor = "#6B7280" // Muted gray
	}

	// Don't apply background in markdown - let the block renderer handle it
	bgColor := ""

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "",
				BlockSuffix: "",
				Color:       stringPtr(textColor),
			},
			Margin: uintPtr(0), // Remove margin to prevent spacing
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(mutedColor),
				Italic: boolPtr(true),
				Prefix: "┃ ",
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr(lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: bgColor, Dark: bgColor}).Render(" ")),
		},
		List: ansi.StyleList{
			LevelIndent: 0, // Remove list indentation
			StyleBlock: ansi.StyleBlock{
				IndentToken: stringPtr(lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: bgColor, Dark: bgColor}).Render(" ")),
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr(textColor),
				},
			},
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       stringPtr(headingColor),
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "# ",
				Color:  stringPtr(headingColor),
				Bold:   boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
				Color:  stringPtr(headingColor),
				Bold:   boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Color:  stringPtr(headingColor),
				Bold:   boolPtr(true),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
				Color:  stringPtr(headingColor),
				Bold:   boolPtr(true),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
				Color:  stringPtr(headingColor),
				Bold:   boolPtr(true),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  stringPtr(headingColor),
				Bold:   boolPtr(true),
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: boolPtr(true),
			Color:      stringPtr(mutedColor),
		},
		Emph: ansi.StylePrimitive{
			Color: stringPtr(emphColor),

			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold:  boolPtr(true),
			Color: stringPtr(strongColor),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  stringPtr(mutedColor),
			Format: "\n─────────────────────────────────────────\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       stringPtr(textColor),
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       stringPtr(textColor),
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{},
			Ticked:         "[✓] ",
			Unticked:       "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color: stringPtr(linkColor),

			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr(linkColor),

			Bold: boolPtr(true),
		},
		Image: ansi.StylePrimitive{
			Color: stringPtr(linkColor),

			Underline: boolPtr(true),
			Format:    "🖼 {{.text}}",
		},
		ImageText: ansi.StylePrimitive{
			Color: stringPtr(linkColor),

			Format: "{{.text}}",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(codeColor),

				Prefix: "",
				Suffix: "",
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Prefix: "",
					Color:  stringPtr(codeColor),
				},
				Margin: uintPtr(0), // Remove margin
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: stringPtr(textColor),
				},
				Error: ansi.StylePrimitive{
					Color: stringPtr(errorColor),
				},
				Comment: ansi.StylePrimitive{
					Color: stringPtr(commentColor),
				},
				CommentPreproc: ansi.StylePrimitive{
					Color: stringPtr(keywordColor),
				},
				Keyword: ansi.StylePrimitive{
					Color: stringPtr(keywordColor),
				},
				KeywordReserved: ansi.StylePrimitive{
					Color: stringPtr(keywordColor),
				},
				KeywordNamespace: ansi.StylePrimitive{
					Color: stringPtr(keywordColor),
				},
				KeywordType: ansi.StylePrimitive{
					Color: stringPtr(keywordColor),
				},
				Operator: ansi.StylePrimitive{
					Color: stringPtr(textColor),
				},
				Punctuation: ansi.StylePrimitive{
					Color: stringPtr(textColor),
				},
				Name: ansi.StylePrimitive{
					Color: stringPtr(textColor),
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: stringPtr(textColor),
				},
				NameTag: ansi.StylePrimitive{
					Color: stringPtr(keywordColor),
				},
				NameAttribute: ansi.StylePrimitive{
					Color: stringPtr(textColor),
				},
				NameClass: ansi.StylePrimitive{
					Color: stringPtr(keywordColor),
				},
				NameConstant: ansi.StylePrimitive{
					Color: stringPtr(textColor),
				},
				NameDecorator: ansi.StylePrimitive{
					Color: stringPtr(textColor),
				},
				NameFunction: ansi.StylePrimitive{
					Color: stringPtr(textColor),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: stringPtr(numberColor),
				},
				LiteralString: ansi.StylePrimitive{
					Color: stringPtr(stringColor),
				},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: stringPtr(keywordColor),
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: stringPtr(errorColor),
				},
				GenericEmph: ansi.StylePrimitive{
					Color: stringPtr(emphColor),

					Italic: boolPtr(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: stringPtr(stringColor),
				},
				GenericStrong: ansi.StylePrimitive{
					Color: stringPtr(strongColor),

					Bold: boolPtr(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: stringPtr(headingColor),
				},
			},
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					BlockPrefix: "\n",
					BlockSuffix: "\n",
				},
			},
			CenterSeparator: stringPtr("┼"),
			ColumnSeparator: stringPtr("│"),
			RowSeparator:    stringPtr("─"),
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n ❯ ",
			Color:       stringPtr(linkColor),
		},
		Text: ansi.StylePrimitive{
			Color: stringPtr(textColor),
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(textColor),
			},
		},
	}
}

// toMarkdown renders markdown content using glamour
func toMarkdown(content string, width int) string {
	r := GetMarkdownRenderer(width)
	rendered, _ := r.Render(content)
	return rendered
}
