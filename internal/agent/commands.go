package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/logging"
	"github.com/abdul-hamid-achik/vecai/internal/session"
	"github.com/abdul-hamid-achik/vecai/internal/tui"
)

// CommandContext provides command-specific operations beyond basic output.
// Different environments (TUI vs CLI) implement this to handle
// display clearing, queue management, conversation copying, and mode toggling.
type CommandContext interface {
	// ClearDisplay clears the display/conversation view.
	ClearDisplay()
	// ClearQueue clears any pending message queue (TUI only, no-op for CLI).
	ClearQueue()
	// GetConversationText returns the full conversation text for /copy.
	GetConversationText() string
	// SetAgentMode changes the agent mode in the display.
	SetAgentMode(mode tui.AgentMode)
	// SetSessionID updates the displayed session ID.
	SetSessionID(id string)
	// GetTUIAdapter returns the underlying TUI adapter, or nil for CLI.
	// Used by commands that need to pass the adapter to agent methods
	// (e.g., compactConversation, checkVecgrepStatusTUI).
	GetTUIAdapter() *tui.TUIAdapter
}

// CommandHandler handles slash commands with unified output.
type CommandHandler struct {
	agent *Agent
}

// NewCommandHandler creates a new CommandHandler.
func NewCommandHandler(agent *Agent) *CommandHandler {
	return &CommandHandler{agent: agent}
}

