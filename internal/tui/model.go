package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/logging"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tuiLog is a prefixed logger for TUI events (lazy-init)
var tuiLog *logging.Logger

// getTUILog returns the prefixed logger, initializing lazily if needed
func getTUILog() *logging.Logger {
	if tuiLog == nil {
		if log := logging.Global(); log != nil {
			tuiLog = log.WithPrefix("TUI")
		}
	}
	return tuiLog
}

// logTUIDebug logs a debug message with printf-style formatting for TUI model
func logTUIDebug(format string, args ...any) {
	if log := getTUILog(); log != nil {
		log.Debug(fmt.Sprintf(format, args...))
	}
}

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
	BlockPlan                        // Plan (Glamour-rendered, updatable)
)

// ContentBlock represents a piece of content in the conversation
type ContentBlock struct {
	Type          BlockType
	Content       string
	ToolName      string
	IsError       bool
	ToolMeta      *ToolBlockMeta // Metadata for tool blocks (optional)
	Collapsed     bool           // Whether this block is collapsed
	BlockID       int            // Unique block identifier
	Summary       string         // Collapsed summary text (e.g., "42 lines, 1.2 KB")
	RenderedCache string         // Cached render output (cleared when content changes)
}

// AgentMode represents the current operating mode of the agent
type AgentMode int

const (
	ModeAsk   AgentMode = iota // Q&A, read-only exploration
	ModePlan                   // Design & explore, writes prompt
	ModeBuild                  // Full execution
)

// String returns the display name of the mode
func (m AgentMode) String() string {
	switch m {
	case ModeAsk:
		return "Ask"
	case ModePlan:
		return "Plan"
	case ModeBuild:
		return "Build"
	default:
		return "Unknown"
	}
}

// Next returns the next mode in the cycle: Ask → Plan → Build → Ask
func (m AgentMode) Next() AgentMode {
	return (m + 1) % 3
}

// ToolCategory classifies a tool by its effect
type ToolCategory int

const (
	ToolCategoryRead    ToolCategory = iota // Read-only tools
	ToolCategoryWrite                       // File-modifying tools
	ToolCategoryExecute                     // Command execution tools
)

// String returns the display label for the category
func (c ToolCategory) String() string {
	switch c {
	case ToolCategoryRead:
		return "READ"
	case ToolCategoryWrite:
		return "WRITE"
	case ToolCategoryExecute:
		return "EXEC"
	default:
		return "TOOL"
	}
}

// ToolBlockMeta holds metadata for tool call visualization
type ToolBlockMeta struct {
	ToolType  ToolCategory
	StartTime time.Time
	EndTime   time.Time
	Elapsed   time.Duration
	IsRunning bool
	GroupID   string // Links tool_call to tool_result
	ResultLen int    // Original length before truncation
}

// classifyTool returns the category for a given tool name
func classifyTool(name string) ToolCategory {
	switch name {
	case "read_file", "list_files", "grep", "ast_parse", "lsp_query",
		"vecgrep_search", "vecgrep_similar", "vecgrep_status",
		"vecgrep_overview", "vecgrep_related_files":
		return ToolCategoryRead
	case "write_file", "edit_file":
		return ToolCategoryWrite
	case "bash":
		return ToolCategoryExecute
	default:
		// Default: if name contains "read", "list", "search", "get" → read
		lower := strings.ToLower(name)
		if strings.Contains(lower, "read") || strings.Contains(lower, "list") ||
			strings.Contains(lower, "search") || strings.Contains(lower, "get") ||
			strings.Contains(lower, "status") {
			return ToolCategoryRead
		}
		return ToolCategoryWrite // Default to write for unknown tools (safer)
	}
}

// PermissionResult represents the user's permission decision
type PermissionResult struct {
	Decision string // "y", "n", "a", "v"
}

