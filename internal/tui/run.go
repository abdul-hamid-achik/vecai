package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

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

	// Create program
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
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
	model      *Model
	program    *tea.Program
	adapter    *TUIAdapter
	streamChan chan StreamMsg
}

// NewTUIRunner creates a new TUI runner
func NewTUIRunner(modelName string) *TUIRunner {
	streamChan := make(chan StreamMsg, 100)
	model := NewModel(modelName, streamChan)

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	adapter := NewTUIAdapter(program, streamChan, model.resultChan, model.interruptChan)

	return &TUIRunner{
		model:      &model,
		program:    program,
		adapter:    adapter,
		streamChan: streamChan,
	}
}

// GetAdapter returns the TUI adapter for use as OutputHandler
func (r *TUIRunner) GetAdapter() *TUIAdapter {
	return r.adapter
}

// SetSubmitCallback sets the callback for when user submits a query
func (r *TUIRunner) SetSubmitCallback(fn func(string)) {
	r.model.SetSubmitCallback(fn)
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

// Run starts the TUI and blocks until it exits
func (r *TUIRunner) Run() error {
	if _, err := r.program.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	return nil
}
