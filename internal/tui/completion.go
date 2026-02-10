package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TriggerChar identifies what activates a completion provider
type TriggerChar rune

const (
	TriggerSlash TriggerChar = '/'
	TriggerAt    TriggerChar = '@'
)

// CompletionKind classifies a completion item
type CompletionKind int

const (
	KindCommand  CompletionKind = iota // Slash command (/help, /mode)
	KindArgument                       // Command argument (fast, smart, genius)
	KindFile                           // File path (@agent.go)
	KindChunk                          // Code chunk — function/class (@agent.go:Run)
)

// CompletionItem is a single item in the completion dropdown
type CompletionItem struct {
	Label      string         // Primary display text
	Detail     string         // Secondary text (description, path)
	Kind       CompletionKind // Classification
	InsertText string         // Text to insert on accept
	Score      float64        // Relevance score (0-1), higher = better
	FilePath   string         // For file items: path for context injection
	Language   string         // For file items: programming language
}

// CompletionProvider generates completion items for a trigger character
type CompletionProvider interface {
	// Trigger returns the character that activates this provider
	Trigger() TriggerChar
	// Complete returns items matching the query synchronously
	Complete(query string) []CompletionItem
	// IsAsync returns true if this provider does async work (needs debouncing)
	IsAsync() bool
}

// CompletionEngine orchestrates providers and manages completion state.
// Replaces the old Completer. Pointer-based to survive Bubbletea model copies.
type CompletionEngine struct {
	providers map[TriggerChar]CompletionProvider

	// State
	active        bool
	activeTrigger TriggerChar
	items         []CompletionItem
	selected      int
	scrollOffset  int
	maxVisible    int

	// Query tracking
	query    string // Text after trigger char
	rawInput string // Full input text
	loading  bool   // Async search in progress
}

// NewCompletionEngine creates a new engine with the given providers
func NewCompletionEngine(providers ...CompletionProvider) *CompletionEngine {
	e := &CompletionEngine{
		providers:  make(map[TriggerChar]CompletionProvider),
		maxVisible: 8,
	}
	for _, p := range providers {
		e.providers[p.Trigger()] = p
	}
	return e
}

// RegisterProvider adds or replaces a completion provider
func (e *CompletionEngine) RegisterProvider(p CompletionProvider) {
	e.providers[p.Trigger()] = p
}

// Update processes new input text and dispatches to the appropriate provider
func (e *CompletionEngine) Update(input string) {
	e.rawInput = input

	// Try slash trigger: input starts with "/" and has no space yet (command name phase)
	// OR input starts with "/" followed by a known command and a space (argument phase)
	if strings.HasPrefix(input, "/") {
		if p, ok := e.providers[TriggerSlash]; ok {
			// The slash provider handles both command and argument completion
			query := input[1:] // Everything after "/"
			items := p.Complete(query)
			e.activeTrigger = TriggerSlash
			e.query = query
			e.items = items
			e.active = len(items) > 0
			if e.selected >= len(e.items) {
				e.selected = 0
				e.scrollOffset = 0
			}
			return
		}
	}

	// Try @ trigger: find the last '@' in input that isn't followed by a space
	if atIdx := findActiveTrigger(input, '@'); atIdx >= 0 {
		if p, ok := e.providers[TriggerAt]; ok {
			query := input[atIdx+1:]
			items := p.Complete(query)
			e.activeTrigger = TriggerAt
			e.query = query
			e.items = items
			e.active = len(items) > 0
			if e.selected >= len(e.items) {
				e.selected = 0
				e.scrollOffset = 0
			}
			return
		}
	}

	// No trigger matched
	e.Dismiss()
}

// findActiveTrigger finds the last occurrence of trigger char that is still "active"
// (preceded by whitespace or start-of-string, and not followed by a space after the query)
func findActiveTrigger(input string, trigger byte) int {
	// Scan backwards to find the last trigger char
	for i := len(input) - 1; i >= 0; i-- {
		if input[i] == trigger {
			// Must be at start or preceded by whitespace
			if i == 0 || input[i-1] == ' ' || input[i-1] == '\t' || input[i-1] == '\n' {
				// Check that the text after @ has no space (still typing the query)
				afterTrigger := input[i+1:]
				if !strings.Contains(afterTrigger, " ") {
					return i
				}
			}
		}
	}
	return -1
}