// modelCallbacks holds callbacks that need to survive model copies
type modelCallbacks struct {
	onSubmit     func(string)
	onReady      func()
	onModeChange func(AgentMode)
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
	viewport viewport.Model
	textArea textarea.Model

	// Channels
	streamChan chan StreamMsg
	resultChan chan PermissionResult
	readyChan  chan struct{} // Signals when TUI is ready
	doneChan   chan struct{} // Closed on quit to unblock waitForStream goroutines

	// Permission state
	permToolName       string
	permLevel          string
	permDescription    string
	permDetailsExpanded bool   // Whether permission details are expanded
	permFullContent    string // Full content for expanded view

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

	// Interrupt channels for ESC during streaming
	interruptChan      chan struct{} // Graceful interrupt (first ESC)
	forceInterruptChan chan struct{} // Force interrupt (second ESC)
	interruptPending   bool         // Whether a graceful interrupt has been sent
	lastInterruptTime  time.Time    // When last ESC was pressed (for double-ESC detection)

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

	// Agent mode state
	agentMode AgentMode // Current mode: Ask, Plan, Build

	// Autocomplete
	completer *Completer // Slash command autocomplete (pointer survives model copies)

	// Tool visualization
	activeTools map[string]*ToolBlockMeta // Track running tools by GroupID

	// Streaming render cache (debounced markdown rendering)
	lastRenderTime  time.Time // When we last ran Glamour on the streaming buffer
	lastRenderedLen int       // Length of streaming buffer at last render
	renderedCache   string    // Cached Glamour output from last render
	renderPending   bool      // Whether a RenderTickMsg is pending

	// Smart auto-scroll (sticky bottom)
	userScrolledUp    bool // True if user has scrolled up from bottom
	newContentPending bool // True if new content arrived while user scrolled up

	// Viewport render batching
	viewportDirty bool // Whether viewport needs re-rendering

	// Collapsible blocks
	nextBlockID    int  // Monotonically increasing block ID
	toolsCollapsed bool // Global toggle for tool/thinking block collapse

	// Project context (enhanced header)
	workingDir string // Current working directory (shortened)
	gitBranch  string // Current git branch

	// Input history navigation
	inputHistory   []string // Previous inputs (newest first)
	historyIdx     int      // Current position in history (-1 = not browsing)
	historySavedIn string   // Saved current input when entering history mode

	// Help overlay
	showHelpOverlay bool

	// Progress tracking
	progressInfo *ProgressInfo

	// Plan block tracking (for dynamic step updates)
	planBlockID int // Unique BlockID of the plan block (0 if none)
}

// NewModel creates a new TUI model
func NewModel(modelName string, streamChan chan StreamMsg) Model {
	ta := textarea.New()
	ta.Placeholder = "Type message..."
	ta.Prompt = "" // We render our own prompt in the footer
	ta.Focus()
	ta.CharLimit = 0          // No limit
	ta.MaxHeight = 5          // Grow up to 5 lines
	ta.ShowLineNumbers = false // Clean look
	ta.SetWidth(50)
	ta.SetHeight(1) // Start as single line

	// Apply Nord theme styling to textarea
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(nord8)                    // Cyan cursor
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(colorMuted)    // Readable placeholder
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(nord4)               // Primary text
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()                           // No highlight on current line
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(colorMuted)   // Readable placeholder
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(colorMuted)          // Muted when blurred

	blocks := make([]ContentBlock, 0)
	return Model{
		state:         StateStarting,
		modelName:     modelName,
		blocks:        &blocks,            // Pointer survives model copies
		streaming:     &strings.Builder{}, // Pointer survives model copies
		streamChan:    streamChan,
		resultChan:    make(chan PermissionResult, 1),
		readyChan:     make(chan struct{}),
		doneChan:      make(chan struct{}),
		textArea:      ta,
		maxIterations: 20,
		interruptChan:      make(chan struct{}, 1),
		forceInterruptChan: make(chan struct{}, 1),
		inputQueue:    make([]string, 0, 10),
		maxQueueSize:  10,
		callbacks:     &modelCallbacks{}, // Pointer survives copy to tea.Program
		agentMode:     ModeBuild,         // Default to full execution mode
		completer:     NewCompleter(),    // Pointer survives model copies
		activeTools:   make(map[string]*ToolBlockMeta),
		historyIdx:  -1,
	}
}

// SetOnReady sets the callback for when TUI is ready
func (m *Model) SetOnReady(fn func()) {
	logTUIDebug("SetOnReady: callbacks=%p, hasCallback=%v", m.callbacks, fn != nil)
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
		textarea.Blink,
		tickCmd(), // Start spinner for loading animation
	)
}

