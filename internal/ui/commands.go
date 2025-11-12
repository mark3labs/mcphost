package ui

// SlashCommand represents a user-invokable slash command with its metadata.
// Commands can have multiple aliases and are organized by category for better
// discoverability and help display.
type SlashCommand struct {
	Name        string
	Description string
	Aliases     []string
	Category    string // e.g., "Navigation", "System", "Info"
}

// SlashCommands provides the global registry of all available slash commands
// in the application. Commands are organized by category (Info, System) and
// include their primary names, descriptions, and alternative aliases.
var SlashCommands = []SlashCommand{
	{
		Name:        "/help",
		Description: "Show available commands and usage information",
		Category:    "Info",
		Aliases:     []string{"/h", "/?"},
	},
	{
		Name:        "/tools",
		Description: "List all available MCP tools",
		Category:    "Info",
		Aliases:     []string{"/t"},
	},
	{
		Name:        "/servers",
		Description: "Show connected MCP servers",
		Category:    "Info",
		Aliases:     []string{"/s"},
	},

	{
		Name:        "/clear",
		Description: "Clear conversation and start fresh",
		Category:    "System",
		Aliases:     []string{"/c", "/cls"},
	},
	{
		Name:        "/usage",
		Description: "Show token usage statistics",
		Category:    "Info",
		Aliases:     []string{"/u"},
	},
	{
		Name:        "/reset-usage",
		Description: "Reset usage statistics",
		Category:    "System",
		Aliases:     []string{"/ru"},
	},
	{
		Name:        "/quit",
		Description: "Exit the application",
		Category:    "System",
		Aliases:     []string{"/q", "/exit"},
	},
}

// GetCommandByName looks up a slash command by its primary name or any of its
// aliases. Returns a pointer to the matching SlashCommand, or nil if no command
// matches the provided name.
func GetCommandByName(name string) *SlashCommand {
	for i := range SlashCommands {
		cmd := &SlashCommands[i]
		if cmd.Name == name {
			return cmd
		}
		for _, alias := range cmd.Aliases {
			if alias == name {
				return cmd
			}
		}
	}
	return nil
}

// GetAllCommandNames returns a complete list of all command names and their aliases.
// This is useful for command completion, validation, and help display. The returned
// slice contains both primary command names and all alternative aliases.
func GetAllCommandNames() []string {
	var names []string
	for _, cmd := range SlashCommands {
		names = append(names, cmd.Name)
		names = append(names, cmd.Aliases...)
	}
	return names
}
