package agent

import (
	"github.com/abdul-hamid-achik/vecai/internal/tui"
	"github.com/abdul-hamid-achik/vecai/internal/ui"
)

// CLICommandContext implements CommandContext for the CLI (non-TUI) environment.
// Most TUI-specific operations are no-ops since the CLI doesn't have a
// viewport, message queue, or agent mode switching.
type CLICommandContext struct {
	Input *ui.InputHandler
}

// Verify interface compliance at compile time.
var _ CommandContext = (*CLICommandContext)(nil)

// ClearDisplay clears the terminal input line/history.
func (c *CLICommandContext) ClearDisplay() {
	if c.Input != nil {
		c.Input.Clear()
	}
}

// ClearQueue is a no-op for CLI mode (no message queue).
func (c *CLICommandContext) ClearQueue() {}

// GetConversationText returns empty string for CLI mode.
// The CLI doesn't maintain a conversation view that can be copied.
func (c *CLICommandContext) GetConversationText() string {
	return ""
}

// SetAgentMode is a no-op for CLI mode (no display state to update).
func (c *CLICommandContext) SetAgentMode(_ tui.AgentMode) {}

// SetSessionID is a no-op for CLI mode (no header to update).
func (c *CLICommandContext) SetSessionID(_ string) {}

// GetTUIAdapter returns nil for CLI mode.
func (c *CLICommandContext) GetTUIAdapter() *tui.TUIAdapter {
	return nil
}
