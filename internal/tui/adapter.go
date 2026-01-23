package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// TUIAdapter bridges the TUI with the agent's OutputHandler and InputHandler interfaces
type TUIAdapter struct {
	program       *tea.Program
	streamChan    chan StreamMsg
	resultChan    chan PermissionResult
	interruptChan chan struct{}
	useColors     bool
}

// NewTUIAdapter creates a new TUI adapter
func NewTUIAdapter(program *tea.Program, streamChan chan StreamMsg, resultChan chan PermissionResult, interruptChan chan struct{}) *TUIAdapter {
	return &TUIAdapter{
		program:       program,
		streamChan:    streamChan,
		resultChan:    resultChan,
		interruptChan: interruptChan,
		useColors:     true,
	}
}

// OutputHandler interface implementation

// Text outputs regular text
func (a *TUIAdapter) Text(text string) {
	a.streamChan <- NewInfoMsg(text)
}

// TextLn outputs regular text with newline
func (a *TUIAdapter) TextLn(text string) {
	a.streamChan <- NewInfoMsg(text)
}

// StreamText outputs streaming text without newline
func (a *TUIAdapter) StreamText(text string) {
	a.streamChan <- NewTextMsg(text)
}

// StreamThinking outputs streaming thinking text
func (a *TUIAdapter) StreamThinking(text string) {
	a.streamChan <- NewThinkingMsg(text)
}

// StreamDone signals end of streaming
func (a *TUIAdapter) StreamDone() {
	a.streamChan <- NewDoneMsg()
}

// StreamDoneWithUsage signals end of streaming with token usage
func (a *TUIAdapter) StreamDoneWithUsage(inputTokens, outputTokens int64) {
	a.streamChan <- NewDoneMsgWithUsage(inputTokens, outputTokens)
}

// UpdateStats sends a stats update message
func (a *TUIAdapter) UpdateStats(stats SessionStats) {
	a.streamChan <- NewStatsMsg(stats)
}

// GetInterruptChan returns the interrupt channel for ESC handling
func (a *TUIAdapter) GetInterruptChan() <-chan struct{} {
	return a.interruptChan
}

// ToolCall outputs a tool call notification
func (a *TUIAdapter) ToolCall(name string, description string) {
	// Send activity update first
	activityMsg := fmt.Sprintf("Running: %s", name)
	if description != "" && len(description) <= 30 {
		activityMsg = fmt.Sprintf("Running: %s - %s", name, description)
	} else if description != "" {
		activityMsg = fmt.Sprintf("Running: %s - %s...", name, description[:27])
	}
	a.streamChan <- NewActivityMsg(activityMsg)

	// Then send tool call block
	a.streamChan <- NewToolCallMsg(name, description)
}

// ToolResult outputs a tool result
func (a *TUIAdapter) ToolResult(name string, result string, isError bool) {
	a.streamChan <- NewToolResultMsg(name, result, isError)
}

// Error outputs an error message
func (a *TUIAdapter) Error(err error) {
	a.streamChan <- NewErrorMsg(err.Error())
}

// ErrorStr outputs an error string
func (a *TUIAdapter) ErrorStr(msg string) {
	a.streamChan <- NewErrorMsg(msg)
}

// Warning outputs a warning message
func (a *TUIAdapter) Warning(msg string) {
	a.streamChan <- NewWarningMsg(msg)
}

// Success outputs a success message
func (a *TUIAdapter) Success(msg string) {
	a.streamChan <- NewSuccessMsg(msg)
}

// Info outputs an info message
func (a *TUIAdapter) Info(msg string) {
	a.streamChan <- NewInfoMsg(msg)
}

// Done outputs a completion message
func (a *TUIAdapter) Done() {
	a.streamChan <- NewDoneMsg()
}

// Header outputs a header (rendered as info in TUI)
func (a *TUIAdapter) Header(text string) {
	a.streamChan <- NewInfoMsg(text)
}

// Separator outputs a horizontal line (no-op in TUI)
func (a *TUIAdapter) Separator() {
	// No-op in TUI mode
}

// ModelInfo outputs the current model info
func (a *TUIAdapter) ModelInfo(model string) {
	a.streamChan <- NewModelInfoMsg(model)
}

// PermissionPrompt outputs a permission prompt
func (a *TUIAdapter) PermissionPrompt(toolName string, level tools.PermissionLevel, description string) {
	a.streamChan <- NewPermissionMsg(toolName, level, description)
}

// Question outputs a question (rendered as info in TUI)
func (a *TUIAdapter) Question(question string, options []string) {
	a.streamChan <- NewInfoMsg(question)
	for i, opt := range options {
		a.streamChan <- NewInfoMsg(fmt.Sprintf("  [%d] %s", i+1, opt))
	}
}

// Thinking outputs thinking text
func (a *TUIAdapter) Thinking(text string) {
	a.streamChan <- NewThinkingMsg(text)
}

// ThinkingLn outputs thinking text with newline
func (a *TUIAdapter) ThinkingLn(text string) {
	a.streamChan <- NewThinkingMsg(text)
}

// Prompt outputs a prompt (no-op in TUI, input handled separately)
func (a *TUIAdapter) Prompt(prompt string) {
	// No-op in TUI mode
}

// IsTTY returns true (TUI is always TTY)
func (a *TUIAdapter) IsTTY() bool {
	return true
}

// UseColors returns true if colors are enabled
func (a *TUIAdapter) UseColors() bool {
	return a.useColors
}

// InputHandler interface implementation

// ReadLine reads a single line of input
// In TUI mode, this blocks until the user submits input via the permission prompt
func (a *TUIAdapter) ReadLine(prompt string) (string, error) {
	// Wait for permission result
	result := <-a.resultChan
	return result.Decision, nil
}

// ReadInput reads user input (handled by TUI event loop)
func (a *TUIAdapter) ReadInput(prompt string) (string, error) {
	// This is handled by the TUI event loop and submit callback
	// This method should not be called in TUI mode
	return "", nil
}

// ReadMultiLine reads multiple lines (not used in TUI mode)
func (a *TUIAdapter) ReadMultiLine(prompt string) (string, error) {
	return "", nil
}

// Confirm asks for a yes/no confirmation
func (a *TUIAdapter) Confirm(prompt string, defaultYes bool) (bool, error) {
	// Wait for permission result
	result := <-a.resultChan
	return result.Decision == "y" || result.Decision == "yes", nil
}

// Select presents options (not fully supported in TUI mode)
func (a *TUIAdapter) Select(prompt string, options []string) (int, error) {
	return 0, nil
}

// Clear clears the conversation
func (a *TUIAdapter) Clear() {
	a.streamChan <- NewClearMsg()
}
