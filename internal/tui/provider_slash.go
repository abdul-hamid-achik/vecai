package tui

import "strings"

// SlashCommandProvider provides completion for slash commands and their arguments
type SlashCommandProvider struct {
	commands []CommandDef
	argDefs  map[string][]string // Command name → valid arguments
}

// NewSlashCommandProvider creates a provider with builtin commands
func NewSlashCommandProvider() *SlashCommandProvider {
	return &SlashCommandProvider{
		commands: BuiltinCommands,
		argDefs: map[string][]string{
			"/mode": {"fast", "smart", "genius"},
		},
	}
}

// Trigger returns TriggerSlash
func (p *SlashCommandProvider) Trigger() TriggerChar {
	return TriggerSlash
}

// IsAsync returns false — slash completion is instant
func (p *SlashCommandProvider) IsAsync() bool {
	return false
}

// AddCommands adds additional commands (e.g., from skills)
func (p *SlashCommandProvider) AddCommands(cmds []CommandDef) {
	p.commands = append(p.commands, cmds...)
}

// Complete returns matching commands or arguments.
// query is everything after "/" (e.g., "mo" for "/mo", "mode fast" for "/mode fast")
func (p *SlashCommandProvider) Complete(query string) []CompletionItem {
	// Check if we're completing arguments (query contains a space)
	if spaceIdx := strings.Index(query, " "); spaceIdx >= 0 {
		cmdName := "/" + query[:spaceIdx]
		argPrefix := strings.ToLower(strings.TrimSpace(query[spaceIdx+1:]))
		return p.completeArgs(cmdName, argPrefix)
	}

	// Command name completion
	return p.completeCommands(query)
}

// completeCommands filters commands by prefix
func (p *SlashCommandProvider) completeCommands(prefix string) []CompletionItem {
	lowerPrefix := strings.ToLower(prefix)
	var items []CompletionItem

	for _, cmd := range p.commands {
		// Match against command name without the leading "/"
		cmdWithout := strings.TrimPrefix(cmd.Name, "/")
		if !strings.HasPrefix(strings.ToLower(cmdWithout), lowerPrefix) {
			continue
		}

		label := cmd.Name
		if cmd.HasArgs && cmd.ArgHint != "" {
			label += " " + cmd.ArgHint
		}

		insertText := cmd.Name
		if cmd.HasArgs {
			insertText += " " // Trailing space to start typing args
		}

		items = append(items, CompletionItem{
			Label:      label,
			Detail:     cmd.Description,
			Kind:       KindCommand,
			InsertText: insertText,
			Score:      1.0,
		})
	}

	return items
}

// completeArgs returns argument completions for a known command
func (p *SlashCommandProvider) completeArgs(cmdName, argPrefix string) []CompletionItem {
	args, ok := p.argDefs[cmdName]
	if !ok {
		return nil
	}

	var items []CompletionItem
	for _, arg := range args {
		if argPrefix != "" && !strings.HasPrefix(strings.ToLower(arg), argPrefix) {
			continue
		}
		items = append(items, CompletionItem{
			Label:      arg,
			Detail:     cmdName + " " + arg,
			Kind:       KindArgument,
			InsertText: cmdName + " " + arg,
			Score:      1.0,
		})
	}

	return items
}
