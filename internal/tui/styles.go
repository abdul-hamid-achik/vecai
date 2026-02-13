package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Nord Theme Colors
// https://www.nordtheme.com/docs/colors-and-palettes
var (
	// Polar Night - dark backgrounds
	nord0 = lipgloss.Color("#2e3440") // Darkest background
	nord1 = lipgloss.Color("#3b4252") // Elevated surfaces
	nord3 = lipgloss.Color("#4c566a") // Comments/subtle

	// Snow Storm - text
	nord4 = lipgloss.Color("#d8dee9") // Primary text
	nord6 = lipgloss.Color("#eceff4") // Bright text

	// Frost - accents
	nord8 = lipgloss.Color("#88c0d0") // Primary accent (cyan)
	nord9 = lipgloss.Color("#81a1c1") // Secondary accent (blue)

	// Aurora - semantic
	nord11 = lipgloss.Color("#bf616a") // Error (red)
	nord13 = lipgloss.Color("#ebcb8b") // Caution (yellow)
	nord14 = lipgloss.Color("#a3be8c") // Success (green)
	nord15 = lipgloss.Color("#b48ead") // Special (purple)
)

// Semantic color aliases for cleaner code
var (
	colorBgElevate = nord1
	colorDim       = nord3  // Decorative only (borders, separators)
	colorMuted     = nord4  // Readable subdued text (Snow Storm lightest)
	colorText      = nord4
	colorTextBold  = nord6
	colorAccent    = nord8
	colorAccent2   = nord9
	colorError     = nord11
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
				Foreground(colorMuted)

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
			Foreground(colorMuted).
			Background(colorBgElevate)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	statsValueStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	statsLabelStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	statsHintStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// ═══════════════════════════════════════════════════════════════════════
	// USER MESSAGES
	// ═══════════════════════════════════════════════════════════════════════
	userStyle = lipgloss.NewStyle().
			Foreground(colorTextBold)

	userPrefixStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	// ═══════════════════════════════════════════════════════════════════════
	// ASSISTANT MESSAGES (rendered via glamour markdown)
	// ═══════════════════════════════════════════════════════════════════════
	thinkingStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
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
			Foreground(colorMuted)

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

	permissionPanelStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorBgElevate).
				Padding(0, 1)

	permKeyStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	// ═══════════════════════════════════════════════════════════════════════
	// MODE SELECTOR (input line pills — Claude Code style)
	// ═══════════════════════════════════════════════════════════════════════
	modeActiveAskStyle = lipgloss.NewStyle().
				Background(nord8).
				Foreground(nord0).
				Bold(true).
				Padding(0, 2)

	modeActivePlanStyle = lipgloss.NewStyle().
				Background(nord13).
				Foreground(nord0).
				Bold(true).
				Padding(0, 2)

	modeActiveBuildStyle = lipgloss.NewStyle().
				Background(nord14).
				Foreground(nord0).
				Bold(true).
				Padding(0, 2)

	// ═══════════════════════════════════════════════════════════════════════
	// TOOL VISUALIZATION
	// ═══════════════════════════════════════════════════════════════════════
	toolCategoryReadStyle = lipgloss.NewStyle().
				Foreground(colorAccent2)

	toolCategoryWriteStyle = lipgloss.NewStyle().
				Foreground(colorCaution)

	toolCategoryExecStyle = lipgloss.NewStyle().
				Foreground(colorError)

	toolElapsedStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Italic(true)

	toolTruncIndicatorStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	// Collapsed block hint
	collapsedHintStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Italic(true)

	// ═══════════════════════════════════════════════════════════════════════
	// HEADER — PROJECT CONTEXT
	// ═══════════════════════════════════════════════════════════════════════
	headerDirStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	headerBranchStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Italic(true)

	// ═══════════════════════════════════════════════════════════════════════
	// DIFF VISUALIZATION
	// ═══════════════════════════════════════════════════════════════════════
	diffAddStyle = lipgloss.NewStyle().
			Foreground(colorSuccess) // nord14 green

	diffDelStyle = lipgloss.NewStyle().
			Foreground(colorError) // nord11 red

	diffHunkStyle = lipgloss.NewStyle().
			Foreground(colorAccent) // nord8 cyan

	diffHeaderStyle = lipgloss.NewStyle().
			Foreground(colorAccent2). // nord9
			Bold(true)

	diffContextStyle = lipgloss.NewStyle().
				Foreground(colorText) // nord4

	diffSummaryStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Italic(true)

	// ═══════════════════════════════════════════════════════════════════════
	// PROGRESS BAR
	// ═══════════════════════════════════════════════════════════════════════
	progressFilledStyle = lipgloss.NewStyle().
				Foreground(colorSuccess)

	progressEmptyStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	// ═══════════════════════════════════════════════════════════════════════
	// BORDERS
	// ═══════════════════════════════════════════════════════════════════════
	toolResultBorderStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	// ═══════════════════════════════════════════════════════════════════════
	// WELCOME SCREEN
	// ═══════════════════════════════════════════════════════════════════════
	welcomeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDim).
			Padding(1, 3)

	welcomeTitleStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	welcomeSubtitleStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	welcomeKeyStyle = lipgloss.NewStyle().
			Foreground(colorTextBold).
			Bold(true).
			Width(14)

	welcomeDescStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	welcomeValueStyle = lipgloss.NewStyle().
				Foreground(colorAccent2)

	// ═══════════════════════════════════════════════════════════════════════
	// TOOL CATEGORY PILLS (colored background)
	// ═══════════════════════════════════════════════════════════════════════
	toolCategoryReadPillStyle = lipgloss.NewStyle().
					Foreground(nord0).
					Background(colorAccent2).
					Bold(true).
					Padding(0, 1)

	toolCategoryWritePillStyle = lipgloss.NewStyle().
					Foreground(nord0).
					Background(colorCaution).
					Bold(true).
					Padding(0, 1)

	toolCategoryExecPillStyle = lipgloss.NewStyle().
					Foreground(nord0).
					Background(colorError).
					Bold(true).
					Padding(0, 1)
)

