package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/logging"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// appLog is a prefixed logger for TUI app events (lazy-init)
var appLog *logging.Logger

// getAppLog returns the prefixed logger, initializing lazily if needed
func getAppLog() *logging.Logger {
	if appLog == nil {
		if log := logging.Global(); log != nil {
			appLog = log.WithPrefix("TUI-APP")
		}
	}
	return appLog
}

// logAppDebug logs a debug message with printf-style formatting for TUI app
func logAppDebug(format string, args ...any) {
	if log := getAppLog(); log != nil {
		log.Debug(fmt.Sprintf(format, args...))
	}
}

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
		logAppDebug("WindowSizeMsg received: %dx%d, ready=%v", msg.Width, msg.Height, m.ready)
		m.width = msg.Width
		m.height = msg.Height

		// Calculate viewport size (total height minus header and footer)
		// Header: 1 line, Footer: 2 lines (status bar + input line)
		headerHeight := 1
		footerHeight := 2
		viewportHeight := m.height - headerHeight - footerHeight - 2
		if viewportHeight < 1 {
			viewportHeight = 1 // Prevent negative/zero viewport height on small terminals
		}

		if !m.ready {
			logAppDebug("First WindowSizeMsg - initializing TUI, callbacks=%p", m.callbacks)
			// Initialize viewport
			m.viewport = viewport.New(m.width, viewportHeight)
			m.textArea.SetWidth(m.width - 4)
			m.ready = true
			m.state = StateIdle
			m.spinnerActive = false

			// Signal ready and start stream listener
			logAppDebug("Returning signalReady command")
			return m, tea.Batch(m.waitForStream(), m.signalReady())
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = viewportHeight
			m.textArea.SetWidth(m.width - 4)
		}

		m.updateViewportContent()
		return m, nil

	case StreamMsg:
		return m.handleStreamMsg(msg)

	case TickMsg:
		if m.spinnerActive {
			m.spinnerFrame++
			// Only re-render viewport if there are active tools with spinners
			if len(m.activeTools) > 0 {
				m.viewportDirty = true
			}
			// Batch viewport updates: only re-render if dirty
			if m.viewportDirty {
				m.updateViewportContent()
				m.viewportDirty = false
			}
			return m, tickCmd()
		}
		return m, nil

	case VecgrepDebounceMsg:
		// Debounce expired — check if this is still the current query
		if msg.DebounceID != *m.vecgrepDebounceID {
			return m, nil // Stale debounce, ignore
		}
		// Fire async vecgrep search in goroutine
		query := msg.Query
		queryCtx := msg.QueryContext
		debounceID := msg.DebounceID
		projectRoot := m.projectRoot
		return m, func() tea.Msg {
			items := SearchVecgrep(context.Background(), query, queryCtx, projectRoot, 8)
			return VecgrepCompletionMsg{
				Items:      items,
				Query:      query,
				DebounceID: debounceID,
			}
		}

	case VecgrepCompletionMsg:
		// Async vecgrep results arrived — merge if still relevant
		if msg.DebounceID != *m.vecgrepDebounceID {
			return m, nil // Stale results, ignore
		}
		if m.engine.IsActive() && m.engine.ActiveTrigger() == TriggerAt {
			m.engine.MergeAsyncResults(msg.Items)
		}
		return m, nil

	case SuggestDebounceMsg:
		// Proactive suggestion debounce expired
		if msg.DebounceID != *m.suggestDebounceID {
			return m, nil // Stale
		}
		query := msg.Query
		debounceID := msg.DebounceID
		projectRoot := m.projectRoot
		return m, func() tea.Msg {
			items := SearchVecgrep(context.Background(), query, "", projectRoot, 3)
			var files []SuggestedFile
			for _, item := range items {
				relPath := item.Label
				if item.Detail != "" {
					relPath = item.Detail + "/" + item.Label
				}
				files = append(files, SuggestedFile{
					RelPath:  relPath,
					Language: item.Language,
					Score:    item.Score,
				})
			}
			return SuggestResultMsg{
				Files:      files,
				Query:      query,
				DebounceID: debounceID,
			}
		}

	case SuggestResultMsg:
		// Proactive suggestions arrived
		if msg.DebounceID != *m.suggestDebounceID {
			return m, nil // Stale
		}
		m.SetSuggestedFiles(msg.Files, msg.Query)
		return m, nil

	case RenderTickMsg:
		// Debounced render tick: render streaming markdown now if there's pending content
		if m.streaming.Len() > 0 && m.streaming.Len() > m.lastRenderedLen {
			m.renderedCache = renderMarkdown(m.streaming.String())
			m.lastRenderedLen = m.streaming.Len()
			m.lastRenderTime = time.Now()
			m.updateViewportContent()
			if !m.userScrolledUp {
				m.scrollToBottom()
			}
		}
		m.renderPending = false
		return m, nil

	case QuitMsg:
		m.quitting = true
		// Close doneChan to unblock all waitForStream goroutines
		select {
		case <-m.doneChan:
			// Already closed
		default:
			close(m.doneChan)
		}
		return m, tea.Quit
	}

	// Update text input
	m.textArea, cmd = m.textArea.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleModeCycle cycles the agent mode and shows feedback
