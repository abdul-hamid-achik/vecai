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
// Designed for clean, readable output similar to Claude Code
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
				Prefix: "# ",
			},
			Margin: uintPtr(1),
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#88c0d0"),
				Bold:   boolPtr(true),
				Prefix: "## ",
			},
			Margin: uintPtr(1),
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#81a1c1"), // nord9
				Bold:   boolPtr(true),
				Prefix: "### ",
			},
			Margin: uintPtr(1),
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#81a1c1"),
				Bold:   boolPtr(true),
				Prefix: "#### ",
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#8fbcbb"), // nord7
				Bold:   boolPtr(true),
				Prefix: "##### ",
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#8fbcbb"),
				Prefix: "###### ",
			},
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#d8dee9"),
			},
			Margin: uintPtr(1),
		},
		Text: ansi.StylePrimitive{
			Color: stringPtr("#d8dee9"),
		},
		Emph: ansi.StylePrimitive{
			Color:  stringPtr("#eceff4"), // nord6 - make italic text brighter
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
				Margin: uintPtr(1),
			},
			LevelIndent: 3,
		},
		Item: ansi.StylePrimitive{
			Color:       stringPtr("#d8dee9"),
			BlockPrefix: "  ",
		},
		Enumeration: ansi.StylePrimitive{
			Color:       stringPtr("#88c0d0"), // nord8 - accent for numbers
			BlockPrefix: ". ",                 // separator between number and text
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#d8dee9"),
			},
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           stringPtr("#a3be8c"), // nord14 - green for inline code
				BackgroundColor: stringPtr("#3b4252"), // nord1
				Prefix:          " ",
				Suffix:          " ",
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
					Color: stringPtr("#81a1c1"), // nord9
					Bold:  boolPtr(true),
				},
				KeywordReserved: ansi.StylePrimitive{
					Color: stringPtr("#81a1c1"),
					Bold:  boolPtr(true),
				},
				KeywordType: ansi.StylePrimitive{
					Color: stringPtr("#8fbcbb"), // nord7
				},
				Name: ansi.StylePrimitive{
					Color: stringPtr("#d8dee9"),
				},
				NameFunction: ansi.StylePrimitive{
					Color: stringPtr("#88c0d0"), // nord8
				},
				NameClass: ansi.StylePrimitive{
					Color: stringPtr("#8fbcbb"), // nord7
				},
				NameConstant: ansi.StylePrimitive{
					Color: stringPtr("#ebcb8b"), // nord13
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: stringPtr("#88c0d0"),
				},
				LiteralString: ansi.StylePrimitive{
					Color: stringPtr("#a3be8c"), // nord14
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: stringPtr("#b48ead"), // nord15
				},
				Comment: ansi.StylePrimitive{
					Color:  stringPtr("#4c566a"), // nord3
					Italic: boolPtr(true),
				},
				CommentPreproc: ansi.StylePrimitive{
					Color: stringPtr("#5e81ac"), // nord10
				},
				Operator: ansi.StylePrimitive{
					Color: stringPtr("#81a1c1"),
				},
				Punctuation: ansi.StylePrimitive{
					Color: stringPtr("#eceff4"), // nord6
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: stringPtr("#bf616a"), // nord11 - red for deleted
				},
				GenericInserted: ansi.StylePrimitive{
					Color: stringPtr("#a3be8c"), // nord14 - green for inserted
				},
			},
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr("#88c0d0"),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr("#8fbcbb"),
			Bold:  boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  stringPtr("#4c566a"),
			Format: "────────────────────────────────────────────────────────────────────────────────",
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr("#d8dee9"),
				},
			},
			CenterSeparator: stringPtr("┼"),
			ColumnSeparator: stringPtr("│"),
			RowSeparator:    stringPtr("─"),
		},
		DefinitionList: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#d8dee9"),
			},
			Margin: uintPtr(1),
		},
		DefinitionTerm: ansi.StylePrimitive{
			Color: stringPtr("#88c0d0"),
			Bold:  boolPtr(true),
		},
		DefinitionDescription: ansi.StylePrimitive{
			Color:       stringPtr("#d8dee9"),
			BlockPrefix: "  ",
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#81a1c1"), // nord9 - slightly brighter for quotes
				Italic: boolPtr(true),
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr("│ "),
		},
		ImageText: ansi.StylePrimitive{
			Color:  stringPtr("#b48ead"), // nord15
			Format: "[Image: %s]",
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

	// Content viewport (with optional help overlay)
	if m.showHelpOverlay {
		b.WriteString(renderHelpOverlay(m.width, m.viewport.Height))
	} else {
		b.WriteString(m.viewport.View())
	}
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
		sessionPart = headerModelStyle.Render(fmt.Sprintf(" [%s]", m.sessionID))
	}

	// Project context: working dir and git branch
	projectPart := ""
	if m.workingDir != "" {
		projectPart += " " + headerDirStyle.Render(m.workingDir)
	}
	if m.gitBranch != "" {
		projectPart += " " + headerBranchStyle.Render("("+m.gitBranch+")")
	}

	// Model info
	model := headerModelStyle.Render(fmt.Sprintf("Model: %s", m.modelName))

	// Calculate spacing
	leftPart := title + sessionPart + projectPart
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

	// Show completer dropdown above input if active
	if m.completer.IsActive() {
		dropdown := m.completer.Render(m.width)
		if dropdown != "" {
			b.WriteString(dropdown)
			b.WriteString("\n")
		}
	}

	// Mode selector + input prompt (Claude Code style)
	modeSelector := m.renderModeSelector()
	prompt := modeSelector + " "

	// Always show text input - user can type and queue messages
	inputLine := prompt + m.textArea.View()

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

	// Show progress bar if active
	if m.progressInfo != nil && m.progressInfo.Total > 0 {
		parts = append(parts, renderProgressBar(m.progressInfo, 20))
	}

	// Show "new content below" indicator when user has scrolled up
	if m.newContentPending && m.userScrolledUp {
		parts = append(parts, warningStyle.Render("↓ New content (End to jump)"))
	}

	// Show contextual hint based on state
	switch m.state {
	case StateStreaming, StateRateLimited:
		parts = append(parts, statsHintStyle.Render("ESC to stop"))
	case StatePermission:
		parts = append(parts, statsHintStyle.Render("y/n/a/v"))
	default:
		parts = append(parts, statsHintStyle.Render("Shift+Tab: mode  F1: help"))
	}

	// Join with simple spacing (cleaner look)
	content := strings.Join(parts, "   ")

	return statusBarStyle.Width(m.width).Padding(0, 1).Render(content)
}

