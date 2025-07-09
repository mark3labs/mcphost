package ui

// SlashCommand represents a slash command with its metadata
type SlashCommand struct {
	Name        string
	Description string
	Aliases     []string
	Category    string // e.g., "Navigation", "System", "Info"
}

// SlashCommands is the registry of all available slash commands
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

// GetCommandByName returns a command by its name or alias
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

// GetAllCommandNames returns all command names and aliases
func GetAllCommandNames() []string {
	var names []string
	for _, cmd := range SlashCommands {
		names = append(names, cmd.Name)
		names = append(names, cmd.Aliases...)
	}
	return names
}
