package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
)

// Markdown renderer with Nord-compatible dark style
var mdRenderer *glamour.TermRenderer

func init() {
	// Create a Nord-themed renderer
	nordStyle := getNordGlamourStyle()
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(nordStyle),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		// Fallback: no markdown rendering
		mdRenderer = nil
	} else {
		mdRenderer = r
	}
}

// getNordGlamourStyle returns a glamour style matching the Nord theme
func getNordGlamourStyle() ansi.StyleConfig {
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#d8dee9"), // nord4 - primary text
			},
			Margin: uintPtr(0),
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#88c0d0"), // nord8 - accent
				Bold:  boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#88c0d0"),
				Bold:   boolPtr(true),
				Prefix: "",
			},
			Margin: uintPtr(1),
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#81a1c1"), // nord9
				Bold:   boolPtr(true),
				Prefix: "",
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#81a1c1"),
				Bold:   boolPtr(true),
				Prefix: "",
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#8fbcbb"), // nord7
				Bold:  boolPtr(true),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#8fbcbb"),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#4c566a"), // nord3
			},
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#d8dee9"),
			},
		},
		Text: ansi.StylePrimitive{
			Color: stringPtr("#d8dee9"),
		},
		Emph: ansi.StylePrimitive{
			Color:  stringPtr("#d8dee9"),
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Color: stringPtr("#eceff4"), // nord6 - bright
			Bold:  boolPtr(true),
		},
		Strikethrough: ansi.StylePrimitive{
			Color: stringPtr("#4c566a"),
		},
		List: ansi.StyleList{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr("#d8dee9"),
				},
			},
			LevelIndent: 2,
		},
		Item: ansi.StylePrimitive{
			Color:       stringPtr("#d8dee9"),
			BlockPrefix: "  ", // Indent list items
		},
		Enumeration: ansi.StylePrimitive{
			Color:  stringPtr("#88c0d0"),
			Format: "%d. ", // Explicit format: number + period + space
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           stringPtr("#a3be8c"), // nord14 - green for inline code
				BackgroundColor: stringPtr("#3b4252"), // nord1
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr("#d8dee9"),
				},
				Margin: uintPtr(1),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: stringPtr("#d8dee9"),
				},
				Keyword: ansi.StylePrimitive{
					Color: stringPtr("#81a1c1"),
				},
				Name: ansi.StylePrimitive{
					Color: stringPtr("#88c0d0"),
				},
				NameFunction: ansi.StylePrimitive{
					Color: stringPtr("#88c0d0"),
				},
				LiteralString: ansi.StylePrimitive{
					Color: stringPtr("#a3be8c"),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: stringPtr("#b48ead"),
				},
				Comment: ansi.StylePrimitive{
					Color: stringPtr("#4c566a"),
				},
				Operator: ansi.StylePrimitive{
					Color: stringPtr("#81a1c1"),
				},
			},
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr("#88c0d0"),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr("#8fbcbb"),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  stringPtr("#4c566a"),
			Format: "─────",
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr("#d8dee9"),
				},
			},
		},
		DefinitionList: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#d8dee9"),
			},
		},
		DefinitionTerm: ansi.StylePrimitive{
			Color: stringPtr("#88c0d0"),
			Bold:  boolPtr(true),
		},
		DefinitionDescription: ansi.StylePrimitive{
			Color: stringPtr("#d8dee9"),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#4c566a"),
				Italic: boolPtr(true),
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr("│ "),
		},
	}
}

// Helper functions for glamour style config
func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func uintPtr(u uint) *uint       { return &u }

// renderMarkdown renders markdown text, falling back to plain text on error
func renderMarkdown(content string) string {
	if mdRenderer == nil {
		return content
	}

	rendered, err := mdRenderer.Render(content)
	if err != nil {
		return content
	}

	// Trim trailing newlines that glamour adds
	return strings.TrimSuffix(rendered, "\n")
}

