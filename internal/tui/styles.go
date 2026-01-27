package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Nord Theme Colors
// https://www.nordtheme.com/docs/colors-and-palettes
var (
	// Polar Night - dark backgrounds
	nord0 = lipgloss.Color("#2e3440") // Base background
	nord1 = lipgloss.Color("#3b4252") // Elevated surfaces
	nord2 = lipgloss.Color("#434c5e") // Selection/active
	nord3 = lipgloss.Color("#4c566a") // Comments/subtle

	// Snow Storm - text
	nord4 = lipgloss.Color("#d8dee9") // Primary text
	nord5 = lipgloss.Color("#e5e9f0") // Secondary text
	nord6 = lipgloss.Color("#eceff4") // Bright text

	// Frost - accents
	nord7  = lipgloss.Color("#8fbcbb") // Calm accent (teal)
	nord8  = lipgloss.Color("#88c0d0") // Primary accent (cyan)
	nord9  = lipgloss.Color("#81a1c1") // Secondary accent (blue)
	nord10 = lipgloss.Color("#5e81ac") // Tertiary accent (deep blue)

	// Aurora - semantic
	nord11 = lipgloss.Color("#bf616a") // Error (red)
	nord12 = lipgloss.Color("#d08770") // Warning (orange)
	nord13 = lipgloss.Color("#ebcb8b") // Caution (yellow)
	nord14 = lipgloss.Color("#a3be8c") // Success (green)
	nord15 = lipgloss.Color("#b48ead") // Special (purple)
)

// Semantic color aliases for cleaner code
var (
	colorBg        = nord0
	colorBgElevate = nord1
	colorBgActive  = nord2
	colorDim       = nord3
	colorText      = nord4
	colorTextBold  = nord6
	colorAccent    = nord8
	colorAccent2   = nord9
	colorError     = nord11
	colorWarn      = nord12
	colorCaution   = nord13
	colorSuccess   = nord14
	colorSpecial   = nord15
)

// Styles - organized by component
var (
	// ═══════════════════════════════════════════════════════════════════════
	// HEADER
	// ═══════════════════════════════════════════════════════════════════════
	headerStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorBgElevate).
			Padding(0, 1)

	headerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent)

	headerModelStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	headerSessionStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	// ═══════════════════════════════════════════════════════════════════════
	// FOOTER & INPUT
	// ═══════════════════════════════════════════════════════════════════════
	footerStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorBgElevate).
			Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Bold(true)

	// ═══════════════════════════════════════════════════════════════════════
	// STATUS BAR
	// ═══════════════════════════════════════════════════════════════════════
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Background(colorBgElevate)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	statsValueStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	statsLabelStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	statsHintStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	statsSeparatorStyle = lipgloss.NewStyle().
				Foreground(nord2) // Very subtle separator

	// ═══════════════════════════════════════════════════════════════════════
	// USER MESSAGES
	// ═══════════════════════════════════════════════════════════════════════
	userStyle = lipgloss.NewStyle().
			Foreground(colorTextBold)

	userPrefixStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	// ═══════════════════════════════════════════════════════════════════════
	// ASSISTANT MESSAGES
	// ═══════════════════════════════════════════════════════════════════════
	assistantStyle = lipgloss.NewStyle().
			Foreground(colorText)

	thinkingStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Italic(true)

	// ═══════════════════════════════════════════════════════════════════════
	// TOOL CALLS & RESULTS
	// ═══════════════════════════════════════════════════════════════════════
	toolCallStyle = lipgloss.NewStyle().
			Foreground(colorSpecial)

	toolNameStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	toolDescStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	toolResultSuccessStyle = lipgloss.NewStyle().
				Foreground(colorSuccess)

	toolResultErrorStyle = lipgloss.NewStyle().
				Foreground(colorError)

	// ═══════════════════════════════════════════════════════════════════════
	// SYSTEM MESSAGES
	// ═══════════════════════════════════════════════════════════════════════
	errorStyle = lipgloss.NewStyle().
			Foreground(colorError)

	warningStyle = lipgloss.NewStyle().
			Foreground(colorCaution)

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	infoStyle = lipgloss.NewStyle().
			Foreground(colorAccent2)

	// ═══════════════════════════════════════════════════════════════════════
	// PERMISSION PROMPT
	// ═══════════════════════════════════════════════════════════════════════
	permissionPromptStyle = lipgloss.NewStyle().
				Foreground(colorCaution).
				Bold(true)

	permissionLevelReadStyle = lipgloss.NewStyle().
					Foreground(colorAccent2)

	permissionLevelWriteStyle = lipgloss.NewStyle().
					Foreground(colorWarn)

	permissionLevelExecStyle = lipgloss.NewStyle().
					Foreground(colorError)
)

// Icons - minimal, consistent set
const (
	iconToolCall = "◆"  // Diamond for tool calls
	iconSuccess  = "✓"  // Checkmark
	iconError    = "✗"  // X mark
	iconInfo     = "●"  // Bullet for info
	iconWarning  = "!"  // Simple exclamation
	iconUser     = ">"  // Prompt
	iconArrowUp  = "↑"  // Upload/input tokens
	iconArrowDn  = "↓"  // Download/output tokens
)

// Spinner frames (dots pattern - cleaner look)
var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// GetSpinnerFrame returns the current spinner frame
func GetSpinnerFrame(frame int) string {
	return spinnerFrames[frame%len(spinnerFrames)]
}

// truncate truncates a string to maxLen characters, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
