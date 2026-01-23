package tui

import (
	"fmt"
	"strings"

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

	// Input prompt
	prompt := inputPromptStyle.Render("> ")

	// Spinner with activity message if processing
	var suffix string
	if m.state == StateStreaming && m.activityMessage != "" {
		frame := GetSpinnerFrame(m.spinnerFrame)
		suffix = spinnerStyle.Render(" " + frame + " " + m.activityMessage)
	}

	// Build input line
	inputLine := prompt + m.textInput.View() + suffix

	return footerStyle.Width(m.width).Render(inputLine)
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
