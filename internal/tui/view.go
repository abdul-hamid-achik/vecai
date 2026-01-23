package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// View renders the entire TUI
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
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

	// Model info
	model := headerModelStyle.Render(fmt.Sprintf("Model: %s", m.modelName))

	// Status
	var status string
	switch m.state {
	case StateIdle:
		status = headerStatusStyle.Render("Ready")
	case StateStreaming:
		frame := GetSpinnerFrame(m.spinnerFrame)
		msg := m.activityMessage
		if msg == "" {
			msg = "Processing..."
		}
		status = spinnerStyle.Render(frame + " " + msg)
	case StatePermission:
		status = warningStyle.Render("Permission Required")
	}

	// Calculate spacing
	leftPart := title
	rightPart := model + " | " + status

	// Build header with proper width
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

	// Add status bar if streaming
	if m.state == StateStreaming {
		b.WriteString(m.renderStatusBar())
		b.WriteString("\n")
	}

	// Input prompt
	prompt := inputPromptStyle.Render("> ")

	// Build input line (input disabled during streaming)
	var inputLine string
	if m.state == StateStreaming {
		inputLine = prompt + footerStyle.Foreground(dimColor).Render("[input disabled during processing]")
	} else {
		inputLine = prompt + m.textInput.View()
	}

	b.WriteString(footerStyle.Width(m.width).Render(inputLine))
	return b.String()
}

// renderStatusBar renders the status bar with session stats
func (m Model) renderStatusBar() string {
	var parts []string

	// Model name
	if m.modelName != "" {
		parts = append(parts, statsLabelStyle.Render("Model: ")+statsValueStyle.Render(m.modelName))
	}

	// Current activity (what's happening)
	if m.activityMessage != "" {
		parts = append(parts, statsValueStyle.Render(m.activityMessage))
	}

	// Duration - how long this loop has been running
	if !m.loopStartTime.IsZero() {
		elapsed := time.Since(m.loopStartTime)
		durationStr := formatDuration(elapsed)
		parts = append(parts, statsValueStyle.Render(durationStr))
	}

	// Token usage - show accumulated tokens
	if m.inputTokens > 0 || m.outputTokens > 0 {
		tokenStr := fmt.Sprintf("⬆%s ⬇%s",
			formatTokenCount(m.inputTokens),
			formatTokenCount(m.outputTokens))
		parts = append(parts, statsValueStyle.Render(tokenStr))
	}

	// Iteration count
	if m.loopIteration > 0 {
		iterStr := fmt.Sprintf("[%d/%d]", m.loopIteration, m.maxIterations)
		parts = append(parts, statsValueStyle.Render(iterStr))
	}

	// ESC hint
	parts = append(parts, statsHintStyle.Render("ESC to stop"))

	// Join with separators
	content := strings.Join(parts, statsLabelStyle.Render(" │ "))

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
	prompt := permissionPromptStyle.Render(
		fmt.Sprintf("%s %s - %s [y]es / [n]o / [a]lways / ne[v]er: ",
			iconWarning, m.permToolName, m.permDescription))
	return footerStyle.Width(m.width).Render(prompt)
}

// renderContent renders all content blocks
func (m Model) renderContent() string {
	var b strings.Builder

	for _, block := range m.blocks {
		b.WriteString(m.renderBlock(block))
		b.WriteString("\n")
	}

	// Add streaming content if any
	if m.streaming.Len() > 0 {
		b.WriteString(assistantStyle.Render(m.streaming.String()))
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

// renderAssistantBlock renders an assistant message
func (m Model) renderAssistantBlock(block ContentBlock) string {
	return assistantStyle.Render(block.Content)
}

// renderThinkingBlock renders thinking text
func (m Model) renderThinkingBlock(block ContentBlock) string {
	return thinkingStyle.Render(block.Content)
}

// renderToolCallBlock renders a tool call notification
func (m Model) renderToolCallBlock(block ContentBlock) string {
	icon := toolCallStyle.Render(iconToolCall + " ")
	name := toolNameStyle.Render(block.ToolName)
	desc := toolDescStyle.Render(" - " + block.Content)
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
			lines := strings.Split(block.Content, "\n")
			// Limit lines
			if len(lines) > 10 {
				lines = append(lines[:10], "... (truncated)")
			}
			for _, line := range lines {
				b.WriteString("\n")
				indent := toolResultIndentStyle.Render("  " + iconIndent + " ")
				b.WriteString(indent + line)
			}
		}
	}

	return b.String()
}

// renderErrorBlock renders an error message
func (m Model) renderErrorBlock(block ContentBlock) string {
	return errorStyle.Render("Error: " + block.Content)
}

// renderInfoBlock renders an info message
func (m Model) renderInfoBlock(block ContentBlock) string {
	icon := infoStyle.Render(iconInfo + " ")
	return icon + block.Content
}

// renderWarningBlock renders a warning message
func (m Model) renderWarningBlock(block ContentBlock) string {
	return warningStyle.Render("Warning: " + block.Content)
}

// renderSuccessBlock renders a success message
func (m Model) renderSuccessBlock(block ContentBlock) string {
	icon := successStyle.Render(iconSuccess + " ")
	return icon + block.Content
}