// View renders the entire TUI
func (m Model) View() string {
	if !m.ready {
		return m.renderStartupView()
	}

	if m.quitting {
		return "Goodbye!\n"
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Content viewport
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Footer
	b.WriteString(m.renderFooter())

	return b.String()
}

// renderHeader renders the header bar
func (m Model) renderHeader() string {
	// Title
	title := headerTitleStyle.Render("vecai")

	// Session ID (if available)
	sessionPart := ""
	if m.sessionID != "" {
		// Show short session ID in subtle style
		sessionPart = headerModelStyle.Render(fmt.Sprintf(" [%s]", m.sessionID))
	}

	// Model info
	model := headerModelStyle.Render(fmt.Sprintf("Model: %s", m.modelName))

	// Calculate spacing - header shows title, session, and model
	leftPart := title + sessionPart
	rightPart := model

	availWidth := m.width - lipgloss.Width(leftPart) - lipgloss.Width(rightPart) - 4
	if availWidth < 0 {
		availWidth = 0
	}
	spaces := strings.Repeat(" ", availWidth)

	header := headerStyle.Width(m.width).Render(leftPart + spaces + rightPart)
	return header
}

// renderFooter renders the footer bar with input or permission prompt
func (m Model) renderFooter() string {
	if m.state == StatePermission {
		return m.renderPermissionFooter()
	}

	var b strings.Builder

	// Always show status bar (like Claude Code)
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")

	// Input prompt
	prompt := inputPromptStyle.Render("> ")

	// Always show text input - user can type and queue messages
	inputLine := prompt + m.textInput.View()

	b.WriteString(footerStyle.Width(m.width).Render(inputLine))
	return b.String()
}

// renderStatusBar renders the status bar with session stats
func (m Model) renderStatusBar() string {
	var parts []string

	// Permission state takes priority
	if m.state == StatePermission {
		parts = append(parts, warningStyle.Render(iconWarning+" Permission Required"))
	} else if m.rateLimitInfo != nil {
		// Rate limit info
		remaining := time.Until(m.rateLimitEndTime)
		if remaining > 0 {
			rateLimitStr := fmt.Sprintf("Rate limited: %s", formatDuration(remaining))
			if m.rateLimitInfo.Reason != "" {
				rateLimitStr = fmt.Sprintf("%s: %s", m.rateLimitInfo.Reason, formatDuration(remaining))
			}
			if m.rateLimitInfo.Attempt > 0 {
				rateLimitStr += fmt.Sprintf(" (%d/%d)", m.rateLimitInfo.Attempt, m.rateLimitInfo.MaxAttempts)
			}
			parts = append(parts, warningStyle.Render(rateLimitStr))
		}
	} else if m.activityMessage != "" {
		// Show spinner + activity during active processing states
		if m.state == StateStreaming || m.state == StateRateLimited {
			frame := GetSpinnerFrame(m.spinnerFrame)
			parts = append(parts, spinnerStyle.Render(frame+" "+m.activityMessage))
		} else {
			parts = append(parts, statsValueStyle.Render(m.activityMessage))
		}
	}

	// Duration - how long this loop has been running (only during active processing)
	if !m.loopStartTime.IsZero() && m.rateLimitInfo == nil && (m.state == StateStreaming || m.state == StateRateLimited) {
		elapsed := time.Since(m.loopStartTime)
		parts = append(parts, statsLabelStyle.Render(formatDuration(elapsed)))
	}

	// Token usage - show accumulated tokens (always show if we have any)
	if m.inputTokens > 0 || m.outputTokens > 0 {
		tokenStr := fmt.Sprintf("%s%s  %s%s",
			statsValueStyle.Render(iconArrowUp),
			statsLabelStyle.Render(formatTokenCount(m.inputTokens)),
			statsValueStyle.Render(iconArrowDn),
			statsLabelStyle.Render(formatTokenCount(m.outputTokens)))
		parts = append(parts, tokenStr)
	}

	// Context usage - show if tracked (color-coded by threshold)
	if m.contextUsage > 0 {
		contextStr := fmt.Sprintf("Ctx: %.0f%%", m.contextUsage*100)
		var style lipgloss.Style
		switch {
		case m.contextUsage >= 0.95:
			style = errorStyle // Red - needs compaction
		case m.contextUsage >= 0.80:
			style = warningStyle // Yellow - warning
		default:
			style = statsLabelStyle // Normal - subtle
		}
		parts = append(parts, style.Render(contextStr))
	}

	// Iteration count (only during active processing)
	if m.loopIteration > 0 && m.rateLimitInfo == nil && (m.state == StateStreaming || m.state == StateRateLimited) {
		iterStr := fmt.Sprintf("[%d/%d]", m.loopIteration, m.maxIterations)
		parts = append(parts, statsLabelStyle.Render(iterStr))
	}

	// Queue count - show if items queued
	if queueLen := len(m.inputQueue); queueLen > 0 {
		parts = append(parts, warningStyle.Render(fmt.Sprintf("+%d queued", queueLen)))
	}

	// Show contextual hint based on state
	switch m.state {
	case StateStreaming, StateRateLimited:
		parts = append(parts, statsHintStyle.Render("ESC to stop"))
	case StatePermission:
		parts = append(parts, statsHintStyle.Render("y/n/a/v"))
	default:
		parts = append(parts, statsHintStyle.Render("/help"))
	}

	// Join with simple spacing (cleaner look)
	content := strings.Join(parts, "   ")

	return statusBarStyle.Width(m.width).Padding(0, 1).Render(content)
}

// formatDuration formats a duration as a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes >= 60 {
		hours := minutes / 60
		minutes = minutes % 60
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	}
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// formatTokenCount formats a token count with K suffix for thousands
func formatTokenCount(count int64) string {
	if count < 1000 {
		return fmt.Sprintf("%d", count)
	}
	return fmt.Sprintf("%.1fk", float64(count)/1000)
}

