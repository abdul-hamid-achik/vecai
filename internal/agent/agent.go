package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	ctxmgr "github.com/abdul-hamid-achik/vecai/internal/context"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/skills"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/tui"
	"github.com/abdul-hamid-achik/vecai/internal/ui"
)

// ErrExit is returned when the user requests to exit
var ErrExit = errors.New("user requested exit")

const systemPrompt = `You are vecai, an AI-powered codebase assistant. You help developers understand, navigate, and modify their code.

You have access to the following tools:
- vecgrep_search: Semantic search across the codebase using vector embeddings
- vecgrep_status: Check the status of the search index
- read_file: Read file contents
- write_file: Write content to a file
- edit_file: Make targeted edits to a file
- list_files: List files in a directory
- bash: Execute shell commands
- grep: Search for patterns in files

Guidelines:
1. Use vecgrep_search for understanding concepts, finding related code, or exploring the codebase
2. Use grep for exact string/pattern matching
3. Always read files before modifying them
4. Use edit_file for small changes, write_file for new files or complete rewrites
5. Be concise but thorough in explanations
6. Show relevant code snippets when explaining
7. Ask clarifying questions if the request is ambiguous

When responding:
- Format code blocks with language specifiers
- Use bullet points for lists
- Reference file paths and line numbers when discussing code`

// Config holds agent configuration
type Config struct {
	LLM         llm.LLMClient
	Tools       *tools.Registry
	Permissions *permissions.Policy
	Skills      *skills.Loader
	Output      *ui.OutputHandler
	Input       *ui.InputHandler
	Config      *config.Config
}

// Agent is the main AI agent
type Agent struct {
	llm         llm.LLMClient
	tools       *tools.Registry
	permissions *permissions.Policy
	skills      *skills.Loader
	output      *ui.OutputHandler
	input       *ui.InputHandler
	config      *config.Config
	contextMgr  *ctxmgr.ContextManager
	compactor   *ctxmgr.Compactor
	planner     *Planner

	// Track if we've shown the context warning this session
	shownContextWarning bool
}

// New creates a new agent
func New(cfg Config) *Agent {
	// Build context config from app config
	ctxConfig := ctxmgr.ContextConfig{
		AutoCompactThreshold: cfg.Config.Context.AutoCompactThreshold,
		WarnThreshold:        cfg.Config.Context.WarnThreshold,
		PreserveLast:         cfg.Config.Context.PreserveLast,
		EnableAutoCompact:    cfg.Config.Context.EnableAutoCompact,
		ContextWindow:        cfg.Config.Context.ContextWindow,
	}
	if ctxConfig.ContextWindow == 0 {
		ctxConfig.ContextWindow = ctxmgr.DefaultContextWindow
	}
	if ctxConfig.AutoCompactThreshold == 0 {
		ctxConfig.AutoCompactThreshold = 0.95
	}
	if ctxConfig.WarnThreshold == 0 {
		ctxConfig.WarnThreshold = 0.80
	}
	if ctxConfig.PreserveLast == 0 {
		ctxConfig.PreserveLast = 4
	}

	a := &Agent{
		llm:         cfg.LLM,
		tools:       cfg.Tools,
		permissions: cfg.Permissions,
		skills:      cfg.Skills,
		output:      cfg.Output,
		input:       cfg.Input,
		config:      cfg.Config,
		contextMgr:  ctxmgr.NewContextManager(systemPrompt, ctxConfig),
		compactor:   ctxmgr.NewCompactor(cfg.LLM),
	}
	a.planner = NewPlanner(a)
	return a
}

// Run executes a single query
func (a *Agent) Run(query string) error {
	ctx := context.Background()

	// Check for skill match
	if skill := a.skills.Match(query); skill != nil {
		a.output.Info(fmt.Sprintf("Using skill: %s", skill.Name))
		query = skill.GetPrompt() + "\n\nUser request: " + query
	}

	// Add user message
	a.contextMgr.AddMessage(llm.Message{
		Role:    "user",
		Content: query,
	})

	return a.runLoop(ctx)
}

