package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/logging"
	tea "github.com/charmbracelet/bubbletea"
)

// runLog is a prefixed logger for TUI runner events (lazy-init)
var runLog *logging.Logger

// getRunLog returns the prefixed logger, initializing lazily if needed
func getRunLog() *logging.Logger {
	if runLog == nil {
		if log := logging.Global(); log != nil {
			runLog = log.WithPrefix("TUI-RUN")
		}
	}
	return runLog
}

// logDebug logs a debug message with printf-style formatting
func logDebug(format string, args ...any) {
	if log := getRunLog(); log != nil {
		log.Debug(fmt.Sprintf(format, args...))
	}
}

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
	streamChan := make(chan StreamMsg, 500)

	// Create model
	model := NewModel(cfg.ModelName, streamChan)

	// Create program (no mouse capture to allow native text selection)
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	// Create adapter
	adapter := NewTUIAdapter(program, streamChan, model.resultChan, model.interruptChan, model.forceInterruptChan)

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

	// Run program
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}

// IsTTYAvailable checks if the terminal supports TUI mode.
// Note: NO_COLOR is handled separately via IsNoColor() — it disables colors
// but should not prevent the TUI from launching.
func IsTTYAvailable() bool {
	// Check if stdout is a terminal
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	return true
}

// IsNoColor returns true if the NO_COLOR environment variable is set,
// indicating that color output should be suppressed.
func IsNoColor() bool {
	return os.Getenv("NO_COLOR") != ""
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
	logDebug("NewTUIRunner: creating with model=%s", modelName)
	streamChan := make(chan StreamMsg, 500)
	model := NewModel(modelName, streamChan)
	logDebug("NewTUIRunner: model created, callbacks=%p", model.callbacks)

	// No mouse capture to allow native text selection
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)
	logDebug("NewTUIRunner: tea.Program created")

	adapter := NewTUIAdapter(program, streamChan, model.resultChan, model.interruptChan, model.forceInterruptChan)

	runner := &TUIRunner{
		model:      &model,
		program:    program,
		adapter:    adapter,
		streamChan: streamChan,
	}
	logDebug("NewTUIRunner: runner created, model.callbacks=%p", runner.model.callbacks)
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
	logDebug("SetOnReady: setting callback, model.callbacks=%p", r.model.callbacks)
	r.onReady = fn
	r.model.SetOnReady(fn)
	logDebug("SetOnReady: callback set, model.callbacks.onReady=%v", r.model.callbacks.onReady != nil)
}

// SetInitialQuery sets a query to execute after TUI is ready
func (r *TUIRunner) SetInitialQuery(query string) {
	r.initialQuery = query
}

// SetInteractive sets whether to stay open for follow-ups after initial query
func (r *TUIRunner) SetInteractive(interactive bool) {
	r.interactive = interactive
}

// SetModeChangeCallback sets a callback for when the user changes modes
func (r *TUIRunner) SetModeChangeCallback(fn func(AgentMode)) {
	r.model.SetModeChangeCallback(fn)
}

// AddSkillCommands adds skill-based commands to the autocomplete dropdown
func (r *TUIRunner) AddSkillCommands(cmds []CommandDef) {
	r.model.AddSkillCommands(cmds)
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

// GetTaggedFiles returns the currently tagged files
func (r *TUIRunner) GetTaggedFiles() []TaggedFile {
	return r.model.GetTaggedFiles()
}

// ClearTaggedFiles removes all tagged files
func (r *TUIRunner) ClearTaggedFiles() {
	r.model.ClearTaggedFiles()
}

// SetProjectRoot sets the project root for vecgrep async searches
func (r *TUIRunner) SetProjectRoot(root string) {
	r.model.SetProjectRoot(root)
}

// GetConversationText returns the conversation as plain text for copying
func (r *TUIRunner) GetConversationText() string {
	return r.model.GetConversationText()
}

// Run starts the TUI and blocks until it exits
func (r *TUIRunner) Run() error {
	logDebug("Run: starting TUI program, model.callbacks=%p, hasOnReady=%v",
		r.model.callbacks, r.model.callbacks != nil && r.model.callbacks.onReady != nil)
	if _, err := r.program.Run(); err != nil {
		logDebug("Run: TUI exited with error: %v", err)
		return fmt.Errorf("error running TUI: %w", err)
	}
	logDebug("Run: TUI exited normally")
	return nil
}