// renderPermissionFooter renders the permission prompt in the footer
func (m Model) renderPermissionFooter() string {
	// Show permission level with color coding
	levelStr := ""
	switch m.permLevel {
	case "read":
		levelStr = infoStyle.Render("[READ]")
	case "write":
		levelStr = warningStyle.Render("[WRITE]")
	case "execute":
		levelStr = errorStyle.Render("[EXEC]")
	default:
		levelStr = infoStyle.Render("[" + m.permLevel + "]")
	}

	prompt := permissionPromptStyle.Render(
		fmt.Sprintf("%s %s %s - %s",
			iconWarning, levelStr, m.permToolName, m.permDescription))

	// Key hints on same line
	hints := statsHintStyle.Render(" [y]es/[n]o/[a]lways/ne[v]er")

	return footerStyle.Width(m.width).Render(prompt + hints)
}

// renderStartupView renders the loading/startup view
func (m Model) renderStartupView() string {
	frame := GetSpinnerFrame(m.spinnerFrame)
	return fmt.Sprintf("\n\n  %s Starting vecai...\n\n", frame)
}

// renderContent renders all content blocks
func (m Model) renderContent() string {
	var b strings.Builder

	for _, block := range *m.blocks {
		b.WriteString(m.renderBlock(block))
		b.WriteString("\n")
	}

	// Add streaming content if any (render markdown for consistent formatting)
	if m.streaming.Len() > 0 {
		b.WriteString(renderMarkdown(m.streaming.String()))
	}

	return b.String()
}

// renderBlock renders a single content block
func (m Model) renderBlock(block ContentBlock) string {
	switch block.Type {
	case BlockUser:
		return m.renderUserBlock(block)
	case BlockAssistant:
		return m.renderAssistantBlock(block)
	case BlockThinking:
		return m.renderThinkingBlock(block)
	case BlockToolCall:
		return m.renderToolCallBlock(block)
	case BlockToolResult:
		return m.renderToolResultBlock(block)
	case BlockError:
		return m.renderErrorBlock(block)
	case BlockInfo:
		return m.renderInfoBlock(block)
	case BlockWarning:
		return m.renderWarningBlock(block)
	case BlockSuccess:
		return m.renderSuccessBlock(block)
	default:
		return block.Content
	}
}

// renderUserBlock renders a user message
func (m Model) renderUserBlock(block ContentBlock) string {
	prefix := userPrefixStyle.Render(iconUser + " ")
	content := userStyle.Render(block.Content)
	return prefix + content
}

// renderAssistantBlock renders an assistant message with markdown
func (m Model) renderAssistantBlock(block ContentBlock) string {
	return renderMarkdown(block.Content)
}

// renderThinkingBlock renders thinking text
func (m Model) renderThinkingBlock(block ContentBlock) string {
	return thinkingStyle.Render(block.Content)
}

// renderToolCallBlock renders a tool call notification
func (m Model) renderToolCallBlock(block ContentBlock) string {
	icon := toolCallStyle.Render(iconToolCall)
	name := toolNameStyle.Render(" " + block.ToolName)
	desc := ""
	if block.Content != "" {
		desc = toolDescStyle.Render(" " + block.Content)
	}
	return icon + name + desc
}

// renderToolResultBlock renders a tool execution result
func (m Model) renderToolResultBlock(block ContentBlock) string {
	var b strings.Builder

	if block.IsError {
		icon := toolResultErrorStyle.Render(iconError + " ")
		name := toolResultErrorStyle.Render(block.ToolName + ": ")
		b.WriteString(icon + name + block.Content)
	} else {
		icon := toolResultSuccessStyle.Render(iconSuccess + " ")
		name := toolResultSuccessStyle.Render(block.ToolName)
		b.WriteString(icon + name)

		// Render result if present
		if block.Content != "" && block.Content != "(no output)" {
			b.WriteString("\n")
			// Render markdown for rich tool output (vecgrep, etc.)
			rendered := renderMarkdown(block.Content)
			b.WriteString(rendered)
		}
	}

	return b.String()
}

// renderErrorBlock renders an error message
func (m Model) renderErrorBlock(block ContentBlock) string {
	return errorStyle.Render(iconError + " " + block.Content)
}

// renderInfoBlock renders an info message
func (m Model) renderInfoBlock(block ContentBlock) string {
	return infoStyle.Render(iconInfo) + " " + block.Content
}

// renderWarningBlock renders a warning message
func (m Model) renderWarningBlock(block ContentBlock) string {
	return warningStyle.Render(iconWarning + " " + block.Content)
}

// renderSuccessBlock renders a success message
func (m Model) renderSuccessBlock(block ContentBlock) string {
	icon := successStyle.Render(iconSuccess + " ")
	return icon + block.Content
}
