package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/logger"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// tuiLog is a prefixed logger for TUI events
var tuiLog = logger.WithPrefix("TUI")

// AppState represents the current state of the application
type AppState int

const (
	StateStarting    AppState = iota // TUI is initializing
	StateReady                       // TUI ready, showing welcome
	StateIdle                        // Waiting for user input
	StateStreaming                   // Streaming LLM response
	StatePermission                  // Waiting for permission input
	StateRateLimited                 // Waiting for rate limit to clear
)

// BlockType represents the type of content block
type BlockType int

const (
	BlockUser       BlockType = iota // User message
	BlockAssistant                   // Assistant response
	BlockThinking                    // Thinking text
	BlockToolCall                    // Tool call notification
	BlockToolResult                  // Tool execution result
	BlockError                       // Error message
	BlockInfo                        // Info message
	BlockWarning                     // Warning message
	BlockSuccess                     // Success message
)

// ContentBlock represents a piece of content in the conversation
type ContentBlock struct {
	Type     BlockType
	Content  string
	ToolName string
	IsError  bool
}

// PermissionResult represents the user's permission decision
type PermissionResult struct {
	Decision string // "y", "n", "a", "v"
}

// modelCallbacks holds callbacks that need to survive model copies
type modelCallbacks struct {
	onSubmit func(string)
	onReady  func()
}

// Model is the main Bubble Tea model for the TUI
type Model struct {
	// Dimensions
	width  int
	height int

	// State
	state     AppState
	modelName string
	sessionID string // Current session ID (short form for display)
	ready     bool

	// Content
	blocks    *[]ContentBlock  // Pointer to survive model copies
	streaming *strings.Builder // Pointer to survive model copies

	// Components
	viewport  viewport.Model
	textInput textinput.Model

	// Channels
	streamChan chan StreamMsg
	resultChan chan PermissionResult
	readyChan  chan struct{} // Signals when TUI is ready

	// Permission state
	permToolName    string
	permLevel       string
	permDescription string

	// Spinner state
	spinnerActive bool
	spinnerFrame  int

	// Activity indicator state
	activityMessage string // Current activity message (e.g., "Thinking...", "Running: bash")

	// Session statistics
	inputTokens   int64     // Total input tokens used in session
	outputTokens  int64     // Total output tokens used in session
	loopIteration int       // Current loop iteration
	maxIterations int       // Maximum loop iterations
	loopStartTime time.Time // When the current loop started

	// Context tracking
	contextUsage float64 // Context usage as percentage (0.0 - 1.0)
	contextWarn  bool    // Whether context warning threshold reached

	// Interrupt channel for ESC during streaming
	interruptChan chan struct{}

	// Rate limit state
	rateLimitInfo    *RateLimitInfo // Current rate limit info (nil if not rate limited)
	rateLimitEndTime time.Time      // When rate limit expires

	// Callbacks (use pointer so they survive copy to tea.Program)
	callbacks *modelCallbacks

	// Quit signal
	quitting bool

	// Input queue for messages submitted during processing
	inputQueue   []string
	maxQueueSize int

	// Architect mode state
	architectMode    bool   // Whether in architect mode
	architectSubMode string // "plan" or "chat" (toggle with shift+tab)
}

// NewModel creates a new TUI model
func NewModel(modelName string, streamChan chan StreamMsg) Model {
	ti := textinput.New()
	ti.Placeholder = "Type message..."
	ti.Prompt = "" // We render our own prompt in the footer
	ti.Focus()
	ti.CharLimit = 0 // No limit
	ti.Width = 50

	blocks := make([]ContentBlock, 0)
	return Model{
		state:         StateStarting,
		modelName:     modelName,
		blocks:        &blocks,             // Pointer survives model copies
		streaming:     &strings.Builder{}, // Pointer survives model copies
		streamChan:    streamChan,
		resultChan:    make(chan PermissionResult, 1),
		readyChan:     make(chan struct{}),
		textInput:     ti,
		maxIterations: 20,
		interruptChan: make(chan struct{}, 1),
		inputQueue:    make([]string, 0, 10),
		maxQueueSize:  10,
		callbacks:     &modelCallbacks{}, // Pointer survives copy to tea.Program
	}
}

// SetOnReady sets the callback for when TUI is ready
func (m *Model) SetOnReady(fn func()) {
	tuiLog.Debug("SetOnReady: callbacks=%p, hasCallback=%v", m.callbacks, fn != nil)
	m.callbacks.onReady = fn
}

// GetReadyChan returns the ready channel
func (m *Model) GetReadyChan() <-chan struct{} {
	return m.readyChan
}

// SetSubmitCallback sets the callback for when user submits a query
func (m *Model) SetSubmitCallback(fn func(string)) {
	m.callbacks.onSubmit = fn
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	// Start with spinner for loading animation
	// Don't start stream listener until we're ready
	return tea.Batch(
		textinput.Blink,
		tickCmd(), // Start spinner for loading animation
	)
}

// waitForStream returns a command that waits for stream messages
func (m Model) waitForStream() tea.Cmd {
	return func() tea.Msg {
		msg := <-m.streamChan
		return msg
	}
}