// RunInteractive starts interactive mode
func (a *Agent) RunInteractive() error {
	a.output.Header("vecai - Interactive Mode")
	a.output.ModelInfo(a.llm.GetModel())
	a.output.TextLn("Type /help for commands, /exit to quit")
	a.output.Separator()

	// Check vecgrep status on first run
	a.checkVecgrepStatus()

	for {
		input, err := a.input.ReadInput("\n> ")
		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			if !a.handleSlashCommand(input) {
				return nil // Exit requested
			}
			continue
		}

		if err := a.Run(input); err != nil {
			a.output.Error(err)
		}
	}
}

// RunInteractiveTUI starts interactive mode with full-screen TUI
func (a *Agent) RunInteractiveTUI() error {
	// Fall back to line-based mode if not a TTY or NO_COLOR is set
	if !tui.IsTTYAvailable() {
		return a.RunInteractive()
	}

	// Create TUI runner
	runner := tui.NewTUIRunner(a.llm.GetModel())

	// Get the adapter for output
	adapter := runner.GetAdapter()

	// Set up submit callback
	runner.SetSubmitCallback(func(input string) {
		input = strings.TrimSpace(input)
		if input == "" {
			return
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			if !a.handleSlashCommandTUI(input, runner) {
				runner.Quit()
				return
			}
			return
		}

		// Run query with TUI output
		if err := a.runWithTUIOutput(input, adapter); err != nil {
			adapter.Error(err)
		}
	})

	// Check vecgrep status on startup
	a.checkVecgrepStatusTUI(adapter)

	// Send initial welcome message
	runner.SendInfo("vecai - Interactive Mode")
	runner.SendInfo("Model: " + a.llm.GetModel())

	// Run TUI
	return runner.Run()
}

// runWithTUIOutput runs a query using the TUI adapter for output
func (a *Agent) runWithTUIOutput(query string, adapter *tui.TUIAdapter) error {
	ctx := context.Background()

	// Set TUI-aware rate limit callback if client supports it
	if rlClient, ok := a.llm.(*llm.RateLimitedClient); ok {
		rlClient.SetWaitCallback(func(ctx context.Context, info llm.WaitInfo) error {
			return adapter.WaitForRateLimit(ctx, info.Duration, info.Reason, info.Attempt, info.MaxAttempts)
		})
	}

	// Check for skill match
	if skill := a.skills.Match(query); skill != nil {
		adapter.Info(fmt.Sprintf("Using skill: %s", skill.Name))
		query = skill.GetPrompt() + "\n\nUser request: " + query
	}

	// Add user message
	a.contextMgr.AddMessage(llm.Message{
		Role:    "user",
		Content: query,
	})

	return a.runLoopTUI(ctx, adapter)
}

// ErrInterrupted is returned when the user interrupts the loop with ESC
var ErrInterrupted = errors.New("interrupted by user")

