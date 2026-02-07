package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// CommandDef defines a slash command for autocomplete
type CommandDef struct {
	Name        string // e.g., "/help"
	Description string // e.g., "Show available commands"
	HasArgs     bool   // Whether the command accepts arguments
	ArgHint     string // e.g., "<goal>" or "<tier>"
}

// BuiltinCommands is the list of all built-in slash commands.
// Duplicated from agent/commands.go to avoid circular import tui↔agent.
var BuiltinCommands = []CommandDef{
	{Name: "/help", Description: "Show available commands"},
	{Name: "/ask", Description: "Switch to Ask mode (read-only)"},
	{Name: "/plan", Description: "Switch to Plan mode, or plan a goal", HasArgs: true, ArgHint: "[goal]"},
	{Name: "/build", Description: "Switch to Build mode (full execution)"},
	{Name: "/mode", Description: "Switch model tier", HasArgs: true, ArgHint: "<fast|smart|genius>"},
	{Name: "/copy", Description: "Copy conversation to clipboard"},
	{Name: "/skills", Description: "List available skills"},
	{Name: "/status", Description: "Check vecgrep index status"},
	{Name: "/reindex", Description: "Update vecgrep search index"},
	{Name: "/context", Description: "Show context usage breakdown"},
	{Name: "/compact", Description: "Compact conversation", HasArgs: true, ArgHint: "[focus]"},
	{Name: "/sessions", Description: "List saved sessions"},
	{Name: "/resume", Description: "Resume a session", HasArgs: true, ArgHint: "[id]"},
	{Name: "/new", Description: "Start a new session"},
	{Name: "/delete", Description: "Delete a session", HasArgs: true, ArgHint: "<id>"},
	{Name: "/clear", Description: "Clear conversation"},
	{Name: "/exit", Description: "Exit interactive mode"},
}

// Completer provides slash command autocomplete functionality
type Completer struct {
	allCommands  []CommandDef
	filtered     []CommandDef
	selected     int
	active       bool
	maxVisible   int // Maximum visible items in dropdown
	scrollOffset int
}

// NewCompleter creates a new Completer with builtin commands
func NewCompleter() *Completer {
	return &Completer{
		allCommands: BuiltinCommands,
		maxVisible:  8,
	}
}

// AddCommands adds additional commands (e.g., from skills)
func (c *Completer) AddCommands(cmds []CommandDef) {
	c.allCommands = append(c.allCommands, cmds...)
}

// Update updates the completer state based on the current input text
func (c *Completer) Update(input string) {
	// Only activate when input starts with "/" and has no space (not typing args)
	if !strings.HasPrefix(input, "/") || strings.Contains(input, " ") {
		c.active = false
		c.filtered = nil
		c.selected = 0
		c.scrollOffset = 0
		return
	}

	// Filter commands by prefix (case-insensitive)
	prefix := strings.ToLower(input)
	var filtered []CommandDef
	for _, cmd := range c.allCommands {
		if strings.HasPrefix(strings.ToLower(cmd.Name), prefix) {
			filtered = append(filtered, cmd)
		}
	}

	c.filtered = filtered
	c.active = len(filtered) > 0

	// Reset selection if it's out of bounds
	if c.selected >= len(c.filtered) {
		c.selected = 0
		c.scrollOffset = 0
	}
}

// IsActive returns whether the completer dropdown should be shown
func (c *Completer) IsActive() bool {
	return c.active
}

// MoveUp moves the selection up
func (c *Completer) MoveUp() {
	if !c.active || len(c.filtered) == 0 {
		return
	}
	c.selected--
	if c.selected < 0 {
		c.selected = len(c.filtered) - 1
		// Scroll to show the last item
		if len(c.filtered) > c.maxVisible {
			c.scrollOffset = len(c.filtered) - c.maxVisible
		}
	}
	// Adjust scroll to keep selection visible
	if c.selected < c.scrollOffset {
		c.scrollOffset = c.selected
	}
}

// MoveDown moves the selection down
func (c *Completer) MoveDown() {
	if !c.active || len(c.filtered) == 0 {
		return
	}
	c.selected++
	if c.selected >= len(c.filtered) {
		c.selected = 0
		c.scrollOffset = 0
	}
	// Adjust scroll to keep selection visible
	if c.selected >= c.scrollOffset+c.maxVisible {
		c.scrollOffset = c.selected - c.maxVisible + 1
	}
}

// Accept returns the selected command name and dismisses the dropdown.
// Returns empty string if nothing is selected.
func (c *Completer) Accept() string {
	if !c.active || len(c.filtered) == 0 {
		return ""
	}
	cmd := c.filtered[c.selected]
	c.Dismiss()
	if cmd.HasArgs {
		return cmd.Name + " "
	}
	return cmd.Name
}

// Dismiss hides the dropdown
func (c *Completer) Dismiss() {
	c.active = false
	c.filtered = nil
	c.selected = 0
	c.scrollOffset = 0
}

// VisibleCount returns how many items are visible in the dropdown
func (c *Completer) VisibleCount() int {
	if len(c.filtered) < c.maxVisible {
		return len(c.filtered)
	}
	return c.maxVisible
}

// Render renders the dropdown popup. Returns empty string if not active.
func (c *Completer) Render(width int) string {
	if !c.active || len(c.filtered) == 0 {
		return ""
	}

	var lines []string
	visible := c.VisibleCount()
	for i := c.scrollOffset; i < c.scrollOffset+visible && i < len(c.filtered); i++ {
		cmd := c.filtered[i]

		// Build the line: name + arg hint + description
		name := cmd.Name
		if cmd.HasArgs && cmd.ArgHint != "" {
			name += " " + cmd.ArgHint
		}

		// Pad name to fixed width for alignment
		nameWidth := 22
		if len(name) < nameWidth {
			name += strings.Repeat(" ", nameWidth-len(name))
		}

		line := name + " " + cmd.Description

		// Truncate if too long
		maxLineWidth := width - 4
		if maxLineWidth < 20 {
			maxLineWidth = 20
		}
		if len(line) > maxLineWidth {
			line = line[:maxLineWidth-3] + "..."
		}

		if i == c.selected {
			lines = append(lines, completerSelectedStyle.Render(line))
		} else {
			lines = append(lines, completerItemStyle.Render(line))
		}
	}

	// Add scroll indicators
	if c.scrollOffset > 0 {
		lines = append([]string{completerScrollStyle.Render("  ▲ more")}, lines...)
	}
	if c.scrollOffset+visible < len(c.filtered) {
		lines = append(lines, completerScrollStyle.Render("  ▼ more"))
	}

	content := strings.Join(lines, "\n")
	return completerBoxStyle.Width(width).Render(content)
}

// Completer styles
var (
	completerBoxStyle = lipgloss.NewStyle().
				Background(colorBgElevate).
				Foreground(colorText).
				Padding(0, 1)

	completerItemStyle = lipgloss.NewStyle().
				Foreground(colorText)

	completerSelectedStyle = lipgloss.NewStyle().
				Foreground(colorTextBold).
				Background(lipgloss.Color("#434c5e")). // Slightly brighter than nord1
				Bold(true)

	completerScrollStyle = lipgloss.NewStyle().
				Foreground(colorDim)
)
