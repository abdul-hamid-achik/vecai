package agent

import (
	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/tui"
)

// AgentOutput is the unified interface for agent output.
// Both CLIOutput and TUIOutput implement this.
type AgentOutput interface {
	// Streaming
	StreamText(text string)
	StreamThinking(text string)
	StreamDone()
	StreamDoneWithUsage(inputTokens, outputTokens int64)

	// Messages
	Text(text string)
	TextLn(text string)
	Error(err error)
	ErrorStr(msg string)
	Warning(msg string)
	Success(msg string)
	Info(msg string)

	// Tools
	ToolCall(name, description string)
	ToolResult(name, result string, isError bool)

	// Prompts
	PermissionPrompt(toolName string, level tools.PermissionLevel, description string)

	// Formatting
	Header(text string)
	Separator()
	Thinking(text string)
	ThinkingLn(text string)

	// Model & Activity
	ModelInfo(model string)
	Activity(msg string)
	Done()
}

// AgentInput is the unified interface for user input.
type AgentInput interface {
	ReadLine(prompt string) (string, error)
	Confirm(prompt string, defaultYes bool) (bool, error)
}

// InterruptSupport is optionally implemented by outputs that support interruption.
type InterruptSupport interface {
	GetInterruptChan() <-chan struct{}
}

// ForceInterruptSupport is optionally implemented by outputs that support force-stop (double ESC).
type ForceInterruptSupport interface {
	GetForceInterruptChan() <-chan struct{}
}

// PlanSupport is optionally implemented by outputs that can display live-updating plans.
type PlanSupport interface {
	Plan(text string)
	PlanUpdate(text string)
}

// StatsSupport is optionally implemented by outputs that display stats.
type StatsSupport interface {
	UpdateContextStats(usagePercent float64, usedTokens, contextWindow int, needsWarning bool)
	SetSessionID(sessionID string)
	UpdateStats(stats tui.SessionStats)
	Clear()
}