// runLoopTUI executes the agent loop with TUI output
func (a *Agent) runLoopTUI(ctx context.Context, adapter *tui.TUIAdapter) error {
	const maxIterations = 20
	loopStartTime := time.Now()
	interruptChan := adapter.GetInterruptChan()

	for i := 0; i < maxIterations; i++ {
		// Check for interrupt before starting iteration
		select {
		case <-interruptChan:
			adapter.Warning("Interrupted by user")
			adapter.StreamDone() // Reset TUI state
			return nil           // Not an error, user chose to stop
		default:
		}

		// Send stats update at start of each iteration
		adapter.UpdateStats(tui.SessionStats{
			LoopIteration:  i + 1,
			MaxIterations:  maxIterations,
			LoopStartTime:  loopStartTime,
		})

		// Get tool definitions
		toolDefs := a.getToolDefinitions()

		// Call LLM with streaming
		stream := a.llm.ChatStream(ctx, a.contextMgr.GetMessages(), toolDefs, systemPrompt)

		var response llm.Response
		var textContent strings.Builder
		var toolCalls []llm.ToolCall

		// Process stream
		for chunk := range stream {
			// Check for interrupt during streaming
			select {
			case <-interruptChan:
				adapter.Warning("Interrupted by user")
				adapter.StreamDone() // Reset TUI state
				return nil           // Not an error, user chose to stop
			default:
			}

			switch chunk.Type {
			case "text":
				adapter.StreamText(chunk.Text)
				textContent.WriteString(chunk.Text)

			case "thinking":
				adapter.StreamThinking(chunk.Text)

			case "tool_call":
				if chunk.ToolCall != nil {
					toolCalls = append(toolCalls, *chunk.ToolCall)
				}

			case "done":
				// Pass usage data if available
				if chunk.Usage != nil {
					adapter.StreamDoneWithUsage(chunk.Usage.InputTokens, chunk.Usage.OutputTokens)
				} else {
					adapter.StreamDone()
				}

			case "error":
				if chunk.Error != nil {
					return chunk.Error
				}
			}
		}

		response.Content = textContent.String()
		response.ToolCalls = toolCalls

		// Add assistant message
		if response.Content != "" {
			a.contextMgr.AddMessage(llm.Message{
				Role:    "assistant",
				Content: response.Content,
			})
		}

		// Update context stats after each API call
		a.updateContextStatsTUI(ctx, adapter)

		// If no tool calls, we're done
		if len(response.ToolCalls) == 0 {
			return nil
		}

		// Execute tool calls
		toolResults := a.executeToolCallsTUI(ctx, response.ToolCalls, adapter)

		// Add tool results as user message
		var resultContent strings.Builder
		for _, result := range toolResults {
			resultContent.WriteString(fmt.Sprintf("Tool %s result:\n%s\n\n", result.Name, result.Result))
		}

		a.contextMgr.AddMessage(llm.Message{
			Role:    "user",
			Content: resultContent.String(),
		})
	}

	return fmt.Errorf("max iterations reached")
}

// updateContextStatsTUI updates context stats and handles auto-compact/warnings
func (a *Agent) updateContextStatsTUI(ctx context.Context, adapter *tui.TUIAdapter) {
	stats := a.contextMgr.GetStats()

	// Update TUI with context stats
	adapter.UpdateContextStats(
		stats.UsagePercent,
		stats.UsedTokens,
		stats.ContextWindow,
		stats.NeedsWarning,
	)

	// Handle auto-compact at threshold
	if stats.NeedsCompaction && a.config.Context.EnableAutoCompact {
		adapter.Warning(fmt.Sprintf("Context at %.0f%% - auto-compacting...", stats.UsagePercent*100))
		if err := a.autoCompactTUI(ctx, adapter); err != nil {
			adapter.Warning("Auto-compact failed: " + err.Error())
		}
		return
	}

	// Show warning at threshold (once per high-usage session)
	if stats.NeedsWarning && !a.shownContextWarning {
		adapter.Warning(fmt.Sprintf("Context at %.0f%% - consider using /compact", stats.UsagePercent*100))
		a.shownContextWarning = true
	}
}

// autoCompactTUI performs automatic compaction with TUI output
func (a *Agent) autoCompactTUI(ctx context.Context, adapter *tui.TUIAdapter) error {
	return a.compactConversation(ctx, "", adapter)
}

// compactConversation compacts the conversation history
func (a *Agent) compactConversation(ctx context.Context, focusPrompt string, adapter *tui.TUIAdapter) error {
	messages := a.contextMgr.GetMessages()
	if len(messages) == 0 {
		if adapter != nil {
			adapter.Info("No conversation to compact")
		}
		return nil
	}

	preserveLast := a.contextMgr.GetPreserveLast()

	result, err := a.compactor.Compact(ctx, ctxmgr.CompactRequest{
		Messages:     messages,
		FocusPrompt:  focusPrompt,
		PreserveLast: preserveLast,
	})
	if err != nil {
		return fmt.Errorf("compaction failed: %w", err)
	}

	// Replace history with summary
	a.contextMgr.ReplaceWithSummary(result.Summary, result.PreservedMsgs)

	// Reset warning flag after compaction
	a.shownContextWarning = false

	// Update TUI with new stats
	if adapter != nil {
		newStats := a.contextMgr.GetStats()
		adapter.UpdateContextStats(
			newStats.UsagePercent,
			newStats.UsedTokens,
			newStats.ContextWindow,
			newStats.NeedsWarning,
		)
		adapter.Success(fmt.Sprintf("Compacted: %d msgs summarized, saved ~%d tokens (now at %.0f%%)",
			result.MessagesSummarized, result.TokensSaved, newStats.UsagePercent*100))
	}

	return nil
}