// formatDuration formats a duration as a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Second {
		ms := d.Milliseconds()
		return fmt.Sprintf("%dms", ms)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
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

// formatByteCount formats a byte count as a human-readable string
func formatByteCount(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

// renderModeSelector renders the Claude Code-style mode pills on the input line
func (m Model) renderModeSelector() string {
	modes := []struct {
		mode        AgentMode
		label       string
		activeStyle lipgloss.Style
	}{
		{ModeAsk, "Ask", modeActiveAskStyle},
		{ModePlan, "Plan", modeActivePlanStyle},
		{ModeBuild, "Build", modeActiveBuildStyle},
	}

	var parts []string
	for _, md := range modes {
		if m.agentMode == md.mode {
			parts = append(parts, md.activeStyle.Render(md.label))
		} else {
			parts = append(parts, modeInactiveStyle.Render(md.label))
		}
	}

	return strings.Join(parts, "") + " " + inputPromptStyle.Render("▸")
}

// renderPermissionFooter renders the enhanced permission prompt panel
func (m Model) renderPermissionFooter() string {
	var b strings.Builder

	// Severity color band based on level
	var levelStyle lipgloss.Style
	var levelLabel string
	switch m.permLevel {
	case "read":
		levelStyle = infoStyle
		levelLabel = "READ"
	case "write":
		levelStyle = warningStyle
		levelLabel = "WRITE"
	case "execute":
		levelStyle = errorStyle
		levelLabel = "EXEC"
	default:
		levelStyle = infoStyle
		levelLabel = strings.ToUpper(m.permLevel)
	}

	// Line 1: severity badge + tool name
	badge := levelStyle.Bold(true).Render(" " + levelLabel + " ")
	toolName := permissionPromptStyle.Render(" " + m.permToolName)
	b.WriteString(badge + toolName)
	b.WriteString("\n")

	// Line 2: description (truncated or full)
	desc := m.permDescription
	if !m.permDetailsExpanded && len(desc) > 60 {
		desc = desc[:57] + "..."
	}
	b.WriteString("  " + lipgloss.NewStyle().Foreground(colorText).Render(desc))
	b.WriteString("\n")

	// Line 3: expanded details if toggled
	if m.permDetailsExpanded && m.permFullContent != "" && len(m.permFullContent) > 60 {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(colorDim).Render(m.permFullContent))
		b.WriteString("\n")
	}

	// Line 3/4: key hints
	hints := permKeyStyle.Render("[y]") + statsHintStyle.Render("es  ") +
		permKeyStyle.Render("[n]") + statsHintStyle.Render("o  ") +
		permKeyStyle.Render("[a]") + statsHintStyle.Render("lways  ") +
		permKeyStyle.Render("[v]") + statsHintStyle.Render("eto  ") +
		permKeyStyle.Render("[d]") + statsHintStyle.Render("etails")
	b.WriteString("  " + hints)

	return permissionPanelStyle.Width(m.width).Render(b.String())
}