// Handle processes a slash command.
// Returns shouldContinue (true = keep running, false = exit requested).
func (ch *CommandHandler) Handle(cmd string, output AgentOutput, cmdCtx CommandContext) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return true
	}

	a := ch.agent

	switch parts[0] {
	case "/help":
		ch.showHelp(output)
		return true

	case "/exit", "/quit":
		output.Info("Goodbye!")
		return false

	case "/clear":
		a.contextMgr.Clear()
		a.shownContextWarning = false
		cmdCtx.ClearDisplay()
		cmdCtx.ClearQueue()
		output.Success("Conversation cleared")
		return true

	case "/copy":
		text := cmdCtx.GetConversationText()
		if text == "" {
			output.Info("No conversation to copy")
			return true
		}
		if err := copyToClipboard(text); err != nil {
			// Fallback: save to file
			filename := fmt.Sprintf("vecai-conversation-%s.txt", time.Now().Format("20060102-150405"))
			if writeErr := os.WriteFile(filename, []byte(text), 0644); writeErr != nil {
				output.ErrorStr("Failed to copy: " + writeErr.Error())
				return true
			}
			output.Success(fmt.Sprintf("Saved conversation to %s", filename))
			return true
		}
		output.Success("Conversation copied to clipboard")
		return true

	case "/context":
		stats := a.contextMgr.GetStats()
		breakdown := a.contextMgr.GetBreakdown()
		output.Info(fmt.Sprintf("Context: %d/%d tokens (%.1f%%)",
			stats.UsedTokens, stats.ContextWindow, stats.UsagePercent*100))
		output.Info(fmt.Sprintf("  System: %d | User: %d | Assistant: %d | Tools: %d",
			breakdown.SystemPrompt, breakdown.UserMessages,
			breakdown.AssistantMsgs, breakdown.ToolResults))
		output.Info(fmt.Sprintf("  Messages: %d", stats.MessageCount))
		if stats.NeedsCompaction {
			output.Warning("Context needs compaction - use /compact")
		} else if stats.NeedsWarning {
			output.Warning("Context is getting full - consider /compact")
		}
		return true

	case "/compact":
		focusPrompt := ""
		if len(parts) > 1 {
			focusPrompt = strings.Join(parts[1:], " ")
		}
		output.Info("Compacting conversation...")
		ctx := context.Background()
		if err := a.compactConversation(ctx, focusPrompt, output); err != nil {
			output.ErrorStr("Compact failed: " + err.Error())
		} else if _, ok := output.(StatsSupport); !ok {
			// CLI mode: compactConversation only prints success for StatsSupport outputs
			newStats := a.contextMgr.GetStats()
			output.Success(fmt.Sprintf("Compacted to %.0f%%", newStats.UsagePercent*100))
		}
		return true

	case "/mode":
		if len(parts) < 2 {
			output.Info("Current model: " + a.llm.GetModel())
			output.Info("Usage: /mode <fast|smart|genius>")
			return true
		}
		switch parts[1] {
		case "fast":
			a.llm.SetTier(config.TierFast)
			output.Success("Switched to fast mode (" + a.config.GetModel(config.TierFast) + ")")
		case "smart":
			a.llm.SetTier(config.TierSmart)
			output.Success("Switched to smart mode (" + a.config.GetModel(config.TierSmart) + ")")
		case "genius":
			a.llm.SetTier(config.TierGenius)
			output.Success("Switched to genius mode (" + a.config.GetModel(config.TierGenius) + ")")
		default:
			output.ErrorStr("Unknown mode: " + parts[1])
		}
		return true

	case "/ask":
		ch.switchMode(tui.ModeAsk, output, cmdCtx)
		return true

	case "/plan":
		if len(parts) < 2 {
			// No args: switch to Plan mode
			ch.switchMode(tui.ModePlan, output, cmdCtx)
			return true
		}
		// With args: run plan command
		goal := strings.Join(parts[1:], " ")
		if cmdCtx.GetTUIAdapter() != nil {
			output.Info("Plan mode not fully supported in TUI yet. Use: vecai plan \"" + goal + "\"")
		} else {
			if err := a.RunPlan(goal); err != nil {
				output.Error(err)
			}
		}
		return true

	case "/build":
		ch.switchMode(tui.ModeBuild, output, cmdCtx)
		return true

	case "/skills":
		ch.showSkills(output)
		return true

	case "/status":
		if adapter := cmdCtx.GetTUIAdapter(); adapter != nil {
			a.checkVecgrepStatusTUI(adapter)
		} else {
			a.checkVecgrepStatus()
		}
		return true

	case "/reindex":
		output.Info("Reindexing codebase with vecgrep...")
		ctx := context.Background()
		result, err := a.reindexVecgrep(ctx)
		if err != nil {
			output.ErrorStr("Reindex failed: " + err.Error())
		} else {
			output.Success(result)
		}
		return true

	case "/sessions":
		ch.showSessions(output)
		return true

	case "/resume":
		ch.resumeSession(parts, output, cmdCtx)
		return true

	case "/new":
		ch.newSession(output, cmdCtx)
		return true

	case "/rewind":
		ch.rewindCheckpoint(output)
		return true

	case "/software-architect", "/architect":
		// Legacy: toggle between Ask and Build for backwards compat
		if a.agentMode == tui.ModeBuild {
			ch.switchMode(tui.ModeAsk, output, cmdCtx)
		} else {
			ch.switchMode(tui.ModeBuild, output, cmdCtx)
		}
		return true

	case "/delete":
		ch.deleteSession(parts, output)
		return true

	default:
		output.ErrorStr("Unknown command: " + parts[0] + ". Type /help for available commands.")
		return true
	}
}

// showHelp displays available commands.
func (ch *CommandHandler) showHelp(output AgentOutput) {
	output.Info("Commands:")
	output.Info("  /help            Show this help")
	output.Info("  /ask             Switch to Ask mode (read-only)")
	output.Info("  /plan            Switch to Plan mode (explore & design)")
	output.Info("  /plan <goal>     Create a plan for a goal")
	output.Info("  /build           Switch to Build mode (full execution)")
	output.Info("  /copy            Copy conversation to clipboard")
	output.Info("  /mode <tier>     Switch model (fast/smart/genius)")
	output.Info("  /skills          List available skills")
	output.Info("  /status          Check vecgrep status")
	output.Info("  /reindex         Update vecgrep search index")
	output.Info("  /context         Show context usage breakdown")
	output.Info("  /compact [focus] Compact conversation (optional focus)")
	output.Info("  /sessions        List saved sessions")
	output.Info("  /resume [id]     Resume a session (last if no id)")
	output.Info("  /new             Start a new session")
	output.Info("  /delete <id>     Delete a session")
	output.Info("  /rewind          Undo last agent's file changes")
	output.Info("  /clear           Clear conversation")
	output.Info("  /exit            Exit interactive mode")
	output.Info("")
	output.Info("Modes: Shift+Tab to cycle | Ask (read-only) → Plan (explore) → Build (execute)")
	output.Info("Debug: Run with --debug or -d flag")
}