func (m Model) handleModeCycle() (tea.Model, tea.Cmd) {
	newMode := m.CycleAgentMode()
	var feedback string
	switch newMode {
	case ModeAsk:
		feedback = "Ask mode — read-only exploration, Q&A"
	case ModePlan:
		feedback = "Plan mode — design & explore, writes prompt"
	case ModeBuild:
		feedback = "Build mode — full execution"
	}
	m.AddBlock(ContentBlock{Type: BlockInfo, Content: feedback})
	return m, nil
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	logAppDebug("Key: type=%d str=%q", msg.Type, msg.String())

	// Handle Ctrl+C globally
	if msg.Type == tea.KeyCtrlC {
		m.quitting = true
		// Close doneChan to unblock all waitForStream goroutines
		select {
		case <-m.doneChan:
		default:
			close(m.doneChan)
		}
		return m, tea.Quit
	}

	// Handle F1: toggle help overlay
	if msg.Type == tea.KeyF1 {
		m.showHelpOverlay = !m.showHelpOverlay
		return m, nil
	}

	// ESC dismisses help overlay if shown
	if msg.Type == tea.KeyEsc && m.showHelpOverlay {
		m.showHelpOverlay = false
		return m, nil
	}

	// Handle ESC during streaming or rate limiting — double-ESC interrupt
	if msg.Type == tea.KeyEsc && (m.state == StateStreaming || m.state == StateRateLimited) {
		now := time.Now()
		if m.interruptPending && now.Sub(m.lastInterruptTime) < 3*time.Second {
			// Second ESC within 3s → force stop: immediately reset to idle
			select {
			case m.forceInterruptChan <- struct{}{}:
			default:
			}
			m.streaming.Reset()
			m.renderedCache = ""
			m.lastRenderedLen = 0
			m.renderPending = false
			m.activityMessage = ""
			m.interruptPending = false
			m.state = StateIdle
			m.textArea.Focus()
			return m, nil
		}
		// First ESC → graceful interrupt
		select {
		case m.interruptChan <- struct{}{}:
		default:
		}
		m.interruptPending = true
		m.lastInterruptTime = now
		m.activityMessage = "Stopping... (ESC to force stop)"
		return m, nil
	}

	// Handle permission state first - respond immediately on keydown
	if m.state == StatePermission {
		key := msg.String()
		switch key {
		case "y", "Y", "n", "N", "a", "A", "v", "V":
			// Handle permission keys immediately on keydown (lowercase the response)
			m.permDetailsExpanded = false
			return m.handlePermissionKey(string([]byte{key[0] | 0x20})) // convert to lowercase
		case "d", "D":
			// Toggle permission details
			m.permDetailsExpanded = !m.permDetailsExpanded
			return m, nil
		}
		// Treat Esc as deny
		if msg.Type == tea.KeyEsc {
			m.permDetailsExpanded = false
			return m.handlePermissionKey("n")
		}
		// Ignore all other keys in permission state
		return m, nil
	}

	// Handle completion engine when active (intercept before viewport scrolling)
	if m.engine.IsActive() {
		switch msg.Type {
		case tea.KeyUp:
			m.engine.MoveUp()
			return m, nil
		case tea.KeyDown:
			m.engine.MoveDown()
			return m, nil
		case tea.KeyTab:
			// Accept selection, fill input (don't submit)
			item := m.engine.Accept()
			if item.InsertText != "" {
				m.textArea.SetValue(item.InsertText)
				m.textArea.CursorEnd()
			}
			return m, nil
		case tea.KeyEnter:
			// Accept — behavior depends on item kind
			item := m.engine.Accept()
			if item.InsertText != "" {
				if item.Kind == KindCommand || item.Kind == KindArgument {
					// Commands: accept and submit immediately
					m.textArea.Reset()
					if m.callbacks.onSubmit != nil {
						go m.callbacks.onSubmit(item.InsertText)
					}
				} else {
					// Files/chunks: insert text, don't submit (user continues typing)
					m.textArea.SetValue(item.InsertText)
					m.textArea.CursorEnd()
				}
			}
			return m, nil
		case tea.KeyEsc:
			m.engine.Dismiss()
			return m, nil
		}
	}

	// Handle viewport scrolling (arrow keys, page up/down, home/end)
	switch msg.Type {
	case tea.KeyUp, tea.KeyPgUp, tea.KeyHome:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		// Track that user scrolled up
		if !m.viewport.AtBottom() {
			m.userScrolledUp = true
		}
		return m, cmd
	case tea.KeyDown, tea.KeyPgDown:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		// Clear scroll-up state if we've reached the bottom
		if m.viewport.AtBottom() {
			m.userScrolledUp = false
			m.newContentPending = false
		}
		return m, cmd
	case tea.KeyEnd:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.userScrolledUp = false
		m.newContentPending = false
		return m, cmd
	case tea.KeyShiftTab:
		return m.handleModeCycle()
	}

	// Fallback for terminals that don't send standard CSI backtab
	if msg.String() == "shift+tab" {
		return m.handleModeCycle()
	}

	// Handle Ctrl+Y: copy last assistant response to clipboard
	if msg.Type == tea.KeyCtrlY {
		if text := getLastAssistantResponse(*m.blocks); text != "" {
			if err := copyToClipboard(text); err != nil {
				m.AddBlock(ContentBlock{Type: BlockWarning, Content: "Clipboard: " + err.Error()})
			} else {
				m.AddBlock(ContentBlock{Type: BlockSuccess, Content: "Copied last response to clipboard"})
			}
		} else {
			m.AddBlock(ContentBlock{Type: BlockInfo, Content: "No assistant response to copy"})
		}
		return m, nil
	}

	// Handle Ctrl+K: copy last code block to clipboard
	if msg.Type == tea.KeyCtrlK {
		if code := getLastCodeBlock(*m.blocks); code != "" {
			if err := copyToClipboard(code); err != nil {
				m.AddBlock(ContentBlock{Type: BlockWarning, Content: "Clipboard: " + err.Error()})
			} else {
				lines := strings.Count(code, "\n") + 1
				m.AddBlock(ContentBlock{Type: BlockSuccess, Content: fmt.Sprintf("Copied code block (%d lines) to clipboard", lines)})
			}
		} else {
			m.AddBlock(ContentBlock{Type: BlockInfo, Content: "No code block to copy"})
		}
		return m, nil
	}

	// Handle Ctrl+T: toggle collapse/expand all tool blocks
	if msg.Type == tea.KeyCtrlT {
		m.toolsCollapsed = !m.toolsCollapsed
		for i := range *m.blocks {
			b := &(*m.blocks)[i]
			if (b.Type == BlockToolResult && !b.IsError) || b.Type == BlockThinking {
				b.Collapsed = m.toolsCollapsed
				b.RenderedCache = "" // Invalidate cache on collapse toggle
			}
		}
		m.updateViewportContent()
		return m, nil
	}

	// Alt+Enter: insert a newline in textarea (multi-line input)
	if msg.String() == "alt+enter" {
		m.textArea.InsertString("\n")
		// Recalculate footer height for multi-line
		m.recalcFooterHeight()
		return m, nil
	}

	// Handle Up/Down for history navigation when textarea is empty or single-line
	if msg.Type == tea.KeyUp && m.textArea.Value() == "" && !m.engine.IsActive() {
		if len(m.inputHistory) > 0 {
			if m.historyIdx == -1 {
				// Entering history mode: save current input
				m.historySavedIn = m.textArea.Value()
				m.historyIdx = 0
			} else if m.historyIdx < len(m.inputHistory)-1 {
				m.historyIdx++
			}
			m.textArea.SetValue(m.inputHistory[m.historyIdx])
			m.textArea.CursorEnd()
			return m, nil
		}
	}
	if msg.Type == tea.KeyDown && m.historyIdx >= 0 && !m.engine.IsActive() {
		m.historyIdx--
		if m.historyIdx < 0 {
			// Back to current (pre-history) input
			m.historyIdx = -1
			m.textArea.SetValue(m.historySavedIn)
		} else {
			m.textArea.SetValue(m.inputHistory[m.historyIdx])
		}
		m.textArea.CursorEnd()
		return m, nil
	}

	// Handle normal input state
	switch msg.Type {
	case tea.KeyEnter:
		input := strings.TrimSpace(m.textArea.Value())
		if input == "" {
			return m, nil
		}

		// Add to history (prepend, cap at 50)
		m.inputHistory = append([]string{input}, m.inputHistory...)
		if len(m.inputHistory) > 50 {
			m.inputHistory = m.inputHistory[:50]
		}
		m.historyIdx = -1

		m.textArea.Reset()
		m.textArea.SetHeight(1) // Reset to single line
		m.engine.Dismiss()
		m.recalcFooterHeight()

		// Slash commands execute immediately (bypass queue)
		if strings.HasPrefix(input, "/") {
			if m.callbacks.onSubmit != nil {
				go m.callbacks.onSubmit(input)
			}
			return m, nil
		}

		// Parse @file mentions from input and resolve absolute paths
		parsed := ParseFileTags(input, m.projectRoot)
		for _, tag := range parsed.NewTags {
			if tag.AbsPath == "" && m.projectRoot != "" {
				tag.AbsPath = filepath.Join(m.projectRoot, tag.RelPath)
			}
			m.AddTaggedFile(tag)
		}
		submitText := input
		if parsed.CleanQuery != "" {
			submitText = parsed.CleanQuery
		}

		// If idle, execute immediately
		if m.state == StateIdle {
			// Add user message to blocks (show original input with @mentions)
			m.AddBlock(ContentBlock{
				Type:    BlockUser,
				Content: input,
			})

			// Immediately transition to streaming state to prevent double-submit
			m.state = StateStreaming
			m.activityMessage = "Processing..."
			m.loopIteration = 0
			m.loopStartTime = time.Now()

			// Call submit callback with clean query
			if m.callbacks.onSubmit != nil {
				go m.callbacks.onSubmit(submitText)
			}

			return m, tea.Batch(m.waitForStream(), startSpinner(&m))
		}

		// If busy (streaming or rate limited), queue the input
		if m.state == StateStreaming || m.state == StateRateLimited {
			if !m.QueueInput(submitText) {
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

	// Update textarea for regular typing
	var cmd tea.Cmd
	m.textArea, cmd = m.textArea.Update(msg)

	// Any edit exits history mode
	if m.historyIdx >= 0 {
		m.historyIdx = -1
	}

	// Update completion engine after each keystroke
	m.engine.Update(m.textArea.Value())

	// If @ completion is active, start debounced vecgrep search
	var extraCmds []tea.Cmd
	fullInput := m.textArea.Value()
	if m.engine.IsActive() && m.engine.ActiveTrigger() == TriggerAt && m.engine.query != "" {
		*m.vecgrepDebounceID++
		debounceID := *m.vecgrepDebounceID
		query := m.engine.query
		// Extract query context: text before the @ trigger
		queryCtx := ""
		if atIdx := findActiveTrigger(fullInput, '@'); atIdx > 0 {
			queryCtx = strings.TrimSpace(fullInput[:atIdx])
		}
		m.engine.SetLoading(true)
		extraCmds = append(extraCmds, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
			return VecgrepDebounceMsg{Query: query, QueryContext: queryCtx, DebounceID: debounceID}
		}))
	}

	// Proactive file suggestions: when user types 15+ chars without @, suggest files
	if !m.engine.IsActive() && len(fullInput) >= 15 && !strings.Contains(fullInput, "@") && !strings.HasPrefix(fullInput, "/") {
		*m.suggestDebounceID++
		debounceID := *m.suggestDebounceID
		query := strings.TrimSpace(fullInput)
		extraCmds = append(extraCmds, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return SuggestDebounceMsg{Query: query, DebounceID: debounceID}
		}))
	} else if len(fullInput) < 15 || strings.HasPrefix(fullInput, "/") {
		// Clear suggestions when input shrinks or is a command
		m.ClearSuggestions()
	}

	// Recalculate footer/viewport height
	m.recalcFooterHeight()

	if len(extraCmds) > 0 {
		return m, tea.Batch(append([]tea.Cmd{cmd}, extraCmds...)...)
	}
	return m, cmd
}