// signalReady signals that the TUI is ready and calls the onReady callback
func (m Model) signalReady() tea.Cmd {
	// Capture callbacks pointer to ensure we use the shared instance
	callbacks := m.callbacks
	tuiLog.Debug("signalReady: callbacks=%p, hasOnReady=%v", callbacks, callbacks != nil && callbacks.onReady != nil)
	return func() tea.Msg {
		tuiLog.Debug("signalReady cmd executing")
		// Close ready channel to signal ready (non-blocking for multiple listeners)
		select {
		case <-m.readyChan:
			tuiLog.Debug("signalReady: readyChan already closed")
		default:
			close(m.readyChan)
			tuiLog.Debug("signalReady: closed readyChan")
		}
		// Call onReady callback if set
		if callbacks != nil && callbacks.onReady != nil {
			tuiLog.Debug("signalReady: calling onReady callback")
			callbacks.onReady()
			tuiLog.Debug("signalReady: onReady callback returned")
		} else {
			tuiLog.Debug("signalReady: no onReady callback set")
		}
		return nil
	}
}

// AddBlock adds a content block to the conversation
func (m *Model) AddBlock(block ContentBlock) {
	*m.blocks = append(*m.blocks, block)
	m.updateViewportContent()
	m.scrollToBottom()
}

// ClearBlocks clears all content blocks
func (m *Model) ClearBlocks() {
	*m.blocks = []ContentBlock{}
	m.streaming.Reset()
	m.updateViewportContent()
}

// updateViewportContent updates the viewport with current content
func (m *Model) updateViewportContent() {
	content := m.renderContent()
	m.viewport.SetContent(content)
}

// scrollToBottom scrolls the viewport to the bottom
func (m *Model) scrollToBottom() {
	m.viewport.GotoBottom()
}

// IsQuitting returns true if the model is quitting
func (m Model) IsQuitting() bool {
	return m.quitting
}

// GetResultChan returns the permission result channel
func (m *Model) GetResultChan() chan PermissionResult {
	return m.resultChan
}

// GetInterruptChan returns the interrupt channel for ESC handling
func (m *Model) GetInterruptChan() chan struct{} {
	return m.interruptChan
}

// GetSessionStats returns current session statistics
func (m *Model) GetSessionStats() SessionStats {
	return SessionStats{
		LoopIteration:  m.loopIteration,
		MaxIterations:  m.maxIterations,
		LoopStartTime:  m.loopStartTime,
		InputTokens:    m.inputTokens,
		OutputTokens:   m.outputTokens,
	}
}

// ResetLoopStats resets the loop-specific stats for a new query
func (m *Model) ResetLoopStats() {
	m.loopIteration = 0
	m.loopStartTime = time.Now()
}

// QueueInput adds an input to the queue. Returns false if queue is full.
func (m *Model) QueueInput(input string) bool {
	if len(m.inputQueue) >= m.maxQueueSize {
		return false
	}
	m.inputQueue = append(m.inputQueue, input)
	return true
}

// GetQueueLength returns the number of items in the queue.
func (m *Model) GetQueueLength() int {
	return len(m.inputQueue)
}

// DequeueInput removes and returns the next input from the queue.
func (m *Model) DequeueInput() (string, bool) {
	if len(m.inputQueue) == 0 {
		return "", false
	}
	input := m.inputQueue[0]
	m.inputQueue = m.inputQueue[1:]
	return input, true
}

// ClearQueue removes all items from the queue.
func (m *Model) ClearQueue() {
	m.inputQueue = m.inputQueue[:0]
}

// SetArchitectMode enables or disables architect mode
func (m *Model) SetArchitectMode(enabled bool) {
	m.architectMode = enabled
	if enabled {
		m.architectSubMode = "chat" // Start in chat mode
	} else {
		m.architectSubMode = ""
	}
}

// IsArchitectMode returns whether architect mode is active
func (m *Model) IsArchitectMode() bool {
	return m.architectMode
}

// GetArchitectSubMode returns the current sub-mode ("plan" or "chat")
func (m *Model) GetArchitectSubMode() string {
	return m.architectSubMode
}

// ToggleArchitectSubMode switches between plan and chat sub-modes
func (m *Model) ToggleArchitectSubMode() string {
	if m.architectSubMode == "plan" {
		m.architectSubMode = "chat"
	} else {
		m.architectSubMode = "plan"
	}
	return m.architectSubMode
}

// GetConversationText returns the conversation as plain text for copying
func (m *Model) GetConversationText() string {
	var b strings.Builder
	for _, block := range *m.blocks {
		switch block.Type {
		case BlockUser:
			b.WriteString("User: ")
			b.WriteString(block.Content)
			b.WriteString("\n\n")
		case BlockAssistant:
			b.WriteString("Assistant: ")
			b.WriteString(block.Content)
			b.WriteString("\n\n")
		case BlockToolCall:
			b.WriteString(fmt.Sprintf("[Tool: %s] %s\n", block.ToolName, block.Content))
		case BlockToolResult:
			if block.IsError {
				b.WriteString(fmt.Sprintf("[Error: %s] %s\n", block.ToolName, block.Content))
			} else if block.Content != "" && block.Content != "(no output)" {
				b.WriteString(fmt.Sprintf("[Result: %s]\n%s\n\n", block.ToolName, block.Content))
			}
		case BlockError:
			b.WriteString("Error: ")
			b.WriteString(block.Content)
			b.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(b.String())
}