// showSkills displays available skills.
func (ch *CommandHandler) showSkills(output AgentOutput) {
	skillList := ch.agent.skills.List()
	if len(skillList) == 0 {
		output.Info("No skills loaded")
		return
	}
	output.Info("Available Skills:")
	for _, s := range skillList {
		output.Info(fmt.Sprintf("  %s - %s", s.Name, s.Description))
		if len(s.Triggers) > 0 {
			output.Info(fmt.Sprintf("    Triggers: %s", strings.Join(s.Triggers, ", ")))
		}
	}
}

// showSessions lists saved sessions.
func (ch *CommandHandler) showSessions(output AgentOutput) {
	a := ch.agent
	if a.sessionMgr == nil {
		output.ErrorStr("Session manager not available")
		return
	}
	sessions, err := a.sessionMgr.List()
	if err != nil {
		output.ErrorStr("Failed to list sessions: " + err.Error())
		return
	}
	if len(sessions) == 0 {
		output.Info("No saved sessions")
		return
	}
	output.Info("Saved sessions:")
	for _, s := range sessions {
		bullet := "\u25cb" // ○
		suffix := ""
		if curr := a.sessionMgr.GetCurrentSession(); curr != nil && curr.ID == s.ID {
			bullet = "\u25cf" // ●
			suffix = " <- current"
		}
		relTime := session.FormatRelativeTime(s.UpdatedAt)
		preview := s.Preview
		if len(preview) > 35 {
			preview = preview[:32] + "..."
		}
		if preview != "" {
			preview = fmt.Sprintf(" \"%s\"", preview)
		}
		output.Info(fmt.Sprintf("  %s %s  %-8s  %2d msgs%s%s",
			bullet, s.ID[:8], relTime, s.MsgCount, preview, suffix))
	}
}

// resumeSession resumes a saved session.
func (ch *CommandHandler) resumeSession(parts []string, output AgentOutput, cmdCtx CommandContext) {
	a := ch.agent
	if a.sessionMgr == nil {
		output.ErrorStr("Session manager not available")
		return
	}
	var sess *session.Session
	var err error
	if len(parts) > 1 {
		// Resume specific session by ID prefix
		sessions, listErr := a.sessionMgr.List()
		if listErr != nil {
			output.ErrorStr("Failed to list sessions: " + listErr.Error())
			return
		}
		prefix := parts[1]
		for _, s := range sessions {
			if strings.HasPrefix(s.ID, prefix) {
				sess, err = a.sessionMgr.Load(s.ID)
				break
			}
		}
		if sess == nil && err == nil {
			output.ErrorStr("No session found with prefix: " + prefix)
			return
		}
	} else {
		// Resume last session
		sess, err = a.sessionMgr.GetCurrent()
	}
	if err != nil {
		output.ErrorStr("Failed to load session: " + err.Error())
		return
	}
	if sess == nil {
		output.Info("No session to resume")
		return
	}
	a.contextMgr.RestoreMessages(sess.Messages)
	a.sessionMgr.SetCurrent(sess)
	cmdCtx.SetSessionID(sess.ID[:8])
	output.Success(fmt.Sprintf("Resumed session %s (%d messages)", sess.ID[:8], len(sess.Messages)))
	// Show last exchange as context
	if len(sess.Messages) > 0 {
		for i := len(sess.Messages) - 1; i >= 0; i-- {
			if sess.Messages[i].Role == "user" {
				lastMsg := sess.Messages[i].Content
				if len(lastMsg) > 60 {
					lastMsg = lastMsg[:57] + "..."
				}
				output.Info(fmt.Sprintf("Last message: \"%s\"", lastMsg))
				break
			}
		}
	}
}

