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
	// Define colors - using simple colors since we're not implementing theming
	textColor := "#ffffff"
	mutedColor := "#888888"
	headingColor := "#00d7ff"
	emphColor := "#ffff87"
	strongColor := "#ffffff"
	linkColor := "#5fd7ff"
	codeColor := "#d7d7af"
	errorColor := "#ff5f5f"
	keywordColor := "#ff87d7"
	stringColor := "#87ff87"
	numberColor := "#ffaf87"
	commentColor := "#5f5f87"

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "",
				BlockSuffix: "",
				Color:       stringPtr(textColor),
			},
			Margin: uintPtr(defaultMargin),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(mutedColor),
				Italic: boolPtr(true),
				Prefix: "┃ ",
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr(BaseStyle().Render(" ")),
		},
		List: ansi.StyleList{
			LevelIndent: defaultMargin,
			StyleBlock: ansi.StyleBlock{
				IndentToken: stringPtr(BaseStyle().Render(" ")),
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
			Color:  stringPtr(emphColor),
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
			Color:     stringPtr(linkColor),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr(linkColor),
			Bold:  boolPtr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     stringPtr(linkColor),
			Underline: boolPtr(true),
			Format:    "🖼 {{.text}}",
		},
		ImageText: ansi.StylePrimitive{
			Color:  stringPtr(linkColor),
			Format: "{{.text}}",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr(codeColor),
				Prefix: "",
				Suffix: "",
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Prefix: " ",
					Color:  stringPtr(codeColor),
				},
				Margin: uintPtr(defaultMargin),
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
					Color:  stringPtr(emphColor),
					Italic: boolPtr(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: stringPtr(stringColor),
				},
				GenericStrong: ansi.StylePrimitive{
					Color: stringPtr(strongColor),
					Bold:  boolPtr(true),
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