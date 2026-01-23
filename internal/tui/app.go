package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate viewport size (total height minus header and footer)
		headerHeight := 1
		footerHeight := 1
		viewportHeight := m.height - headerHeight - footerHeight - 2

		if !m.ready {
			// Initialize viewport
			m.viewport = viewport.New(m.width, viewportHeight)
			m.textInput.Width = m.width - 4
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = viewportHeight
			m.textInput.Width = m.width - 4
		}

		m.updateViewportContent()
		return m, nil

	case StreamMsg:
		return m.handleStreamMsg(msg)

	case TickMsg:
		if m.spinnerActive {
			m.spinnerFrame++
			return m, tickCmd()
		}
		return m, nil

	case QuitMsg:
		m.quitting = true
		return m, tea.Quit
	}

	// Update text input
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle Ctrl+C globally
	if msg.Type == tea.KeyCtrlC {
		m.quitting = true
		return m, tea.Quit
	}

	// Handle permission state first - respond immediately on keydown
	if m.state == StatePermission {
		key := msg.String()
		switch key {
		case "y", "Y", "n", "N", "a", "A", "v", "V":
			// Handle permission keys immediately on keydown (lowercase the response)
			return m.handlePermissionKey(string([]byte{key[0] | 0x20})) // convert to lowercase
		}
		// Treat Esc as deny
		if msg.Type == tea.KeyEsc {
			return m.handlePermissionKey("n")
		}
		// Ignore all other keys in permission state
		return m, nil
	}

	// Handle normal input state
	switch msg.Type {
	case tea.KeyEnter:
		if m.state == StateIdle {
			input := m.textInput.Value()
			if input == "" {
				return m, nil
			}

			// Clear input
			m.textInput.Reset()

			// Add user message to blocks
			m.AddBlock(ContentBlock{
				Type:    BlockUser,
				Content: input,
			})

			// Call submit callback if set
			if m.onSubmit != nil {
				go m.onSubmit(input)
			}

			return m, nil
		}
		return m, nil

	case tea.KeyEsc:
		return m, nil
	}

	// Update text input for regular typing
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// handlePermissionKey handles permission prompt responses
func (m Model) handlePermissionKey(key string) (tea.Model, tea.Cmd) {
	// Send result through channel
	select {
	case m.resultChan <- PermissionResult{Decision: key}:
	default:
	}

	// Return to idle state
	m.state = StateIdle
	m.textInput.Focus()

	return m, nil
}

// handleStreamMsg handles streaming messages from the adapter
func (m Model) handleStreamMsg(msg StreamMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case "text":
		m.state = StateStreaming
		m.activityMessage = "Thinking..."
		m.streaming.WriteString(msg.Text)
		m.updateViewportContent()
		m.scrollToBottom()
		return m, tea.Batch(m.waitForStream(), startSpinner(&m))

	case "thinking":
		m.state = StateStreaming
		m.activityMessage = "Reasoning..."
		// Add thinking block or append to existing
		m.AddBlock(ContentBlock{
			Type:    BlockThinking,
			Content: msg.Text,
		})
		return m, tea.Batch(m.waitForStream(), startSpinner(&m))

	case "done":
		// Commit streaming content to a block
		if m.streaming.Len() > 0 {
			m.AddBlock(ContentBlock{
				Type:    BlockAssistant,
				Content: m.streaming.String(),
			})
			m.streaming.Reset()
		}
		m.state = StateIdle
		m.spinnerActive = false
		m.activityMessage = ""
		m.updateViewportContent()
		return m, m.waitForStream()

	case "tool_call":
		// Set activity message for tool execution
		m.activityMessage = "Running: " + msg.ToolName
		if msg.ToolDesc != "" {
			m.activityMessage = "Running: " + msg.ToolName + " - " + truncate(msg.ToolDesc, 30)
		}
		m.state = StateStreaming
		m.AddBlock(ContentBlock{
			Type:     BlockToolCall,
			ToolName: msg.ToolName,
			Content:  msg.ToolDesc,
		})
		return m, tea.Batch(m.waitForStream(), startSpinner(&m))

	case "tool_result":
		// Clear activity message when tool completes
		m.activityMessage = ""
		// Truncate long results
		result := msg.Text
		if len(result) > 500 {
			result = result[:500] + "..."
		}
		m.AddBlock(ContentBlock{
			Type:     BlockToolResult,
			ToolName: msg.ToolName,
			Content:  result,
			IsError:  msg.IsError,
		})
		return m, m.waitForStream()

	case "activity":
		// Direct activity message update
		m.activityMessage = msg.Text
		if msg.Text != "" {
			m.state = StateStreaming
			return m, tea.Batch(m.waitForStream(), startSpinner(&m))
		}
		return m, m.waitForStream()

	case "error":
		m.AddBlock(ContentBlock{
			Type:    BlockError,
			Content: msg.Text,
		})
		m.state = StateIdle
		m.spinnerActive = false
		m.activityMessage = ""
		return m, m.waitForStream()

	case "info":
		m.AddBlock(ContentBlock{
			Type:    BlockInfo,
			Content: msg.Text,
		})
		return m, m.waitForStream()

	case "warning":
		m.AddBlock(ContentBlock{
			Type:    BlockWarning,
			Content: msg.Text,
		})
		return m, m.waitForStream()

	case "success":
		m.AddBlock(ContentBlock{
			Type:    BlockSuccess,
			Content: msg.Text,
		})
		return m, m.waitForStream()

	case "permission":
		// Commit any streaming content first
		if m.streaming.Len() > 0 {
			m.AddBlock(ContentBlock{
				Type:    BlockAssistant,
				Content: m.streaming.String(),
			})
			m.streaming.Reset()
		}

		m.state = StatePermission
		m.permToolName = msg.ToolName
		m.permLevel = msg.Level.String()
		m.permDescription = msg.ToolDesc
		m.textInput.Blur()
		return m, m.waitForStream()

	case "clear":
		m.ClearBlocks()
		return m, m.waitForStream()

	case "model_info":
		m.modelName = msg.Text
		return m, m.waitForStream()

	case "user":
		m.AddBlock(ContentBlock{
			Type:    BlockUser,
			Content: msg.Text,
		})
		return m, m.waitForStream()
	}

	return m, m.waitForStream()
}

// tickCmd returns a command that sends a tick after a delay
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

// startSpinner starts the spinner if not already active
func startSpinner(m *Model) tea.Cmd {
	if !m.spinnerActive {
		m.spinnerActive = true
		return tickCmd()
	}
	return nil
}