// newSession starts a new session.
func (ch *CommandHandler) newSession(output AgentOutput, cmdCtx CommandContext) {
	a := ch.agent
	if a.sessionMgr == nil {
		output.ErrorStr("Session manager not available")
		return
	}
	// Current session is already saved via auto-save, just start fresh
	a.contextMgr.Clear()
	a.shownContextWarning = false
	sess, err := a.sessionMgr.StartNew()
	if err != nil {
		output.ErrorStr("Failed to start new session: " + err.Error())
		return
	}
	cmdCtx.ClearDisplay()
	cmdCtx.ClearQueue()
	cmdCtx.SetSessionID(sess.ID[:8])
	output.Success(fmt.Sprintf("Started new session (%s)", sess.ID[:8]))
}

// switchMode changes the agent mode and updates permissions accordingly.
func (ch *CommandHandler) switchMode(mode tui.AgentMode, output AgentOutput, cmdCtx CommandContext) {
	a := ch.agent
	oldMode := a.agentMode
	a.applyModeChange(mode, true)
	cmdCtx.SetAgentMode(mode)

	var desc string
	switch mode {
	case tui.ModeAsk:
		desc = "Ask mode — read-only exploration, Q&A"
	case tui.ModePlan:
		desc = "Plan mode — design & explore, writes prompt"
	case tui.ModeBuild:
		desc = "Build mode — full execution"
	}
	output.Success(desc)

	// Log mode change event
	if log := logging.Global(); log != nil {
		log.Event(logging.EventAgentModeChange,
			logging.From(oldMode.String()),
			logging.To(mode.String()),
		)
	}
}

// rewindCheckpoint restores files from the last checkpoint.
func (ch *CommandHandler) rewindCheckpoint(output AgentOutput) {
	a := ch.agent
	if a.checkpointMgr == nil || !a.checkpointMgr.HasCheckpoints() {
		output.Info("No checkpoints available to rewind")
		return
	}
	restored, err := a.checkpointMgr.Rewind()
	if err != nil {
		output.Warning("Rewind completed with errors: " + err.Error())
	}
	if len(restored) == 0 {
		output.Info("No files were changed in the last checkpoint")
		return
	}
	output.Success(fmt.Sprintf("Rewound %d file(s):", len(restored)))
	for _, path := range restored {
		output.Info("  " + path)
	}
}

// deleteSession deletes a saved session.
func (ch *CommandHandler) deleteSession(parts []string, output AgentOutput) {
	a := ch.agent
	if a.sessionMgr == nil {
		output.ErrorStr("Session manager not available")
		return
	}
	if len(parts) < 2 {
		output.ErrorStr("Usage: /delete <session-id> [--force]")
		return
	}
	// Check for --force flag
	forceDelete := false
	prefix := parts[1]
	if len(parts) > 2 && parts[2] == "--force" {
		forceDelete = true
	}
	// Find session by prefix
	sessions, err := a.sessionMgr.List()
	if err != nil {
		output.ErrorStr("Failed to list sessions: " + err.Error())
		return
	}
	var found string
	for _, s := range sessions {
		if strings.HasPrefix(s.ID, prefix) {
			found = s.ID
			break
		}
	}
	if found == "" {
		output.ErrorStr("No session found with prefix: " + prefix)
		return
	}
	// Check if deleting current session
	if curr := a.sessionMgr.GetCurrentSession(); curr != nil && curr.ID == found {
		if !forceDelete {
			output.Warning(fmt.Sprintf("Session %s is currently active", found[:8]))
			output.Info("Use /delete " + prefix + " --force to confirm")
			return
		}
		// Clear context since we're deleting current session
		a.contextMgr.Clear()
		a.shownContextWarning = false
	}
	if err := a.sessionMgr.Delete(found); err != nil {
		output.ErrorStr("Failed to delete session: " + err.Error())
		return
	}
	output.Success(fmt.Sprintf("Deleted session %s", found[:8]))
}