// IsActive returns whether the dropdown should be shown
func (e *CompletionEngine) IsActive() bool {
	return e.active
}

// IsLoading returns whether an async search is in progress
func (e *CompletionEngine) IsLoading() bool {
	return e.loading
}

// ActiveTrigger returns the currently active trigger character
func (e *CompletionEngine) ActiveTrigger() TriggerChar {
	return e.activeTrigger
}

// MoveUp moves the selection up, wrapping around
func (e *CompletionEngine) MoveUp() {
	if !e.active || len(e.items) == 0 {
		return
	}
	e.selected--
	if e.selected < 0 {
		e.selected = len(e.items) - 1
		if len(e.items) > e.maxVisible {
			e.scrollOffset = len(e.items) - e.maxVisible
		}
	}
	if e.selected < e.scrollOffset {
		e.scrollOffset = e.selected
	}
}

// MoveDown moves the selection down, wrapping around
func (e *CompletionEngine) MoveDown() {
	if !e.active || len(e.items) == 0 {
		return
	}
	e.selected++
	if e.selected >= len(e.items) {
		e.selected = 0
		e.scrollOffset = 0
	}
	if e.selected >= e.scrollOffset+e.maxVisible {
		e.scrollOffset = e.selected - e.maxVisible + 1
	}
}

// Accept returns the selected item and dismisses the dropdown
func (e *CompletionEngine) Accept() CompletionItem {
	if !e.active || len(e.items) == 0 {
		return CompletionItem{}
	}
	item := e.items[e.selected]
	e.Dismiss()
	return item
}

// Dismiss hides the dropdown and resets state
func (e *CompletionEngine) Dismiss() {
	e.active = false
	e.items = nil
	e.selected = 0
	e.scrollOffset = 0
	e.loading = false
}

// SetLoading sets the loading indicator for async providers
func (e *CompletionEngine) SetLoading(loading bool) {
	e.loading = loading
}

// MergeAsyncResults merges async vecgrep results with existing sync results.
// Deduplicates by FilePath. Async items that don't already exist are appended.
// Re-sorts by score descending.
func (e *CompletionEngine) MergeAsyncResults(items []CompletionItem) {
	if len(items) == 0 {
		e.loading = false
		return
	}

	// Build a set of existing file paths for dedup
	existing := make(map[string]bool, len(e.items))
	for _, item := range e.items {
		if item.FilePath != "" {
			existing[item.FilePath] = true
		}
		// Also deduplicate by InsertText for files without AbsPath
		if item.InsertText != "" {
			existing[item.InsertText] = true
		}
	}

	// Append non-duplicate async results
	for _, item := range items {
		if item.FilePath != "" && existing[item.FilePath] {
			continue
		}
		if item.InsertText != "" && existing[item.InsertText] {
			continue
		}
		e.items = append(e.items, item)
	}

	// Re-sort by score descending
	for i := 1; i < len(e.items); i++ {
		for j := i; j > 0 && e.items[j].Score > e.items[j-1].Score; j-- {
			e.items[j], e.items[j-1] = e.items[j-1], e.items[j]
		}
	}

	// Cap results
	if len(e.items) > 12 {
		e.items = e.items[:12]
	}

	e.active = len(e.items) > 0
	e.loading = false
	if e.selected >= len(e.items) {
		e.selected = 0
	}
}

// VisibleCount returns how many items are visible in the dropdown
func (e *CompletionEngine) VisibleCount() int {
	if len(e.items) < e.maxVisible {
		return len(e.items)
	}
	return e.maxVisible
}

// Render renders the dropdown popup. Returns empty string if not active.
func (e *CompletionEngine) Render(width int) string {
	if !e.active || len(e.items) == 0 {
		return ""
	}

	switch e.activeTrigger {
	case TriggerAt:
		return e.renderFileDropdown(width)
	default:
		return e.renderCommandDropdown(width)
	}
}

