package agent

import "github.com/abdul-hamid-achik/vecai/internal/tui"

// TUICommandContext implements CommandContext for the TUI environment.
// It wraps a TUIRunner to provide access to the viewport, message queue,
// conversation text, and agent mode switching.
type TUICommandContext struct {
	Runner *tui.TUIRunner
}

// Verify interface compliance at compile time.
var _ CommandContext = (*TUICommandContext)(nil)

// ClearDisplay clears the TUI viewport.
func (t *TUICommandContext) ClearDisplay() {
	t.Runner.GetAdapter().Clear()
}

// ClearQueue clears the TUI message queue.
func (t *TUICommandContext) ClearQueue() {
	t.Runner.ClearQueue()
}

// GetConversationText returns the full conversation text from the TUI model.
func (t *TUICommandContext) GetConversationText() string {
	return t.Runner.GetConversationText()
}

// SetAgentMode changes the agent mode in the TUI model.
func (t *TUICommandContext) SetAgentMode(mode tui.AgentMode) {
	t.Runner.GetModel().SetAgentMode(mode)
}

// SetSessionID updates the session ID displayed in the TUI header.
func (t *TUICommandContext) SetSessionID(id string) {
	t.Runner.GetAdapter().SetSessionID(id)
}

// GetTUIAdapter returns the underlying TUI adapter.
func (t *TUICommandContext) GetTUIAdapter() *tui.TUIAdapter {
	return t.Runner.GetAdapter()
}
