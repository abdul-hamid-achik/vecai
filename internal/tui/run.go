package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/logger"
	tea "github.com/charmbracelet/bubbletea"
)

// runLog is a prefixed logger for TUI runner events
var runLog = logger.WithPrefix("TUI-RUN")

// RunConfig contains configuration for running the TUI
type RunConfig struct {
	ModelName string
	OnQuery   func(query string) error
	OnCommand func(cmd string) bool // Returns false to exit
}

// Run starts the TUI and blocks until it exits
func Run(cfg RunConfig) error {
	// Check if we should use TUI
	if !IsTTYAvailable() {
		return fmt.Errorf("TUI mode requires a terminal")
	}

	// Create stream channel
	streamChan := make(chan StreamMsg, 100)

	// Create model
	model := NewModel(cfg.ModelName, streamChan)

	// Create program (no mouse capture to allow native text selection)
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	// Create adapter
	adapter := NewTUIAdapter(program, streamChan, model.resultChan, model.interruptChan)

	// Set up submit callback
	model.SetSubmitCallback(func(input string) {
		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			if !cfg.OnCommand(input) {
				// Exit requested
				program.Send(QuitMsg{})
				return
			}
			return
		}

		// Run query
		if err := cfg.OnQuery(input); err != nil {
			adapter.Error(err)
		}
	})

	// Add initial info block
	streamChan <- NewInfoMsg("Type /help for commands, /exit to quit")

	// Run program
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}

// IsTTYAvailable checks if the terminal supports TUI mode
func IsTTYAvailable() bool {
	// Check if stdout is a terminal
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	// Check NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	return true
}

// TUIRunner provides a way to run the TUI with access to the adapter
type TUIRunner struct {
	model        *Model
	program      *tea.Program
	adapter      *TUIAdapter
	streamChan   chan StreamMsg
	initialQuery string   // Query to execute after TUI is ready
	onReady      func()   // Callback when TUI is ready
	interactive  bool     // Whether to stay open for follow-ups
}

// NewTUIRunner creates a new TUI runner
func NewTUIRunner(modelName string) *TUIRunner {
	runLog.Debug("NewTUIRunner: creating with model=%s", modelName)
	streamChan := make(chan StreamMsg, 100)
	model := NewModel(modelName, streamChan)
	runLog.Debug("NewTUIRunner: model created, callbacks=%p", model.callbacks)

	// No mouse capture to allow native text selection
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)
	runLog.Debug("NewTUIRunner: tea.Program created")

	adapter := NewTUIAdapter(program, streamChan, model.resultChan, model.interruptChan)

	runner := &TUIRunner{
		model:      &model,
		program:    program,
		adapter:    adapter,
		streamChan: streamChan,
	}
	runLog.Debug("NewTUIRunner: runner created, model.callbacks=%p", runner.model.callbacks)
	return runner
}

// GetAdapter returns the TUI adapter for use as OutputHandler
func (r *TUIRunner) GetAdapter() *TUIAdapter {
	return r.adapter
}

// GetModel returns the underlying TUI model for direct state access
func (r *TUIRunner) GetModel() *Model {
	return r.model
}

// SetSubmitCallback sets the callback for when user submits a query
func (r *TUIRunner) SetSubmitCallback(fn func(string)) {
	r.model.SetSubmitCallback(fn)
}

// SetOnReady sets a callback to be called when the TUI is ready
func (r *TUIRunner) SetOnReady(fn func()) {
	runLog.Debug("SetOnReady: setting callback, model.callbacks=%p", r.model.callbacks)
	r.onReady = fn
	r.model.SetOnReady(fn)
	runLog.Debug("SetOnReady: callback set, model.callbacks.onReady=%v", r.model.callbacks.onReady != nil)
}

// SetInitialQuery sets a query to execute after TUI is ready
func (r *TUIRunner) SetInitialQuery(query string) {
	r.initialQuery = query
}

// SetInteractive sets whether to stay open for follow-ups after initial query
func (r *TUIRunner) SetInteractive(interactive bool) {
	r.interactive = interactive
}

// GetReadyChan returns a channel that closes when TUI is ready
func (r *TUIRunner) GetReadyChan() <-chan struct{} {
	return r.model.GetReadyChan()
}

// AddUserBlock adds a user message block to the TUI
func (r *TUIRunner) AddUserBlock(content string) {
	r.streamChan <- NewUserMsg(content)
}

// SendInfo sends an info message to the TUI
func (r *TUIRunner) SendInfo(msg string) {
	r.streamChan <- NewInfoMsg(msg)
}

// SendSuccess sends a success message to the TUI
func (r *TUIRunner) SendSuccess(msg string) {
	r.streamChan <- NewSuccessMsg(msg)
}

// SendWarning sends a warning message to the TUI
func (r *TUIRunner) SendWarning(msg string) {
	r.streamChan <- NewWarningMsg(msg)
}

// SendError sends an error message to the TUI
func (r *TUIRunner) SendError(msg string) {
	r.streamChan <- NewErrorMsg(msg)
}

// Quit signals the TUI to quit
func (r *TUIRunner) Quit() {
	r.program.Send(QuitMsg{})
}

// ClearQueue clears all queued inputs
func (r *TUIRunner) ClearQueue() {
	r.model.ClearQueue()
}

// GetConversationText returns the conversation as plain text for copying
func (r *TUIRunner) GetConversationText() string {
	return r.model.GetConversationText()
}

// Run starts the TUI and blocks until it exits
func (r *TUIRunner) Run() error {
	runLog.Debug("Run: starting TUI program, model.callbacks=%p, hasOnReady=%v",
		r.model.callbacks, r.model.callbacks != nil && r.model.callbacks.onReady != nil)
	if _, err := r.program.Run(); err != nil {
		runLog.Debug("Run: TUI exited with error: %v", err)
		return fmt.Errorf("error running TUI: %w", err)
	}
	runLog.Debug("Run: TUI exited normally")
	return nil
}
