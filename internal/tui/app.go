package tui

import (
	"strings"
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
		// Header: 1 line, Footer: 2 lines (status bar + input line)
		headerHeight := 1
		footerHeight := 2
		viewportHeight := m.height - headerHeight - footerHeight - 2

		if !m.ready {
			// Initialize viewport
			m.viewport = viewport.New(m.width, viewportHeight)
			m.textInput.Width = m.width - 4
			m.ready = true
			m.state = StateIdle
			m.spinnerActive = false

			// Signal ready and start stream listener
			return m, tea.Batch(m.waitForStream(), m.signalReady())
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

	// Handle ESC during streaming or rate limiting - send interrupt signal
	if msg.Type == tea.KeyEsc && (m.state == StateStreaming || m.state == StateRateLimited) {
		// Non-blocking send to interrupt channel
		select {
		case m.interruptChan <- struct{}{}:
		default:
			// Channel full, interrupt already pending
		}
		return m, nil
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
		input := m.textInput.Value()
		if input == "" {
			return m, nil
		}
		m.textInput.Reset()

		// Slash commands execute immediately (bypass queue)
		if strings.HasPrefix(input, "/") {
			if m.onSubmit != nil {
				go m.onSubmit(input)
			}
			return m, nil
		}

		// If idle, execute immediately
		if m.state == StateIdle {
			// Add user message to blocks
			m.AddBlock(ContentBlock{
				Type:    BlockUser,
				Content: input,
			})

			// Immediately transition to streaming state to prevent double-submit
			m.state = StateStreaming
			m.activityMessage = "Processing..."
			m.loopIteration = 0
			m.loopStartTime = time.Now()

			// Call submit callback if set
			if m.onSubmit != nil {
				go m.onSubmit(input)
			}

			return m, tea.Batch(m.waitForStream(), startSpinner(&m))
		}

		// If busy (streaming or rate limited), queue the input
		if m.state == StateStreaming || m.state == StateRateLimited {
			if !m.QueueInput(input) {
				m.AddBlock(ContentBlock{Type: BlockWarning, Content: "Queue full (max 10)"})
				return m, nil
			}
			m.AddBlock(ContentBlock{Type: BlockUser, Content: input + " (queued)"})
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
		// Accumulate token usage
		if msg.Usage != nil {
			m.inputTokens += msg.Usage.InputTokens
			m.outputTokens += msg.Usage.OutputTokens
		}

		// Process next queued input if any
		if nextInput, ok := m.DequeueInput(); ok {
			m.AddBlock(ContentBlock{Type: BlockUser, Content: nextInput})
			m.activityMessage = "Processing..."
			m.loopIteration = 0
			m.loopStartTime = time.Now()
			if m.onSubmit != nil {
				go m.onSubmit(nextInput)
			}
			return m, tea.Batch(m.waitForStream(), startSpinner(&m))
		}

		// No more queued - return to idle
		m.state = StateIdle
		m.spinnerActive = false
		m.activityMessage = ""
		m.textInput.Focus() // Re-enable input for next query
		m.updateViewportContent()
		return m, m.waitForStream()

	case "stats":
		// Update session statistics
		if msg.Stats != nil {
			m.loopIteration = msg.Stats.LoopIteration
			m.maxIterations = msg.Stats.MaxIterations
			m.loopStartTime = msg.Stats.LoopStartTime
			// Don't overwrite accumulated tokens from done messages
		}
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

		// Process next queued input if any (continue despite error)
		if nextInput, ok := m.DequeueInput(); ok {
			m.AddBlock(ContentBlock{Type: BlockUser, Content: nextInput})
			m.activityMessage = "Processing..."
			m.loopIteration = 0
			m.loopStartTime = time.Now()
			if m.onSubmit != nil {
				go m.onSubmit(nextInput)
			}
			return m, tea.Batch(m.waitForStream(), startSpinner(&m))
		}

		// No more queued - return to idle
		m.state = StateIdle
		m.spinnerActive = false
		m.activityMessage = ""
		m.textInput.Focus() // Re-enable input for next query
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

	case "rate_limit":
		if msg.RateLimitInfo != nil {
			m.rateLimitInfo = msg.RateLimitInfo
			m.rateLimitEndTime = time.Now().Add(msg.RateLimitInfo.Duration)
			m.state = StateRateLimited
			m.activityMessage = "Rate limited"
			return m, tea.Batch(m.waitForStream(), startSpinner(&m))
		}
		return m, m.waitForStream()

	case "rate_limit_clear":
		m.rateLimitInfo = nil
		m.state = StateIdle
		m.activityMessage = ""
		m.textInput.Focus() // Re-enable input
		return m, m.waitForStream()

	case "context_stats":
		if msg.ContextStats != nil {
			m.contextUsage = msg.ContextStats.UsagePercent
			m.contextWarn = msg.ContextStats.NeedsWarning
		}
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
