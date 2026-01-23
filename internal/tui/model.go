package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// AppState represents the current state of the application
type AppState int

const (
	StateIdle       AppState = iota // Waiting for user input
	StateStreaming                  // Streaming LLM response
	StatePermission                 // Waiting for permission input
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

// Model is the main Bubble Tea model for the TUI
type Model struct {
	// Dimensions
	width  int
	height int

	// State
	state     AppState
	modelName string
	ready     bool

	// Content
	blocks    []ContentBlock
	streaming strings.Builder

	// Components
	viewport  viewport.Model
	textInput textinput.Model

	// Channels
	streamChan chan StreamMsg
	resultChan chan PermissionResult

	// Permission state
	permToolName    string
	permLevel       string
	permDescription string

	// Spinner state
	spinnerActive bool
	spinnerFrame  int

	// Activity indicator state
	activityMessage string // Current activity message (e.g., "Thinking...", "Running: bash")

	// Callback for query submission
	onSubmit func(string)

	// Quit signal
	quitting bool
}

// NewModel creates a new TUI model
func NewModel(modelName string, streamChan chan StreamMsg) Model {
	ti := textinput.New()
	ti.Placeholder = "Type message..."
	ti.Focus()
	ti.CharLimit = 0 // No limit
	ti.Width = 50

	return Model{
		state:      StateIdle,
		modelName:  modelName,
		blocks:     []ContentBlock{},
		streamChan: streamChan,
		resultChan: make(chan PermissionResult, 1),
		textInput:  ti,
	}
}

// SetSubmitCallback sets the callback for when user submits a query
func (m *Model) SetSubmitCallback(fn func(string)) {
	m.onSubmit = fn
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.waitForStream(),
	)
}

// waitForStream returns a command that waits for stream messages
func (m Model) waitForStream() tea.Cmd {
	return func() tea.Msg {
		msg := <-m.streamChan
		return msg
	}
}

// AddBlock adds a content block to the conversation
func (m *Model) AddBlock(block ContentBlock) {
	m.blocks = append(m.blocks, block)
	m.updateViewportContent()
	m.scrollToBottom()
}

// ClearBlocks clears all content blocks
func (m *Model) ClearBlocks() {
	m.blocks = []ContentBlock{}
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