// handlePermissionKey handles permission prompt responses
func (m Model) handlePermissionKey(key string) (tea.Model, tea.Cmd) {
	// Send result through channel
	select {
	case m.resultChan <- PermissionResult{Decision: key}:
	default:
	}

	// Show feedback based on decision
	var feedback ContentBlock
	switch key {
	case "y":
		feedback = ContentBlock{Type: BlockSuccess, Content: "Allowed"}
	case "n":
		feedback = ContentBlock{Type: BlockWarning, Content: "Denied"}
	case "a":
		feedback = ContentBlock{Type: BlockSuccess, Content: "Always allowed (cached)"}
	case "v":
		feedback = ContentBlock{Type: BlockWarning, Content: "Never allowed (cached)"}
	}
	m.AddBlock(feedback)

	// Return to streaming state and restore input focus - the agent is still running and will send
	// more messages (tool results, text, done). Transition to idle happens
	// when the "done" message arrives.
	m.state = StateStreaming
	m.textArea.Focus() // Restore focus so user can type while agent continues

	return m, m.waitForStream()
}

// handleStreamMsg handles streaming messages from the adapter
func (m Model) handleStreamMsg(msg StreamMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case "text":
		m.state = StateStreaming
		m.activityMessage = "Thinking..."
		m.streaming.WriteString(msg.Text)

		// Debounced markdown rendering: only run Glamour every 150ms
		cmds := []tea.Cmd{m.waitForStream(), startSpinner(&m)}
		elapsed := time.Since(m.lastRenderTime)
		if elapsed >= 150*time.Millisecond {
			// Enough time passed — render now
			m.renderedCache = renderMarkdown(m.streaming.String())
			m.lastRenderedLen = m.streaming.Len()
			m.lastRenderTime = time.Now()
			m.renderPending = false
			// Only update viewport on render boundaries, not every token
			m.viewportDirty = true
		} else if !m.renderPending {
			// Schedule a deferred render
			m.renderPending = true
			m.viewportDirty = true
			cmds = append(cmds, tea.Tick(150*time.Millisecond-elapsed, func(t time.Time) tea.Msg {
				return RenderTickMsg{}
			}))
		}

		// Defer viewport update to the next render tick instead of every token
		if m.viewportDirty {
			m.updateViewportContent()
			m.viewportDirty = false
		}
		if m.userScrolledUp {
			m.newContentPending = true
		} else {
			m.scrollToBottom()
		}
		return m, tea.Batch(cmds...)

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
			// Clear render cache
			m.renderedCache = ""
			m.lastRenderedLen = 0
			m.renderPending = false
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
			if m.callbacks.onSubmit != nil {
				go m.callbacks.onSubmit(nextInput)
			}
			return m, tea.Batch(m.waitForStream(), startSpinner(&m))
		}

		// No more queued - return to idle
		m.state = StateIdle
		m.spinnerActive = false
		m.activityMessage = ""
		m.interruptPending = false
		m.textArea.Focus() // Re-enable input for next query
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

		// Create tool meta with timing and category
		meta := &ToolBlockMeta{
			ToolType:  classifyTool(msg.ToolName),
			StartTime: time.Now(),
			IsRunning: true,
			GroupID:   msg.GroupID,
		}
		// Track running tool
		if msg.GroupID != "" {
			m.activeTools[msg.GroupID] = meta
		}

		m.AddBlock(ContentBlock{
			Type:     BlockToolCall,
			ToolName: msg.ToolName,
			Content:  msg.ToolDesc,
			ToolMeta: meta,
		})
		return m, tea.Batch(m.waitForStream(), startSpinner(&m))

	case "tool_result":
		// Clear activity message when tool completes
		m.activityMessage = ""

		// Calculate elapsed time from matching tool_call
		var resultMeta *ToolBlockMeta
		if msg.GroupID != "" {
			if callMeta, ok := m.activeTools[msg.GroupID]; ok {
				callMeta.IsRunning = false
				callMeta.EndTime = time.Now()
				callMeta.Elapsed = callMeta.EndTime.Sub(callMeta.StartTime)
				// Invalidate the tool_call block's cache since it stopped spinning
				for i := range *m.blocks {
					b := &(*m.blocks)[i]
					if b.ToolMeta != nil && b.ToolMeta.GroupID == msg.GroupID {
						b.RenderedCache = ""
						break
					}
				}
				resultMeta = &ToolBlockMeta{
					ToolType:  callMeta.ToolType,
					StartTime: callMeta.StartTime,
					EndTime:   callMeta.EndTime,
					Elapsed:   callMeta.Elapsed,
					GroupID:   msg.GroupID,
					ResultLen: len(msg.Text),
				}
				delete(m.activeTools, msg.GroupID)
			}
		}

		// Truncate long results (UTF-8 safe)
		result := msg.Text
		if len(result) > 500 {
			result = truncateUTF8Safe(result, 500)
		}
		m.AddBlock(ContentBlock{
			Type:     BlockToolResult,
			ToolName: msg.ToolName,
			Content:  result,
			IsError:  msg.IsError,
			ToolMeta: resultMeta,
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
			if m.callbacks.onSubmit != nil {
				go m.callbacks.onSubmit(nextInput)
			}
			return m, tea.Batch(m.waitForStream(), startSpinner(&m))
		}

		// No more queued - return to idle
		m.state = StateIdle
		m.spinnerActive = false
		m.activityMessage = ""
		m.interruptPending = false
		m.textArea.Focus() // Re-enable input for next query
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
		m.permDetailsExpanded = false
		m.permFullContent = msg.ToolDesc
		m.textArea.Blur()
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
		m.textArea.Focus() // Re-enable input
		return m, m.waitForStream()

	case "context_stats":
		if msg.ContextStats != nil {
			m.contextUsage = msg.ContextStats.UsagePercent
			m.contextWarn = msg.ContextStats.NeedsWarning
		}
		return m, m.waitForStream()

	case "session_id":
		m.sessionID = msg.Text
		return m, m.waitForStream()

	case "project_info":
		if msg.ProjectInfo != nil {
			m.workingDir = msg.ProjectInfo.WorkingDir
			m.gitBranch = msg.ProjectInfo.GitBranch
		}
		return m, m.waitForStream()

	case "progress":
		m.progressInfo = msg.ProgressData
		return m, m.waitForStream()

	case "progress_clear":
		m.progressInfo = nil
		return m, m.waitForStream()

	case "plan":
		m.AddBlock(ContentBlock{
			Type:    BlockPlan,
			Content: msg.Text,
		})
		// Store the block's unique ID (stable across insertions/deletions)
		m.planBlockID = (*m.blocks)[len(*m.blocks)-1].BlockID
		m.updateViewportContent()
		return m, m.waitForStream()

	case "plan_update":
		// Find plan block by stable ID instead of array index
		for i := range *m.blocks {
			if (*m.blocks)[i].BlockID == m.planBlockID {
				(*m.blocks)[i].Content = msg.Text
				(*m.blocks)[i].RenderedCache = "" // Invalidate cache for re-render
				break
			}
		}
		m.updateViewportContent()
		return m, m.waitForStream()

	case "mode_change":
		if msg.ModeInfo != nil {
			m.agentMode = *msg.ModeInfo
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

// recalcFooterHeight recalculates the footer height based on textarea and completer state
func (m *Model) recalcFooterHeight() {
	completerLines := 0
	if m.engine.IsActive() {
		visible := min(m.engine.VisibleCount(), m.engine.maxVisible)
		completerLines = visible + 1 // +1 for separator
	}
	// Textarea height: min 1, max 5
	taLines := max(m.textArea.LineCount(), 1)
	if taLines > 5 {
		taLines = 5
	}
	m.textArea.SetHeight(taLines)

	// Footer: 1 status bar + textarea lines + completer
	newFooterHeight := 1 + taLines + completerLines
	newViewportHeight := m.height - 1 - newFooterHeight - 2
	if newViewportHeight > 0 && newViewportHeight != m.viewport.Height {
		m.viewport.Height = newViewportHeight
		m.updateViewportContent()
	}
}

// startSpinner starts the spinner if not already active
func startSpinner(m *Model) tea.Cmd {
	if !m.spinnerActive {
		m.spinnerActive = true
		return tickCmd()
	}
	return nil
}