// executeToolCallsTUI executes tool calls with TUI output
func (a *Agent) executeToolCallsTUI(ctx context.Context, calls []llm.ToolCall, adapter *tui.TUIAdapter) []toolResult {
	var results []toolResult

	for _, call := range calls {
		tool, ok := a.tools.Get(call.Name)
		if !ok {
			results = append(results, toolResult{
				Name:   call.Name,
				Result: fmt.Sprintf("Unknown tool: %s", call.Name),
				Error:  true,
			})
			continue
		}

		// Build description for permission prompt
		description := formatToolDescription(call.Name, call.Input)
		adapter.ToolCall(call.Name, description)

		// Check permission using TUI adapter
		allowed, err := a.checkPermissionTUI(call.Name, tool.Permission(), description, adapter)
		if err != nil {
			results = append(results, toolResult{
				Name:   call.Name,
				Result: fmt.Sprintf("Permission error: %s", err),
				Error:  true,
			})
			adapter.ToolResult(call.Name, "Permission error: "+err.Error(), true)
			continue
		}

		if !allowed {
			results = append(results, toolResult{
				Name:   call.Name,
				Result: "Permission denied by user",
				Error:  true,
			})
			adapter.ToolResult(call.Name, "Permission denied", true)
			continue
		}

		// Execute tool
		result, err := tool.Execute(ctx, call.Input)
		if err != nil {
			results = append(results, toolResult{
				Name:   call.Name,
				Result: fmt.Sprintf("Error: %s", err),
				Error:  true,
			})
			adapter.ToolResult(call.Name, err.Error(), true)
		} else {
			results = append(results, toolResult{
				Name:   call.Name,
				Result: result,
				Error:  false,
			})
			adapter.ToolResult(call.Name, result, false)
		}
	}

	return results
}

// checkPermissionTUI checks permission using the TUI adapter
func (a *Agent) checkPermissionTUI(toolName string, level tools.PermissionLevel, description string, adapter *tui.TUIAdapter) (bool, error) {
	// Use the existing permission policy but with TUI-compatible I/O
	// For auto mode, always allow
	if a.permissions.GetMode() == permissions.ModeAuto {
		return true, nil
	}

	// Check cache
	if decision, ok := a.permissions.GetCachedDecision(toolName); ok {
		switch decision {
		case permissions.DecisionAlwaysAllow:
			return true, nil
		case permissions.DecisionNeverAllow:
			return false, nil
		}
	}

	// In ask mode, auto-approve reads
	if a.permissions.GetMode() == permissions.ModeAsk && level == tools.PermissionRead {
		return true, nil
	}

	// Need to prompt user via TUI
	adapter.PermissionPrompt(toolName, level, description)

	// Wait for response from TUI
	response, err := adapter.ReadLine("")
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	case "a", "always":
		// Note: We can't directly modify the policy cache from here
		// The permission check will handle caching on next call
		return true, nil
	case "v", "never":
		return false, nil
	default:
		return false, nil
	}
}