// waitForStream returns a command that waits for stream messages.
// Uses a select with doneChan to prevent goroutine leaks on quit.
func (m Model) waitForStream() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-m.streamChan:
			if !ok {
				return QuitMsg{}
			}
			return msg
		case <-m.doneChan:
			return QuitMsg{}
		}
	}
}

// signalReady signals that the TUI is ready and calls the onReady callback
func (m Model) signalReady() tea.Cmd {
	// Capture callbacks pointer to ensure we use the shared instance
	callbacks := m.callbacks
	logTUIDebug("signalReady: callbacks=%p, hasOnReady=%v", callbacks, callbacks != nil && callbacks.onReady != nil)
	return func() tea.Msg {
		logTUIDebug("signalReady cmd executing")
		// Close ready channel to signal ready (non-blocking for multiple listeners)
		select {
		case <-m.readyChan:
			logTUIDebug("signalReady: readyChan already closed")
		default:
			close(m.readyChan)
			logTUIDebug("signalReady: closed readyChan")
		}
		// Call onReady callback if set
		if callbacks != nil && callbacks.onReady != nil {
			logTUIDebug("signalReady: calling onReady callback")
			callbacks.onReady()
			logTUIDebug("signalReady: onReady callback returned")
		} else {
			logTUIDebug("signalReady: no onReady callback set")
		}
		return nil
	}
}

// maxBlocks is the maximum number of content blocks to retain in the TUI.
const maxBlocks = 500

// AddBlock adds a content block to the conversation
func (m *Model) AddBlock(block ContentBlock) {
	m.nextBlockID++
	block.BlockID = m.nextBlockID

	// Auto-collapse tool results (not errors) and thinking blocks when toggle is on
	if m.toolsCollapsed {
		if (block.Type == BlockToolResult && !block.IsError) || block.Type == BlockThinking {
			block.Collapsed = true
		}
	}

	// Generate summary for collapsible blocks
	if block.Type == BlockToolResult && !block.IsError {
		lines := strings.Count(block.Content, "\n") + 1
		block.Summary = fmt.Sprintf("%d lines, %s", lines, formatByteCount(len(block.Content)))
	} else if block.Type == BlockThinking {
		block.Summary = fmt.Sprintf("%d chars", len(block.Content))
	}

	*m.blocks = append(*m.blocks, block)

	// Cap block history to prevent unbounded memory growth
	if len(*m.blocks) > maxBlocks {
		trimCount := len(*m.blocks) - maxBlocks
		*m.blocks = (*m.blocks)[trimCount:]
		// Reset plan block ID if the plan block was trimmed
		planFound := false
		for _, b := range *m.blocks {
			if b.BlockID == m.planBlockID {
				planFound = true
				break
			}
		}
		if !planFound {
			m.planBlockID = 0
		}
	}

	m.updateViewportContent()
	if m.userScrolledUp {
		m.newContentPending = true
	} else {
		m.scrollToBottom()
	}
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

// GetForceInterruptChan returns the force interrupt channel (second ESC)
func (m *Model) GetForceInterruptChan() chan struct{} {
	return m.forceInterruptChan
}

// GetSessionStats returns current session statistics
func (m *Model) GetSessionStats() SessionStats {
	return SessionStats{
		LoopIteration: m.loopIteration,
		MaxIterations: m.maxIterations,
		LoopStartTime: m.loopStartTime,
		InputTokens:   m.inputTokens,
		OutputTokens:  m.outputTokens,
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

// SetAgentMode sets the current agent mode
func (m *Model) SetAgentMode(mode AgentMode) {
	m.agentMode = mode
}

// GetAgentMode returns the current agent mode
func (m *Model) GetAgentMode() AgentMode {
	return m.agentMode
}

// CycleAgentMode advances to the next mode: Ask → Plan → Build → Ask
func (m *Model) CycleAgentMode() AgentMode {
	m.agentMode = m.agentMode.Next()
	if m.callbacks != nil && m.callbacks.onModeChange != nil {
		m.callbacks.onModeChange(m.agentMode)
	}
	return m.agentMode
}

// SetModeChangeCallback sets the callback for mode changes
func (m *Model) SetModeChangeCallback(fn func(AgentMode)) {
	m.callbacks.onModeChange = fn
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
