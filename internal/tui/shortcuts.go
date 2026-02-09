package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Shortcut defines a keyboard shortcut for the help overlay
type Shortcut struct {
	Key         string
	Description string
	Category    string
}

// shortcutRegistry holds all registered shortcuts grouped by category
var shortcutRegistry = []Shortcut{
	// Navigation
	{"Up/Down", "Scroll viewport", "Navigation"},
	{"PgUp/PgDn", "Page up/down", "Navigation"},
	{"Home/End", "Jump to top/bottom", "Navigation"},

	// Input
	{"Enter", "Submit message", "Input"},
	{"Alt+Enter", "Insert newline", "Input"},
	{"Up (empty)", "Previous input history", "Input"},
	{"Down", "Next input history", "Input"},
	{"Tab", "Accept autocomplete", "Input"},

	// Modes & Controls
	{"Shift+Tab", "Cycle mode (Ask/Plan/Build)", "Modes"},
	{"Ctrl+T", "Toggle collapse tool blocks", "Modes"},
	{"Ctrl+Y", "Copy last response", "Modes"},
	{"Ctrl+K", "Copy last code block", "Modes"},
	{"ESC", "Stop streaming / dismiss", "Modes"},

	// Commands
	{"/help", "Show commands", "Commands"},
	{"/exit", "Exit vecai", "Commands"},
	{"/clear", "Clear conversation", "Commands"},
	{"/compact", "Compact context", "Commands"},

	// General
	{"F1", "Toggle this help", "General"},
	{"Ctrl+C", "Quit", "General"},
}

// renderHelpOverlay renders a centered help overlay with all shortcuts
func renderHelpOverlay(width, height int) string {
	// Group shortcuts by category
	categories := make(map[string][]Shortcut)
	var categoryOrder []string
	for _, s := range shortcutRegistry {
		if _, exists := categories[s.Category]; !exists {
			categoryOrder = append(categoryOrder, s.Category)
		}
		categories[s.Category] = append(categories[s.Category], s)
	}

	// Build content
	var lines []string
	lines = append(lines, overlayTitleStyle.Render("  Keyboard Shortcuts  "))
	lines = append(lines, "")

	for _, cat := range categoryOrder {
		lines = append(lines, overlayCategoryStyle.Render(cat))
		for _, s := range categories[cat] {
			key := overlayKeyStyle.Render(padRight(s.Key, 18))
			desc := overlayDescStyle.Render(s.Description)
			lines = append(lines, "  "+key+desc)
		}
		lines = append(lines, "")
	}

	lines = append(lines, overlayHintStyle.Render("Press F1 or ESC to close"))

	content := strings.Join(lines, "\n")

	// Style the overlay box
	overlayWidth := 48
	if width < overlayWidth+4 {
		overlayWidth = width - 4
	}

	box := overlayBoxStyle.
		Width(overlayWidth).
		Render(content)

	// Center in viewport
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// padRight pads a string to a minimum width
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// Overlay styles
var (
	overlayBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Background(lipgloss.Color("#2e3440")). // nord0
			Foreground(colorText).
			Padding(1, 2)

	overlayTitleStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	overlayCategoryStyle = lipgloss.NewStyle().
				Foreground(colorAccent2).
				Bold(true)

	overlayKeyStyle = lipgloss.NewStyle().
			Foreground(colorTextBold).
			Bold(true)

	overlayDescStyle = lipgloss.NewStyle().
				Foreground(colorText)

	overlayHintStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true).
			Align(lipgloss.Center)
)