// handleSlashCommandTUI handles slash commands in TUI mode
func (a *Agent) handleSlashCommandTUI(cmd string, runner *tui.TUIRunner) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return true
	}

	switch parts[0] {
	case "/help":
		runner.SendInfo("Commands:")
		runner.SendInfo("  /help            Show this help")
		runner.SendInfo("  /mode <tier>     Switch model (fast/smart/genius)")
		runner.SendInfo("  /plan <goal>     Enter plan mode")
		runner.SendInfo("  /skills          List available skills")
		runner.SendInfo("  /status          Check vecgrep status")
		runner.SendInfo("  /context         Show context usage breakdown")
		runner.SendInfo("  /compact [focus] Compact conversation (optional focus)")
		runner.SendInfo("  /clear           Clear conversation")
		runner.SendInfo("  /exit            Exit interactive mode")
		return true

	case "/exit", "/quit":
		runner.SendInfo("Goodbye!")
		return false

	case "/clear":
		a.contextMgr.Clear()
		a.shownContextWarning = false
		runner.GetAdapter().Clear()
		runner.SendSuccess("Conversation cleared")
		return true

	case "/context":
		stats := a.contextMgr.GetStats()
		breakdown := a.contextMgr.GetBreakdown()
		runner.SendInfo(fmt.Sprintf("Context: %d/%d tokens (%.1f%%)",
			stats.UsedTokens, stats.ContextWindow, stats.UsagePercent*100))
		runner.SendInfo(fmt.Sprintf("  System: %d | User: %d | Assistant: %d | Tools: %d",
			breakdown.SystemPrompt, breakdown.UserMessages,
			breakdown.AssistantMsgs, breakdown.ToolResults))
		runner.SendInfo(fmt.Sprintf("  Messages: %d", stats.MessageCount))
		if stats.NeedsCompaction {
			runner.SendWarning("Context needs compaction - use /compact")
		} else if stats.NeedsWarning {
			runner.SendWarning("Context is getting full - consider /compact")
		}
		return true

	case "/compact":
		focusPrompt := ""
		if len(parts) > 1 {
			focusPrompt = strings.Join(parts[1:], " ")
		}
		runner.SendInfo("Compacting conversation...")
		ctx := context.Background()
		if err := a.compactConversation(ctx, focusPrompt, runner.GetAdapter()); err != nil {
			runner.SendError("Compact failed: " + err.Error())
		}
		return true

	case "/mode":
		if len(parts) < 2 {
			runner.SendInfo("Current model: " + a.llm.GetModel())
			runner.SendInfo("Usage: /mode <fast|smart|genius>")
			return true
		}
		switch parts[1] {
		case "fast":
			a.llm.SetTier(config.TierFast)
			runner.SendSuccess("Switched to fast mode (Haiku)")
		case "smart":
			a.llm.SetTier(config.TierSmart)
			runner.SendSuccess("Switched to smart mode (Sonnet)")
		case "genius":
			a.llm.SetTier(config.TierGenius)
			runner.SendSuccess("Switched to genius mode (Opus)")
		default:
			runner.SendError("Unknown mode: " + parts[1])
		}
		return true

	case "/plan":
		if len(parts) < 2 {
			runner.SendError("Usage: /plan <goal>")
			return true
		}
		goal := strings.Join(parts[1:], " ")
		runner.SendInfo("Plan mode not fully supported in TUI yet. Use: vecai plan \"" + goal + "\"")
		return true

	case "/skills":
		skills := a.skills.List()
		if len(skills) == 0 {
			runner.SendInfo("No skills loaded")
			return true
		}
		runner.SendInfo("Available Skills:")
		for _, s := range skills {
			runner.SendInfo(fmt.Sprintf("  %s - %s", s.Name, s.Description))
		}
		return true

	case "/status":
		a.checkVecgrepStatusTUI(runner.GetAdapter())
		return true

	default:
		runner.SendError("Unknown command: " + parts[0] + ". Type /help for available commands.")
		return true
	}
}

// checkVecgrepStatusTUI checks vecgrep status with TUI output
func (a *Agent) checkVecgrepStatusTUI(adapter *tui.TUIAdapter) {
	ctx := context.Background()
	tool, _ := a.tools.Get("vecgrep_status")
	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil {
		adapter.Warning("vecgrep status check failed: " + err.Error())
		return
	}

	if strings.Contains(result, "not initialized") {
		adapter.Warning("vecgrep is not initialized. Run 'vecgrep init' for semantic search.")
	} else {
		adapter.Info("vecgrep index is ready")
	}
}

