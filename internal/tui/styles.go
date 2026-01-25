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
	nord7  = lipgloss.Color("#8fbcbb") // Calm accent
	nord8  = lipgloss.Color("#88c0d0") // Primary accent
	nord9  = lipgloss.Color("#81a1c1") // Secondary accent
	nord10 = lipgloss.Color("#5e81ac") // Tertiary accent

	// Aurora - semantic
	nord11 = lipgloss.Color("#bf616a") // Error
	nord12 = lipgloss.Color("#d08770") // Warning
	nord13 = lipgloss.Color("#ebcb8b") // Caution
	nord14 = lipgloss.Color("#a3be8c") // Success
	nord15 = lipgloss.Color("#b48ead") // Special
)

// Semantic color aliases (unused variables prefixed with _ for palette completeness)
var (
	_ = nord0  // Base background - available for custom styling
	_ = nord2  // Selection/active - available for custom styling
	_ = nord5  // Secondary text - available for custom styling
	_ = nord7  // Calm accent - available for custom styling
	_ = nord10 // Tertiary accent - available for custom styling
	_ = nord15 // Special - available for custom styling
)

// Styles
var (
	// Header styles - elevated surface (nord1)
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(nord6).
			Background(nord1).
			Padding(0, 1)

	headerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(nord8) // Primary accent

	headerModelStyle = lipgloss.NewStyle().
				Foreground(nord3) // Subtle

	// Footer styles - elevated surface
	footerStyle = lipgloss.NewStyle().
			Foreground(nord4).
			Background(nord1).
			Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(nord14). // Success green
				Bold(true)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(nord8) // Primary accent

	// Content styles
	userStyle = lipgloss.NewStyle().
			Foreground(nord6). // Bright text
			Bold(true)

	userPrefixStyle = lipgloss.NewStyle().
			Foreground(nord14). // Success green
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(nord4) // Primary text

	thinkingStyle = lipgloss.NewStyle().
			Foreground(nord3). // Dim/comments
			Italic(true)

	// Tool styles
	toolCallStyle = lipgloss.NewStyle().
			Foreground(nord8). // Primary accent
			Bold(true)

	toolNameStyle = lipgloss.NewStyle().
			Foreground(nord8) // Primary accent

	toolDescStyle = lipgloss.NewStyle().
			Foreground(nord3) // Subtle

	toolResultSuccessStyle = lipgloss.NewStyle().
				Foreground(nord14) // Success

	toolResultErrorStyle = lipgloss.NewStyle().
				Foreground(nord11) // Error

	toolResultIndentStyle = lipgloss.NewStyle().
				Foreground(nord3) // Subtle

	// Message styles
	errorStyle = lipgloss.NewStyle().
			Foreground(nord11). // Error red
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(nord13). // Warning yellow
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(nord14). // Success green
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(nord9) // Info blue

	// Permission styles - VISIBLE (warning yellow, bold)
	permissionPromptStyle = lipgloss.NewStyle().
				Foreground(nord13). // Warning yellow - VISIBLE!
				Bold(true)

	// Status bar styles - slightly elevated (nord1 bg)
	statusBarStyle = lipgloss.NewStyle().
			Foreground(nord3).
			Background(nord1)

	statsValueStyle = lipgloss.NewStyle().
			Foreground(nord8) // Primary accent

	statsLabelStyle = lipgloss.NewStyle().
			Foreground(nord3) // Subtle

	statsHintStyle = lipgloss.NewStyle().
			Foreground(nord12). // Orange - attention
			Italic(true)
)

// Icons
const (
	iconToolCall = "⚡"
	iconSuccess  = "✓"
	iconError    = "✗"
	iconInfo     = "ℹ"
	iconWarning  = "⚠"
	iconUser     = ">"
	iconIndent   = "│"
)

// Spinner frames (braille pattern)
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

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
