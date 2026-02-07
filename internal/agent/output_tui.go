package agent

import (
	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/tui"
)

// TUIOutput wraps tui.TUIAdapter to satisfy AgentOutput, AgentInput,
// InterruptSupport, and StatsSupport.
type TUIOutput struct {
	Adapter *tui.TUIAdapter
}

// Verify interface compliance at compile time.
var _ AgentOutput = (*TUIOutput)(nil)
var _ AgentInput = (*TUIOutput)(nil)
var _ InterruptSupport = (*TUIOutput)(nil)
var _ StatsSupport = (*TUIOutput)(nil)

// --- AgentOutput: Streaming ---

func (t *TUIOutput) StreamText(text string)    { t.Adapter.StreamText(text) }
func (t *TUIOutput) StreamThinking(text string) { t.Adapter.StreamThinking(text) }
func (t *TUIOutput) StreamDone()                { t.Adapter.StreamDone() }
func (t *TUIOutput) StreamDoneWithUsage(inputTokens, outputTokens int64) {
	t.Adapter.StreamDoneWithUsage(inputTokens, outputTokens)
}

// --- AgentOutput: Messages ---

func (t *TUIOutput) Text(text string)     { t.Adapter.Text(text) }
func (t *TUIOutput) TextLn(text string)   { t.Adapter.TextLn(text) }
func (t *TUIOutput) Error(err error)      { t.Adapter.Error(err) }
func (t *TUIOutput) ErrorStr(msg string)  { t.Adapter.ErrorStr(msg) }
func (t *TUIOutput) Warning(msg string)   { t.Adapter.Warning(msg) }
func (t *TUIOutput) Success(msg string)   { t.Adapter.Success(msg) }
func (t *TUIOutput) Info(msg string)      { t.Adapter.Info(msg) }

// --- AgentOutput: Tools ---

func (t *TUIOutput) ToolCall(name, description string)            { t.Adapter.ToolCall(name, description) }
func (t *TUIOutput) ToolResult(name, result string, isError bool) { t.Adapter.ToolResult(name, result, isError) }

// --- AgentOutput: Prompts ---

func (t *TUIOutput) PermissionPrompt(toolName string, level tools.PermissionLevel, description string) {
	t.Adapter.PermissionPrompt(toolName, level, description)
}

// --- AgentOutput: Formatting ---

func (t *TUIOutput) Header(text string)     { t.Adapter.Header(text) }
func (t *TUIOutput) Separator()             { t.Adapter.Separator() }
func (t *TUIOutput) Thinking(text string)   { t.Adapter.Thinking(text) }
func (t *TUIOutput) ThinkingLn(text string) { t.Adapter.ThinkingLn(text) }

// --- AgentOutput: Model & Activity ---

func (t *TUIOutput) ModelInfo(model string) { t.Adapter.ModelInfo(model) }
func (t *TUIOutput) Activity(msg string)    { t.Adapter.Activity(msg) }
func (t *TUIOutput) Done()                  { t.Adapter.Done() }

// --- AgentInput ---

func (t *TUIOutput) ReadLine(prompt string) (string, error) { return t.Adapter.ReadLine(prompt) }
func (t *TUIOutput) Confirm(prompt string, defaultYes bool) (bool, error) {
	return t.Adapter.Confirm(prompt, defaultYes)
}

// --- InterruptSupport ---

func (t *TUIOutput) GetInterruptChan() <-chan struct{} { return t.Adapter.GetInterruptChan() }

// --- StatsSupport ---

func (t *TUIOutput) UpdateContextStats(usagePercent float64, usedTokens, contextWindow int, needsWarning bool) {
	t.Adapter.UpdateContextStats(usagePercent, usedTokens, contextWindow, needsWarning)
}

func (t *TUIOutput) SetSessionID(sessionID string) { t.Adapter.SetSessionID(sessionID) }
func (t *TUIOutput) UpdateStats(stats tui.SessionStats) { t.Adapter.UpdateStats(stats) }
func (t *TUIOutput) Clear()                             { t.Adapter.Clear() }