// IsTTY checks if stdout is a terminal
func IsTTY() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// RunPlan runs in plan mode
func (a *Agent) RunPlan(goal string) error {
	return a.planner.Execute(goal)
}

// handleSlashCommand handles interactive slash commands.
// Returns: shouldContinue (true = keep running, false = exit)
// Returns: wasHandled (true = command recognized, false = treat as query)
func (a *Agent) handleSlashCommand(cmd string) (shouldContinue bool) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return true // Continue, treat as empty input
	}

	switch parts[0] {
	case "/help":
		a.showHelp()
		return true

	case "/exit", "/quit":
		a.output.TextLn("Goodbye!")
		return false // Signal exit

	case "/clear":
		a.contextMgr.Clear()
		a.shownContextWarning = false
		a.input.Clear()
		a.output.Success("Conversation cleared")
		return true

	case "/context":
		stats := a.contextMgr.GetStats()
		breakdown := a.contextMgr.GetBreakdown()
		a.output.TextLn(fmt.Sprintf("Context: %d/%d tokens (%.1f%%)",
			stats.UsedTokens, stats.ContextWindow, stats.UsagePercent*100))
		a.output.TextLn(fmt.Sprintf("  System: %d | User: %d | Assistant: %d | Tools: %d",
			breakdown.SystemPrompt, breakdown.UserMessages,
			breakdown.AssistantMsgs, breakdown.ToolResults))
		a.output.TextLn(fmt.Sprintf("  Messages: %d", stats.MessageCount))
		if stats.NeedsCompaction {
			a.output.Warning("Context needs compaction - use /compact")
		} else if stats.NeedsWarning {
			a.output.Warning("Context is getting full - consider /compact")
		}
		return true

	case "/compact":
		focusPrompt := ""
		if len(parts) > 1 {
			focusPrompt = strings.Join(parts[1:], " ")
		}
		a.output.Info("Compacting conversation...")
		ctx := context.Background()
		if err := a.compactConversation(ctx, focusPrompt, nil); err != nil {
			a.output.ErrorStr("Compact failed: " + err.Error())
		} else {
			newStats := a.contextMgr.GetStats()
			a.output.Success(fmt.Sprintf("Compacted to %.0f%%", newStats.UsagePercent*100))
		}
		return true

	case "/mode":
		if len(parts) < 2 {
			a.output.TextLn("Current model: " + a.llm.GetModel())
			a.output.TextLn("Usage: /mode <fast|smart|genius>")
			return true
		}
		switch parts[1] {
		case "fast":
			a.llm.SetTier(config.TierFast)
			a.output.Success("Switched to fast mode (Haiku)")
		case "smart":
			a.llm.SetTier(config.TierSmart)
			a.output.Success("Switched to smart mode (Sonnet)")
		case "genius":
			a.llm.SetTier(config.TierGenius)
			a.output.Success("Switched to genius mode (Opus)")
		default:
			a.output.ErrorStr("Unknown mode: " + parts[1])
		}
		return true

	case "/plan":
		if len(parts) < 2 {
			a.output.ErrorStr("Usage: /plan <goal>")
			return true
		}
		goal := strings.Join(parts[1:], " ")
		if err := a.RunPlan(goal); err != nil {
			a.output.Error(err)
		}
		return true

	case "/skills":
		a.showSkills()
		return true

	case "/status":
		a.checkVecgrepStatus()
		return true

	default:
		// Unknown command - show error and continue
		a.output.ErrorStr("Unknown command: " + parts[0] + ". Type /help for available commands.")
		return true
	}
}

