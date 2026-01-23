package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	primaryColor   = lipgloss.Color("39")  // Cyan
	successColor   = lipgloss.Color("82")  // Green
	warningColor   = lipgloss.Color("214") // Orange/Yellow
	errorColor     = lipgloss.Color("196") // Red
	infoColor      = lipgloss.Color("39")  // Blue
	dimColor       = lipgloss.Color("240") // Gray
	userColor      = lipgloss.Color("255") // White
	assistantColor = lipgloss.Color("252") // Light gray
)

// Styles
var (
	// Header styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	headerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor)

	headerModelStyle = lipgloss.NewStyle().
				Foreground(dimColor)

	headerStatusStyle = lipgloss.NewStyle().
				Foreground(successColor)

	// Footer styles
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	// Content styles
	userStyle = lipgloss.NewStyle().
			Foreground(userColor).
			Bold(true)

	userPrefixStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(assistantColor)

	thinkingStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true)

	// Tool styles
	toolCallStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	toolNameStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	toolDescStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	toolResultSuccessStyle = lipgloss.NewStyle().
				Foreground(successColor)

	toolResultErrorStyle = lipgloss.NewStyle().
				Foreground(errorColor)

	toolResultIndentStyle = lipgloss.NewStyle().
				Foreground(dimColor)

	// Message styles
	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(infoColor)

	// Permission styles
	permissionPromptStyle = lipgloss.NewStyle().
				Foreground(dimColor)
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
