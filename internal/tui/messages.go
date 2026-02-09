package tui

import (
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// SessionStats contains statistics about the current agent session
type SessionStats struct {
	LoopIteration  int           // Current iteration number
	MaxIterations  int           // Maximum allowed iterations
	LoopStartTime  time.Time     // When the current loop started
	InputTokens    int64         // Total input tokens used
	OutputTokens   int64         // Total output tokens used
}

// RateLimitInfo contains information about the current rate limit state
type RateLimitInfo struct {
	Duration    time.Duration // Remaining time to wait
	Reason      string        // Why we're rate limited
	Attempt     int           // Current retry attempt (1-based, 0 if not a retry)
	MaxAttempts int           // Maximum retry attempts (0 if not a retry)
}

// StreamMsg represents a streaming message from the LLM
type StreamMsg struct {
	Type          string // "text", "thinking", "done", "tool_call", "tool_result", "error", "info", "warning", "success", "permission", "clear", "model_info", "stats", "rate_limit", "rate_limit_clear", "context_stats", "mode_change"
	Text          string
	ToolName      string
	ToolDesc      string
	IsError       bool
	GroupID       string                // Links tool_call to tool_result
	Level         tools.PermissionLevel
	Stats         *SessionStats     // Session stats (only for "stats" type)
	Usage         *TokenUsage       // Token usage (only for "done" type)
	RateLimitInfo *RateLimitInfo    // Rate limit info (only for "rate_limit" type)
	ContextStats  *ContextStatsInfo // Context stats (only for "context_stats" type)
	ProjectInfo   *ProjectInfo      // Project info (only for "project_info" type)
	ProgressData  *ProgressInfo     // Progress info (only for "progress" type)
	ModeInfo      *AgentMode        // Agent mode (only for "mode_change" type)
}

// TokenUsage represents token counts from API response
type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
}

// TickMsg is sent for spinner animation
type TickMsg struct{}

// RenderTickMsg is sent to trigger a debounced markdown re-render during streaming
type RenderTickMsg struct{}

// PermissionResponseMsg is sent when user responds to a permission prompt
type PermissionResponseMsg struct {
	Decision string
}

// QuitMsg signals the TUI to quit
type QuitMsg struct{}

// ClearMsg signals to clear the conversation
type ClearMsg struct{}

// Message constructors for the adapter

// NewTextMsg creates a text stream message
func NewTextMsg(text string) StreamMsg {
	return StreamMsg{Type: "text", Text: text}
}

// NewThinkingMsg creates a thinking stream message
func NewThinkingMsg(text string) StreamMsg {
	return StreamMsg{Type: "thinking", Text: text}
}

// NewDoneMsg creates a done stream message
func NewDoneMsg() StreamMsg {
	return StreamMsg{Type: "done"}
}

// NewDoneMsgWithUsage creates a done stream message with token usage
func NewDoneMsgWithUsage(inputTokens, outputTokens int64) StreamMsg {
	return StreamMsg{
		Type: "done",
		Usage: &TokenUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
	}
}

// NewToolCallMsg creates a tool call message
func NewToolCallMsg(name, description string) StreamMsg {
	return StreamMsg{Type: "tool_call", ToolName: name, ToolDesc: description}
}

// NewToolResultMsg creates a tool result message
func NewToolResultMsg(name, result string, isError bool) StreamMsg {
	return StreamMsg{Type: "tool_result", ToolName: name, Text: result, IsError: isError}
}

// NewErrorMsg creates an error message
func NewErrorMsg(text string) StreamMsg {
	return StreamMsg{Type: "error", Text: text}
}

// NewInfoMsg creates an info message
func NewInfoMsg(text string) StreamMsg {
	return StreamMsg{Type: "info", Text: text}
}

// NewWarningMsg creates a warning message
func NewWarningMsg(text string) StreamMsg {
	return StreamMsg{Type: "warning", Text: text}
}

// NewSuccessMsg creates a success message
func NewSuccessMsg(text string) StreamMsg {
	return StreamMsg{Type: "success", Text: text}
}

// NewPermissionMsg creates a permission request message
func NewPermissionMsg(toolName string, level tools.PermissionLevel, description string) StreamMsg {
	return StreamMsg{
		Type:     "permission",
		ToolName: toolName,
		Level:    level,
		ToolDesc: description,
	}
}

// NewClearMsg creates a clear message
func NewClearMsg() StreamMsg {
	return StreamMsg{Type: "clear"}
}

// NewModelInfoMsg creates a model info message
func NewModelInfoMsg(model string) StreamMsg {
	return StreamMsg{Type: "model_info", Text: model}
}

// NewUserMsg creates a user message
func NewUserMsg(text string) StreamMsg {
	return StreamMsg{Type: "user", Text: text}
}

// NewActivityMsg creates an activity status message
func NewActivityMsg(message string) StreamMsg {
	return StreamMsg{Type: "activity", Text: message}
}

// NewStatsMsg creates a session statistics message
func NewStatsMsg(stats SessionStats) StreamMsg {
	return StreamMsg{Type: "stats", Stats: &stats}
}

// NewRateLimitMsg creates a rate limit notification message
func NewRateLimitMsg(info RateLimitInfo) StreamMsg {
	return StreamMsg{Type: "rate_limit", RateLimitInfo: &info}
}

// NewRateLimitClearMsg creates a message to clear rate limit status
func NewRateLimitClearMsg() StreamMsg {
	return StreamMsg{Type: "rate_limit_clear"}
}

// ContextStatsInfo contains context usage statistics for TUI display
type ContextStatsInfo struct {
	UsagePercent float64
	UsedTokens   int
	ContextWindow int
	NeedsWarning bool
}

// NewContextStatsMsg creates a context stats update message
func NewContextStatsMsg(info ContextStatsInfo) StreamMsg {
	return StreamMsg{Type: "context_stats", ContextStats: &info}
}

// NewSessionIDMsg creates a session ID update message
func NewSessionIDMsg(sessionID string) StreamMsg {
	return StreamMsg{Type: "session_id", Text: sessionID}
}

// ProjectInfo contains working directory and git branch for the header
type ProjectInfo struct {
	WorkingDir string
	GitBranch  string
}

// ProgressInfo contains progress information for known-length operations
type ProgressInfo struct {
	Current     int    // Current item count
	Total       int    // Total item count
	Description string // What's being done (e.g., "Indexing files")
}

// NewProjectInfoMsg creates a project info update message
func NewProjectInfoMsg(info ProjectInfo) StreamMsg {
	return StreamMsg{Type: "project_info", ProjectInfo: &info}
}

// NewProgressMsg creates a progress update message
func NewProgressMsg(current, total int, description string) StreamMsg {
	return StreamMsg{Type: "progress", ProgressData: &ProgressInfo{
		Current:     current,
		Total:       total,
		Description: description,
	}}
}

// NewProgressClearMsg creates a message to clear the progress bar
func NewProgressClearMsg() StreamMsg {
	return StreamMsg{Type: "progress_clear"}
}

// NewPlanMsg creates a plan display message (rendered as markdown)
func NewPlanMsg(text string) StreamMsg {
	return StreamMsg{Type: "plan", Text: text}
}

// NewPlanUpdateMsg updates an existing plan block with new content
func NewPlanUpdateMsg(text string) StreamMsg {
	return StreamMsg{Type: "plan_update", Text: text}
}

// NewModeChangeMsg creates a mode change message to sync TUI display
func NewModeChangeMsg(mode AgentMode) StreamMsg {
	return StreamMsg{Type: "mode_change", ModeInfo: &mode}
}