// runLoop executes the agent loop until completion
func (a *Agent) runLoop(ctx context.Context) error {
	const maxIterations = 20

	for i := 0; i < maxIterations; i++ {
		// Get tool definitions
		toolDefs := a.getToolDefinitions()

		// Call LLM with streaming
		stream := a.llm.ChatStream(ctx, a.contextMgr.GetMessages(), toolDefs, systemPrompt)

		var response llm.Response
		var textContent strings.Builder
		var toolCalls []llm.ToolCall

		// Process stream
		for chunk := range stream {
			switch chunk.Type {
			case "text":
				a.output.StreamText(chunk.Text)
				textContent.WriteString(chunk.Text)

			case "thinking":
				a.output.StreamThinking(chunk.Text)

			case "tool_call":
				if chunk.ToolCall != nil {
					toolCalls = append(toolCalls, *chunk.ToolCall)
				}

			case "done":
				a.output.StreamDone()

			case "error":
				if chunk.Error != nil {
					return chunk.Error
				}
			}
		}

		response.Content = textContent.String()
		response.ToolCalls = toolCalls

		// Add assistant message
		if response.Content != "" {
			a.contextMgr.AddMessage(llm.Message{
				Role:    "assistant",
				Content: response.Content,
			})
		}

		// Check context usage after each API call
		a.checkContextUsage(ctx)

		// If no tool calls, we're done
		if len(response.ToolCalls) == 0 {
			return nil
		}

		// Execute tool calls
		toolResults := a.executeToolCalls(ctx, response.ToolCalls)

		// Add tool results as user message
		var resultContent strings.Builder
		for _, result := range toolResults {
			resultContent.WriteString(fmt.Sprintf("Tool %s result:\n%s\n\n", result.Name, result.Result))
		}

		a.contextMgr.AddMessage(llm.Message{
			Role:    "user",
			Content: resultContent.String(),
		})
	}

	return fmt.Errorf("max iterations reached")
}

// checkContextUsage checks context usage and handles warnings/auto-compact for non-TUI mode
func (a *Agent) checkContextUsage(ctx context.Context) {
	stats := a.contextMgr.GetStats()

	// Handle auto-compact at threshold
	if stats.NeedsCompaction && a.config.Context.EnableAutoCompact {
		a.output.Warning(fmt.Sprintf("Context at %.0f%% - auto-compacting...", stats.UsagePercent*100))
		if err := a.compactConversation(ctx, "", nil); err != nil {
			a.output.Warning("Auto-compact failed: " + err.Error())
		} else {
			newStats := a.contextMgr.GetStats()
			a.output.Success(fmt.Sprintf("Compacted to %.0f%%", newStats.UsagePercent*100))
		}
		return
	}

	// Show warning at threshold (once per high-usage session)
	if stats.NeedsWarning && !a.shownContextWarning {
		a.output.Warning(fmt.Sprintf("Context at %.0f%% - consider using /compact", stats.UsagePercent*100))
		a.shownContextWarning = true
	}
}

type toolResult struct {
	Name   string
	Result string
	Error  bool
}

// executeToolCalls executes tool calls with permission checking
func (a *Agent) executeToolCalls(ctx context.Context, calls []llm.ToolCall) []toolResult {
	var results []toolResult

	for _, call := range calls {
		tool, ok := a.tools.Get(call.Name)
		if !ok {
			results = append(results, toolResult{
				Name:   call.Name,
				Result: fmt.Sprintf("Unknown tool: %s", call.Name),
				Error:  true,
			})
			continue
		}

		// Build description for permission prompt
		description := formatToolDescription(call.Name, call.Input)
		a.output.ToolCall(call.Name, description)

		// Check permission
		allowed, err := a.permissions.Check(call.Name, tool.Permission(), description)
		if err != nil {
			results = append(results, toolResult{
				Name:   call.Name,
				Result: fmt.Sprintf("Permission error: %s", err),
				Error:  true,
			})
			a.output.ToolResult(call.Name, "Permission error: "+err.Error(), true)
			continue
		}

		if !allowed {
			results = append(results, toolResult{
				Name:   call.Name,
				Result: "Permission denied by user",
				Error:  true,
			})
			a.output.ToolResult(call.Name, "Permission denied", true)
			continue
		}

		// Execute tool
		result, err := tool.Execute(ctx, call.Input)
		if err != nil {
			results = append(results, toolResult{
				Name:   call.Name,
				Result: fmt.Sprintf("Error: %s", err),
				Error:  true,
			})
			a.output.ToolResult(call.Name, err.Error(), true)
		} else {
			results = append(results, toolResult{
				Name:   call.Name,
				Result: result,
				Error:  false,
			})
			a.output.ToolResult(call.Name, result, false)
		}
	}

	return results
}