// renderCommandDropdown renders the slash command dropdown (matches existing style)
func (e *CompletionEngine) renderCommandDropdown(width int) string {
	var lines []string
	visible := e.VisibleCount()
	for i := e.scrollOffset; i < e.scrollOffset+visible && i < len(e.items); i++ {
		item := e.items[i]

		// Build line: label + detail
		name := item.Label
		nameWidth := 22
		if len(name) < nameWidth {
			name += strings.Repeat(" ", nameWidth-len(name))
		}

		line := name + " " + item.Detail

		// Truncate if too long
		maxLineWidth := width - 4
		if maxLineWidth < 20 {
			maxLineWidth = 20
		}
		if len(line) > maxLineWidth {
			line = line[:maxLineWidth-3] + "..."
		}

		if i == e.selected {
			lines = append(lines, completerSelectedStyle.Render(line))
		} else {
			lines = append(lines, completerItemStyle.Render(line))
		}
	}

	// Scroll indicators
	if e.scrollOffset > 0 {
		lines = append([]string{completerScrollStyle.Render("  \u25b2 more")}, lines...)
	}
	if e.scrollOffset+visible < len(e.items) {
		lines = append(lines, completerScrollStyle.Render("  \u25bc more"))
	}

	content := strings.Join(lines, "\n")
	return completerBoxStyle.Width(width).Render(content)
}

// renderFileDropdown renders the @ file mention dropdown
func (e *CompletionEngine) renderFileDropdown(width int) string {
	var lines []string

	// Loading indicator
	if e.loading {
		lines = append(lines, completerScrollStyle.Render("  Searching..."))
	}

	visible := e.VisibleCount()
	for i := e.scrollOffset; i < e.scrollOffset+visible && i < len(e.items); i++ {
		item := e.items[i]

		// Language badge (2 chars)
		langBadge := fileLangBadge(item.Language)

		// File path
		path := item.Label
		maxPathWidth := width - 12 // badge + score + padding
		if maxPathWidth < 10 {
			maxPathWidth = 10
		}
		if len(path) > maxPathWidth {
			path = "..." + path[len(path)-maxPathWidth+3:]
		}

		line := langBadge + " " + path
		if item.Detail != "" {
			remaining := width - len(line) - 6
			if remaining > 5 {
				detail := item.Detail
				if len(detail) > remaining {
					detail = detail[:remaining-3] + "..."
				}
				line += "  " + completerScrollStyle.Render(detail)
			}
		}

		if i == e.selected {
			lines = append(lines, completerSelectedStyle.Render(line))
		} else {
			lines = append(lines, completerItemStyle.Render(line))
		}
	}

	// Scroll indicators
	if e.scrollOffset > 0 {
		lines = append([]string{completerScrollStyle.Render("  \u25b2 more")}, lines...)
	}
	if e.scrollOffset+visible < len(e.items) {
		lines = append(lines, completerScrollStyle.Render("  \u25bc more"))
	}

	if len(lines) == 0 {
		return ""
	}

	content := strings.Join(lines, "\n")
	return completerBoxStyle.Width(width).Render(content)
}

// fileLangBadge returns a 2-char colored language badge
func fileLangBadge(lang string) string {
	lang = strings.ToLower(lang)
	switch lang {
	case "go":
		return fileLangGoStyle.Render("GO")
	case "python", "py":
		return fileLangPyStyle.Render("PY")
	case "javascript", "js":
		return fileLangJsStyle.Render("JS")
	case "typescript", "ts":
		return fileLangTsStyle.Render("TS")
	case "rust", "rs":
		return fileLangRsStyle.Render("RS")
	case "yaml", "yml":
		return fileLangDefaultStyle.Render("YM")
	case "markdown", "md":
		return fileLangDefaultStyle.Render("MD")
	default:
		if len(lang) >= 2 {
			return fileLangDefaultStyle.Render(strings.ToUpper(lang[:2]))
		}
		return fileLangDefaultStyle.Render("  ")
	}
}

// File mention dropdown styles (Nord-themed)
var (
	fileLangGoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#81a1c1")). // nord9 blue
			Bold(true)

	fileLangPyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a3be8c")). // nord14 green
			Bold(true)

	fileLangJsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ebcb8b")). // nord13 yellow
			Bold(true)

	fileLangTsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5e81ac")). // nord10 blue
			Bold(true)

	fileLangRsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#bf616a")). // nord11 red
			Bold(true)

	fileLangDefaultStyle = lipgloss.NewStyle().
				Foreground(colorMuted)
)