// renderStartupView renders the loading/startup view
func (m Model) renderStartupView() string {
	frame := GetSpinnerFrame(m.spinnerFrame)
	return fmt.Sprintf("\n\n  %s Starting vecai...\n\n", frame)
}

// renderContent renders all content blocks
func (m Model) renderContent() string {
	var b strings.Builder

	for i, block := range *m.blocks {
		// Use cached render if available
		if block.RenderedCache != "" {
			b.WriteString(block.RenderedCache)
			b.WriteString("\n")
			continue
		}

		rendered := m.renderBlock(block)

		// Cache rendered output for completed, non-running blocks
		isRunningTool := block.ToolMeta != nil && block.ToolMeta.IsRunning
		if !isRunningTool {
			(*m.blocks)[i].RenderedCache = rendered
		}

		b.WriteString(rendered)
		b.WriteString("\n")
	}

	// Add streaming content if any (debounced markdown rendering)
	if m.streaming.Len() > 0 {
		streamStr := m.streaming.String()
		if m.renderedCache != "" {
			// Show the cached Glamour render
			b.WriteString(m.renderedCache)
			// Append raw text tail (content added since last render)
			if m.lastRenderedLen < len(streamStr) {
				tail := streamStr[m.lastRenderedLen:]
				b.WriteString(tail)
			}
		} else {
			// No cache yet, show raw text
			b.WriteString(streamStr)
		}
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
	case BlockPlan:
		return renderMarkdown(block.Content)
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
	if block.Collapsed {
		return collapsedHintStyle.Render(fmt.Sprintf("... Thinking (%s)  [Ctrl+T to expand]", block.Summary))
	}
	return thinkingStyle.Render(block.Content)
}

// renderToolCallBlock renders a tool call notification with category icon
func (m Model) renderToolCallBlock(block ContentBlock) string {
	// Determine icon and style based on tool category
	icon := iconToolCall
	var iconSty lipgloss.Style
	categoryLabel := ""

	if block.ToolMeta != nil {
		switch block.ToolMeta.ToolType {
		case ToolCategoryRead:
			icon = iconToolRead
			iconSty = toolCategoryReadStyle
			categoryLabel = toolCategoryReadStyle.Render("[READ]  ")
		case ToolCategoryWrite:
			icon = iconToolWrite
			iconSty = toolCategoryWriteStyle
			categoryLabel = toolCategoryWriteStyle.Render("[WRITE] ")
		case ToolCategoryExecute:
			icon = iconToolExec
			iconSty = toolCategoryExecStyle
			categoryLabel = toolCategoryExecStyle.Render("[EXEC]  ")
		default:
			iconSty = toolCallStyle
		}
		// Show spinner if tool is still running
		if block.ToolMeta.IsRunning {
			icon = GetSpinnerFrame(m.spinnerFrame)
			iconSty = spinnerStyle
		}
	} else {
		iconSty = toolCallStyle
	}

	rendered := iconSty.Render(icon) + " " + categoryLabel + toolNameStyle.Render(block.ToolName)
	if block.Content != "" {
		rendered += toolDescStyle.Render("  " + block.Content)
	}
	return rendered
}

// renderToolResultBlock renders a tool execution result with elapsed time
func (m Model) renderToolResultBlock(block ContentBlock) string {
	// Collapsed view for non-error results
	if block.Collapsed && !block.IsError {
		icon := toolResultSuccessStyle.Render(iconSuccess + " ")
		name := toolResultSuccessStyle.Render(block.ToolName)
		elapsed := ""
		if block.ToolMeta != nil && block.ToolMeta.Elapsed > 0 {
			elapsed = " " + toolElapsedStyle.Render("("+formatDuration(block.ToolMeta.Elapsed)+")")
		}
		summary := ""
		if block.Summary != "" {
			summary = " " + collapsedHintStyle.Render("— "+block.Summary+"  [Ctrl+T to expand]")
		}
		return icon + name + elapsed + summary
	}

	var b strings.Builder

	if block.IsError {
		icon := toolResultErrorStyle.Render(iconError + " ")
		name := toolResultErrorStyle.Render(block.ToolName + ": ")
		b.WriteString(icon + name + block.Content)
	} else {
		icon := toolResultSuccessStyle.Render(iconSuccess + " ")
		name := toolResultSuccessStyle.Render(block.ToolName)
		b.WriteString(icon + name)

		// Show elapsed time if available
		if block.ToolMeta != nil && block.ToolMeta.Elapsed > 0 {
			b.WriteString(" " + toolElapsedStyle.Render("("+formatDuration(block.ToolMeta.Elapsed)+")"))
		}

		// Render result if present
		if block.Content != "" && block.Content != "(no output)" {
			b.WriteString("\n")
			// Use diff rendering if content looks like a unified diff
			var rendered string
			if isDiffContent(block.Content) {
				rendered = renderDiff(block.Content)
			} else {
				rendered = renderMarkdown(block.Content)
			}
			// Add 2-space indent to each line for visual grouping
			for i, line := range strings.Split(rendered, "\n") {
				if i > 0 {
					b.WriteString("\n")
				}
				b.WriteString("  " + line)
			}

			// Show truncation indicator if result was truncated
			if block.ToolMeta != nil && block.ToolMeta.ResultLen > 500 {
				b.WriteString("\n")
				b.WriteString("  " + toolTruncIndicatorStyle.Render(
					fmt.Sprintf("▸ %s total (showing first 500)", formatByteCount(block.ToolMeta.ResultLen))))
			}
		}
	}

	return b.String()
}

// renderProgressBar renders a mini progress bar for known-length operations
func renderProgressBar(info *ProgressInfo, barWidth int) string {
	if info.Total <= 0 {
		return ""
	}
	pct := float64(info.Current) / float64(info.Total)
	if pct > 1.0 {
		pct = 1.0
	}
	filled := int(pct * float64(barWidth))
	empty := barWidth - filled

	bar := progressFilledStyle.Render(strings.Repeat("█", filled)) +
		progressEmptyStyle.Render(strings.Repeat("░", empty))
	label := fmt.Sprintf(" %d/%d", info.Current, info.Total)
	desc := ""
	if info.Description != "" {
		desc = " " + info.Description
	}
	return bar + statsLabelStyle.Render(label) + statsHintStyle.Render(desc)
}

// isDiffContent checks if content looks like a unified diff
func isDiffContent(content string) bool {
	return strings.Contains(content, "\n--- ") && strings.Contains(content, "\n+++ ") &&
		strings.Contains(content, "\n@@ ")
}

// renderDiff renders a unified diff with color-coded lines
func renderDiff(content string) string {
	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			b.WriteString(diffHeaderStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			b.WriteString(diffHunkStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			b.WriteString(diffAddStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			b.WriteString(diffDelStyle.Render(line))
		case strings.HasSuffix(line, "insertion(s)") || strings.HasSuffix(line, "deletion(s)"):
			b.WriteString(diffSummaryStyle.Render(line))
		default:
			b.WriteString(diffContextStyle.Render(line))
		}
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
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