// getToolDefinitions converts tools to LLM format
func (a *Agent) getToolDefinitions() []llm.ToolDefinition {
	registryDefs := a.tools.GetDefinitions()
	defs := make([]llm.ToolDefinition, len(registryDefs))

	for i, d := range registryDefs {
		defs[i] = llm.ToolDefinition{
			Name:        d.Name,
			Description: d.Description,
			InputSchema: d.InputSchema,
		}
	}

	return defs
}

// formatToolDescription creates a human-readable description of a tool call
func formatToolDescription(name string, input map[string]any) string {
	switch name {
	case "read_file":
		if path, ok := input["path"].(string); ok {
			return fmt.Sprintf("Read %s", path)
		}
	case "write_file":
		if path, ok := input["path"].(string); ok {
			return fmt.Sprintf("Write to %s", path)
		}
	case "edit_file":
		if path, ok := input["path"].(string); ok {
			return fmt.Sprintf("Edit %s", path)
		}
	case "bash":
		if cmd, ok := input["command"].(string); ok {
			if len(cmd) > 50 {
				cmd = cmd[:50] + "..."
			}
			return fmt.Sprintf("Run: %s", cmd)
		}
	case "vecgrep_search":
		if query, ok := input["query"].(string); ok {
			return fmt.Sprintf("Search: %s", query)
		}
	case "grep":
		if pattern, ok := input["pattern"].(string); ok {
			return fmt.Sprintf("Grep: %s", pattern)
		}
	case "list_files":
		path := "."
		if p, ok := input["path"].(string); ok {
			path = p
		}
		return fmt.Sprintf("List files in %s", path)
	}
	return ""
}

// checkVecgrepStatus checks if vecgrep is initialized
func (a *Agent) checkVecgrepStatus() {
	ctx := context.Background()
	tool, _ := a.tools.Get("vecgrep_status")
	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil {
		a.output.Warning("vecgrep status check failed: " + err.Error())
		return
	}

	if strings.Contains(result, "not initialized") {
		a.output.Warning("vecgrep is not initialized. Run 'vecgrep init' for semantic search.")
	} else {
		a.output.Info("vecgrep index is ready")
	}
}

// showHelp displays help information
func (a *Agent) showHelp() {
	a.output.Header("Commands")
	a.output.TextLn("/help            Show this help")
	a.output.TextLn("/mode <tier>     Switch model (fast/smart/genius)")
	a.output.TextLn("/plan <goal>     Enter plan mode")
	a.output.TextLn("/skills          List available skills")
	a.output.TextLn("/status          Check vecgrep status")
	a.output.TextLn("/context         Show context usage breakdown")
	a.output.TextLn("/compact [focus] Compact conversation (optional focus prompt)")
	a.output.TextLn("/clear           Clear conversation")
	a.output.TextLn("/exit            Exit interactive mode")
}

// showSkills displays available skills
func (a *Agent) showSkills() {
	skills := a.skills.List()
	if len(skills) == 0 {
		a.output.Info("No skills loaded")
		return
	}

	a.output.Header("Available Skills")
	for _, s := range skills {
		a.output.TextLn(fmt.Sprintf("  %s - %s", s.Name, s.Description))
		if len(s.Triggers) > 0 {
			a.output.TextLn(fmt.Sprintf("    Triggers: %s", strings.Join(s.Triggers, ", ")))
		}
	}
}

// ClearHistory clears conversation history
func (a *Agent) ClearHistory() {
	a.contextMgr.Clear()
	a.shownContextWarning = false
}

// GetHistory returns conversation history
func (a *Agent) GetHistory() []llm.Message {
	return a.contextMgr.GetMessages()
}