// Icons - minimal, consistent set
const (
	iconToolCall = "◆" // Diamond for tool calls
	iconSuccess  = "✓" // Checkmark
	iconError    = "✗" // X mark
	iconInfo     = "●" // Bullet for info
	iconWarning  = "!" // Simple exclamation
	iconUser     = ">" // Prompt
	iconArrowUp  = "↑" // Upload/input tokens
	iconArrowDn  = "↓" // Download/output tokens

	// Tool category icons
	iconToolRead  = "◇" // Open diamond for reads
	iconToolWrite = "◆" // Filled diamond for writes
	iconToolExec  = "▶" // Play triangle for execution
)

// Spinner frames (dots pattern - cleaner look)
var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// GetSpinnerFrame returns the current spinner frame
func GetSpinnerFrame(frame int) string {
	return spinnerFrames[frame%len(spinnerFrames)]
}

// truncate truncates a string to approximately maxLen runes, adding "..." if truncated.
// Uses rune-based counting for UTF-8 safety.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// truncateUTF8Safe truncates a string to approximately maxBytes while
// ensuring we don't split a multi-byte UTF-8 character.
func truncateUTF8Safe(s string, maxBytes int) string {
	ellipsis := "..."
	ellipsisLen := len(ellipsis)
	
	if len(s) <= maxBytes {
		return s
	}
	
	// If maxBytes is too small to fit ellipsis, just return empty or partial ellipsis
	if maxBytes <= ellipsisLen {
		if maxBytes <= 0 {
			return ""
		}
		return ellipsis[:maxBytes]
	}
	
	// Reserve space for ellipsis so final result doesn't exceed maxBytes
	targetBytes := maxBytes - ellipsisLen
	
	// Walk backwards from the cut point to find a valid UTF-8 boundary
	for targetBytes > 0 && targetBytes < len(s) {
		// Check if cutting here produces valid UTF-8
		truncated := s[:targetBytes]
		// Verify last rune is complete by checking that the byte count
		// of the rune-decoded string matches
		runes := []rune(truncated)
		if string(runes) == truncated {
			return truncated + ellipsis
		}
		targetBytes--
	}
	
	// Fallback: ensure we don't exceed maxBytes
	if targetBytes <= 0 {
		return ellipsis
	}
	return s[:targetBytes] + ellipsis
}
