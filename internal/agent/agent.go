package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	ctxmgr "github.com/abdul-hamid-achik/vecai/internal/context"
	"github.com/abdul-hamid-achik/vecai/internal/debug"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/logging"
	"github.com/abdul-hamid-achik/vecai/internal/memory"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/session"
	"github.com/abdul-hamid-achik/vecai/internal/skills"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/tui"
	"github.com/abdul-hamid-achik/vecai/internal/ui"
)

// ErrExit is returned when the user requests to exit
var ErrExit = errors.New("user requested exit")

// analysisSystemPrompt is a minimal prompt for token-efficient analysis mode (~300 tokens vs ~2000)
const analysisSystemPrompt = `You are a code analysis assistant. Help developers understand and document code.

TOOLS: vecgrep_search (semantic), read_file, list_files, grep, gpeek_* (git)
STRATEGY: vecgrep_search first, then read_file for context

Be concise. Cite file:line in answers.`

const systemPrompt = `You are vecai, an AI-powered codebase assistant. You help developers understand, navigate, and modify their code.

## Available Tools
- vecgrep_search: Semantic search using vector embeddings (PREFERRED for exploration)
- vecgrep_similar: Find code similar to a snippet or location
- vecgrep_status: Check search index status
- read_file: Read file contents (use AFTER vecgrep identifies relevant files)
- write_file: Write content to a file
- edit_file: Make targeted edits to a file
- list_files: List files in a directory
- bash: Execute shell commands
- grep: Exact pattern matching (literals, identifiers, regex)

## Tool Selection Strategy (CRITICAL)

**ALWAYS use vecgrep_search FIRST for codebase exploration:**
- "Where is X implemented?" → vecgrep_search("X implementation")
- "How does Y work?" → vecgrep_search("Y logic flow")
- "Find code related to Z" → vecgrep_search("Z functionality")
- "What handles authentication?" → vecgrep_search("authentication handler")
- "Show me the API endpoints" → vecgrep_search("API endpoint handler")

**Use vecgrep_similar when:**
- Finding code patterns similar to existing code
- Discovering duplicate or related implementations
- Exploring how similar problems are solved elsewhere

**Only use grep when:**
- Searching for EXACT strings (error messages, constants, specific identifiers)
- Finding all usages of a known function/variable name
- Pattern matching with regex where semantics don't matter

**Only use list_files/read_file when:**
- You already know the exact file path from vecgrep results
- You need the FULL file context after vecgrep identified it
- Browsing directory structure (not searching for code)

## Why vecgrep First?
vecgrep uses semantic embeddings that understand code MEANING, not just text matching.
- "error handling" finds try/catch, Result types, error returns - even without those exact words
- "database connection" finds pool setup, connection strings, ORM initialization
- Much faster than iterating through files with grep

## Guidelines
1. Start exploration with vecgrep_search - it understands concepts
2. Use vecgrep results to identify files, then read_file for full context
3. Always read files before modifying them
4. Use edit_file for small changes, write_file for new files or complete rewrites
5. Be concise but thorough in explanations
6. Ask clarifying questions if the request is ambiguous

## Response Format
- Format code blocks with language specifiers (e.g., ` + "```" + `go)
- Use proper markdown lists: "1. Item" (with dot and space), "- Item" (with space after dash)
- Use **bold** for emphasis, not plain text
- Reference file paths and line numbers when discussing code`

// Config holds agent configuration
type Config struct {
	LLM          llm.LLMClient
	Tools        *tools.Registry
	Permissions  *permissions.Policy
	Skills       *skills.Loader
	Output       *ui.OutputHandler
	Input        *ui.InputHandler
	Config       *config.Config
	AnalysisMode bool // Enable token-efficient analysis mode
	AutoTier     bool // Enable automatic tier selection based on query
	CaptureMode  bool // Prompt to save responses to notes
}

// Agent is the main AI agent
type Agent struct {
	llm                 llm.LLMClient
	tools               *tools.Registry
	toolSelector        *tools.ToolSelector
	tierSelector        *TierSelector
	permissions         *permissions.Policy
	skills              *skills.Loader
	output              *ui.OutputHandler
	input               *ui.InputHandler
	config              *config.Config
	contextMgr          *ctxmgr.ContextManager
	compactor           *ctxmgr.Compactor
	resultCache         *ctxmgr.ToolResultCache
	planner             *Planner
	sessionMgr          *session.Manager
	memoryLayer         *memory.MemoryLayer // Unified memory access
	analysisMode        bool                // Token-efficient analysis mode
	autoTier            bool                // Enable automatic tier selection
	quickMode           bool                // Quick mode (no tools, fast tier)
	captureMode         bool                // Prompt to save responses to notes
	currentQuery        string              // Current query for smart tool selection
	projectInstructions string              // Loaded from VECAI.md or AGENTS.md

	// Track if we've shown the context warning this session
	shownContextWarning bool

	// Architect mode state
	architectMode    bool             // Whether in architect mode
	previousPermMode permissions.Mode // To restore after exiting architect mode
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

	// Apply analysis mode overrides for aggressive compaction
	if cfg.AnalysisMode {
		ctxConfig.AutoCompactThreshold = 0.70
		ctxConfig.WarnThreshold = 0.50
		ctxConfig.PreserveLast = 2
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

	// Initialize session manager
	sessionMgr, err := session.NewManager()
	if err != nil {
		// Non-fatal: session persistence won't work but agent can still function
		if log := logging.Global(); log != nil {
			log.Warn("failed to initialize session manager", logging.Error(err))
		}
	}

	// Initialize memory layer if enabled
	var memLayer *memory.MemoryLayer
	if cfg.Config.Memory.Enabled {
		wd, _ := os.Getwd()
		memLayer, err = memory.NewMemoryLayer(wd)
		if err != nil {
			if log := logging.Global(); log != nil {
				log.Warn("memory layer init failed", logging.Error(err))
			}
		}
	}

	// Select system prompt based on mode
	prompt := systemPrompt
	if cfg.AnalysisMode {
		prompt = analysisSystemPrompt
	}

	// Initialize result cache for analysis mode
	var resultCache *ctxmgr.ToolResultCache
	if cfg.AnalysisMode {
		resultCache = ctxmgr.NewToolResultCache(ctxmgr.DefaultCacheTTL)
	}

	a := &Agent{
		llm:                 cfg.LLM,
		tools:               cfg.Tools,
		toolSelector:        tools.NewToolSelector(cfg.Tools),
		tierSelector:        NewTierSelector(),
		permissions:         cfg.Permissions,
		skills:              cfg.Skills,
		output:              cfg.Output,
		input:               cfg.Input,
		config:              cfg.Config,
		contextMgr:          ctxmgr.NewContextManager(prompt, ctxConfig),
		compactor:           ctxmgr.NewCompactor(cfg.LLM),
		resultCache:         resultCache,
		sessionMgr:          sessionMgr,
		memoryLayer:         memLayer,
		analysisMode:        cfg.AnalysisMode,
		autoTier:            cfg.AutoTier,
		captureMode:         cfg.CaptureMode,
		projectInstructions: loadProjectInstructions(),
	}
	a.planner = NewPlanner(a)

	// Wire up auto-save callback
	if sessionMgr != nil {
		a.contextMgr.SetOnSave(func(msgs []llm.Message) {
			if err := a.sessionMgr.Save(msgs, a.llm.GetModel()); err != nil {
				if log := logging.Global(); log != nil {
					log.Warn("failed to save session", logging.Error(err))
				}
			}
		})
	}

	// Wire up learnings callback for auto-learning during compaction
	if memLayer != nil {
		a.compactor.SetLearningsCallback(func(learnings []string) {
			for _, learning := range learnings {
				// Try to determine if it's a correction or a general note
				lower := strings.ToLower(learning)
				if strings.Contains(lower, "prefer") || strings.Contains(lower, "should") ||
					strings.Contains(lower, "always") || strings.Contains(lower, "never") {
					// Store as correction pattern
					if err := memLayer.LearnCorrection("", learning, learning, ""); err != nil {
						if log := logging.Global(); log != nil {
							log.Warn("failed to store correction", logging.Error(err))
						}
					}
				}
			}
		})
	}

	return a
}

// RunQuick executes a query in quick mode: fast tier, no tools, minimal prompt
func (a *Agent) RunQuick(query string) error {
	ctx := context.Background()

	// Force fast tier for speed
	a.llm.SetTier(config.TierFast)

	// Minimal system prompt for quick responses
	quickPrompt := "You are a concise assistant. Answer briefly and directly."

	// Single message, no history, no tools
	messages := []llm.Message{{Role: "user", Content: query}}
	resp, err := a.llm.Chat(ctx, messages, nil, quickPrompt)
	if err != nil {
		return err
	}

	fmt.Println(resp.Content)
	return nil
}

// Run executes a single query (uses TUI if available, otherwise line-based)
func (a *Agent) Run(query string) error {
	// Use TUI if available for consistent experience
	isTTY := tui.IsTTYAvailable()
	logDebug("Run: query=%q, isTTY=%v", query, isTTY)
	if isTTY {
		logDebug("Run: using TUI mode")
		return a.RunTUI(query, true) // Stay open for follow-up queries
	}
	logDebug("Run: using line-based mode (no TTY)")
	return a.runLineBased(query)
}

// runLineBased executes a query with line-based output (non-TUI fallback)
func (a *Agent) runLineBased(query string) error {
	ctx := context.Background()

	// Track current query for smart tool selection
	a.currentQuery = query

	// Apply auto-tier selection if enabled
	if a.autoTier && !a.quickMode {
		selectedTier := a.tierSelector.SelectTier(query, a.config.DefaultTier)
		a.llm.SetTier(selectedTier)
		reason := a.tierSelector.GetTierReason(query)
		logDebug("Auto-tier selected: %s (reason: %s)", selectedTier, reason)

		// Log tier change event
		if log := logging.Global(); log != nil {
			log.Event(logging.EventAgentTierChange,
				logging.Tier(string(selectedTier)),
				logging.Reason(reason),
				logging.Query(query),
			)
		}
	}

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

	// Detect and record corrections for learning
	a.detectAndRecordCorrection(query)

	originalQuery := query // Save for capture
	err := a.runLoop(ctx)
	if err != nil {
		return err
	}

	// Offer to capture if enabled and we have a response
	if a.captureMode {
		messages := a.contextMgr.GetMessages()
		if len(messages) > 0 {
			lastMsg := messages[len(messages)-1]
			if lastMsg.Role == "assistant" && lastMsg.Content != "" {
				if captureErr := a.offerCapture(ctx, originalQuery, lastMsg.Content); captureErr != nil {
					logWarn("Capture failed: %v", captureErr)
				}
			}
		}
	}

	return nil
}

// RunTUI runs the agent with the TUI interface
// initialQuery: optional query to execute immediately after TUI is ready
// interactive: if true, stay open for follow-up queries; if false, quit after initial query (deprecated, always stays open now)
func (a *Agent) RunTUI(initialQuery string, interactive bool) error {
	logDebug("RunTUI called: query=%q, interactive=%v", initialQuery, interactive)

	// Create TUI runner
	runner := tui.NewTUIRunner(a.llm.GetModel())
	adapter := runner.GetAdapter()
	logDebug("TUI runner created")

	// Set up the onReady callback to execute initial query
	if initialQuery != "" {
		logDebug("Setting up onReady callback for initial query")
		runner.SetOnReady(func() {
			logDebug("onReady callback triggered - starting query execution goroutine")
			go func() {
				logDebug("Query execution goroutine started")
				// Add user message block first
				adapter.Info("> " + initialQuery)

				// Set processing state to show spinner
				adapter.Activity("Processing...")

				// Execute the query
				logDebug("Calling runWithTUIOutput")
				err := a.runWithTUIOutput(initialQuery, adapter)
				if err != nil {
					logDebug("Query execution error: %v", err)
					adapter.Error(err)
				}
				logDebug("Query execution complete")
				// Stay open for follow-up queries
			}()
		})
		logDebug("onReady callback registered")
	}

	// Set up submit callback for follow-up queries
	runner.SetSubmitCallback(func(input string) {
		input = strings.TrimSpace(input)
		if input == "" {
			adapter.StreamDone() // Reset TUI state for empty input
			return
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			if !a.handleSlashCommandTUI(input, runner) {
				runner.Quit()
				return
			}
			adapter.StreamDone() // Reset TUI state after slash command
			return
		}

		// Run query with TUI output
		err := a.runWithTUIOutput(input, adapter)
		if err != nil {
			adapter.Error(err)
			// Send StreamDone on error since runLoopTUI doesn't send it on error paths
			// (normal completion already sends StreamDone via the "done" chunk handler)
			adapter.StreamDone()
		}
	})

	// Check vecgrep status on startup (only for interactive mode)
	if interactive || initialQuery == "" {
		a.checkVecgrepStatusTUI(adapter)
	}

	// Header already shows "vecai" and model - no need for welcome message

	// Run TUI - this blocks until user quits
	return runner.Run()
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

	// Check for existing session to show hint (non-blocking)
	var pendingSession *session.Session
	if a.sessionMgr != nil {
		if sess, err := a.sessionMgr.GetCurrent(); err == nil && sess != nil && len(sess.Messages) > 0 {
			pendingSession = sess
		}
	}

	// Create TUI runner
	runner := tui.NewTUIRunner(a.llm.GetModel())
	adapter := runner.GetAdapter()

	// Set up the onReady callback
	runner.SetOnReady(func() {
		// Show non-blocking session hint if available
		if pendingSession != nil {
			preview := ""
			for _, msg := range pendingSession.Messages {
				if msg.Role == "user" {
					preview = msg.Content
					if len(preview) > 40 {
						preview = preview[:37] + "..."
					}
					break
				}
			}
			// Show session ID in header (dimmed since not yet active)
			adapter.SetSessionID(pendingSession.ID[:8] + "?")
			adapter.Info(fmt.Sprintf("Session available (%d msgs): \"%s\" - /resume to continue",
				len(pendingSession.Messages), preview))
		}
	})

	// Set up submit callback for follow-up queries
	runner.SetSubmitCallback(func(input string) {
		input = strings.TrimSpace(input)
		if input == "" {
			adapter.StreamDone()
			return
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			if !a.handleSlashCommandTUI(input, runner) {
				runner.Quit()
				return
			}
			adapter.StreamDone()
			return
		}

		// Starting fresh query - ensure we have a session
		if a.sessionMgr != nil && a.sessionMgr.GetCurrentSession() == nil {
			if sess, err := a.sessionMgr.StartNew(); err != nil {
				adapter.Warning("Failed to start session: " + err.Error())
			} else {
				adapter.SetSessionID(sess.ID[:8])
			}
		}

		// Run query with TUI output
		err := a.runWithTUIOutput(input, adapter)
		if err != nil {
			adapter.Error(err)
			adapter.StreamDone()
		}
	})

	// Check vecgrep status on startup
	a.checkVecgrepStatusTUI(adapter)

	// Run TUI - this blocks until quit
	return runner.Run()
}

// runWithTUIOutput runs a query using the TUI adapter for output
func (a *Agent) runWithTUIOutput(query string, adapter *tui.TUIAdapter) error {
	ctx := context.Background()

	// Track current query for smart tool selection
	a.currentQuery = query

	// Apply auto-tier selection if enabled
	if a.autoTier && !a.quickMode {
		selectedTier := a.tierSelector.SelectTier(query, a.config.DefaultTier)
		a.llm.SetTier(selectedTier)
		reason := a.tierSelector.GetTierReason(query)
		logDebug("Auto-tier selected: %s (reason: %s)", selectedTier, reason)

		// Log tier change event
		if log := logging.Global(); log != nil {
			log.Event(logging.EventAgentTierChange,
				logging.Tier(string(selectedTier)),
				logging.Reason(reason),
				logging.Query(query),
			)
		}
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

	// Detect and record corrections for learning
	a.detectAndRecordCorrection(query)

	return a.runLoopTUI(ctx, adapter)
}

// ErrInterrupted is returned when the user interrupts the loop with ESC
var ErrInterrupted = errors.New("interrupted by user")

// runLoopTUI executes the agent loop with TUI output
func (a *Agent) runLoopTUI(ctx context.Context, adapter *tui.TUIAdapter) error {
	const maxIterations = 20
	loopStartTime := time.Now()
	interruptChan := adapter.GetInterruptChan()

	// Create cancellable context that responds to interrupt
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	// Monitor interrupt channel in goroutine
	go func() {
		select {
		case <-interruptChan:
			cancelRun()
		case <-runCtx.Done():
		}
	}()

	for i := 0; i < maxIterations; i++ {
		// Check for interrupt/cancellation before starting iteration
		select {
		case <-runCtx.Done():
			adapter.Warning("Interrupted by user")
			adapter.StreamDone() // Reset TUI state
			return nil           // Not an error, user chose to stop
		default:
		}

		// Send stats update at start of each iteration
		adapter.UpdateStats(tui.SessionStats{
			LoopIteration: i + 1,
			MaxIterations: maxIterations,
			LoopStartTime: loopStartTime,
		})

		// Get tool definitions
		toolDefs := a.getToolDefinitions()

		// Call LLM with cancellable context
		stream := a.llm.ChatStream(runCtx, a.contextMgr.GetMessages(), toolDefs, a.getSystemPrompt())

		var response llm.Response
		var textContent strings.Builder
		var toolCalls []llm.ToolCall
		interrupted := false

		// Process stream - use select to race between chunks and context cancellation
	streamLoop:
		for {
			select {
			case <-runCtx.Done():
				// Context cancelled (ESC pressed) - exit immediately
				interrupted = true
				break streamLoop

			case chunk, ok := <-stream:
				if !ok {
					// Channel closed, stream ended
					break streamLoop
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
						// Check if error is due to context cancellation
						if runCtx.Err() != nil {
							interrupted = true
							break streamLoop
						}
						adapter.StreamDone() // Reset TUI state before returning error
						return chunk.Error
					}
				}
			}
		}

		// Handle interruption after stream processing
		if interrupted {
			adapter.Warning("Interrupted by user")
			adapter.StreamDone() // Reset TUI state
			return nil           // Not an error, user chose to stop
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
		a.updateContextStatsTUI(runCtx, adapter)

		// If no tool calls, we're done
		if len(response.ToolCalls) == 0 {
			return nil
		}

		// Execute tool calls
		toolResults := a.executeToolCallsTUI(runCtx, response.ToolCalls, adapter)

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
		// Log tool call to debug tracer
		debug.ToolCall(call.Name, call.Input)

		tool, ok := a.tools.Get(call.Name)
		if !ok {
			debug.ToolResult(call.Name, false, 0)
			results = append(results, toolResult{
				Name:   call.Name,
				Result: fmt.Sprintf("Unknown tool: %s", call.Name),
				Error:  true,
			})
			continue
		}

		// Build description for permission prompt
		description := formatToolDescription(call.Name, call.Input)

		// Check permission FIRST (before showing tool call)
		allowed, err := a.checkPermissionTUI(call.Name, tool.Permission(), description, adapter)
		if err != nil {
			debug.ToolResult(call.Name, false, 0)
			results = append(results, toolResult{
				Name:   call.Name,
				Result: fmt.Sprintf("Permission error: %s", err),
				Error:  true,
			})
			adapter.ToolResult(call.Name, "Permission error: "+err.Error(), true)
			continue
		}

		if !allowed {
			debug.ToolResult(call.Name, false, 0)
			results = append(results, toolResult{
				Name:   call.Name,
				Result: "Permission denied by user",
				Error:  true,
			})
			adapter.ToolResult(call.Name, "Permission denied", true)
			continue
		}

		// Only show tool call AFTER permission granted
		adapter.ToolCall(call.Name, description)

		// Execute tool
		result, err := tool.Execute(ctx, call.Input)
		if err != nil {
			debug.ToolResult(call.Name, false, 0)
			results = append(results, toolResult{
				Name:   call.Name,
				Result: fmt.Sprintf("Error: %s", err),
				Error:  true,
			})
			adapter.ToolResult(call.Name, err.Error(), true)
		} else {
			debug.ToolResult(call.Name, true, len(result))
			// Store in cache if result is large and we're in analysis mode
			contextResult := result
			if a.resultCache != nil && ctxmgr.ShouldCache(result) {
				summary, _ := a.resultCache.Store(call.Name, call.Input, result)
				contextResult = summary // Use summary for LLM context
			}
			results = append(results, toolResult{
				Name:   call.Name,
				Result: contextResult,
				Error:  false,
			})
			adapter.ToolResult(call.Name, result, false) // Show full result to user
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
		runner.SendInfo("  /copy            Copy conversation to clipboard")
		runner.SendInfo("  /mode <tier>     Switch model (fast/smart/genius)")
		runner.SendInfo("  /architect       Toggle architect mode (Shift+Tab for plan/chat)")
		runner.SendInfo("  /plan <goal>     Enter plan mode")
		runner.SendInfo("  /skills          List available skills")
		runner.SendInfo("  /status          Check vecgrep status")
		runner.SendInfo("  /reindex         Update vecgrep search index")
		runner.SendInfo("  /context         Show context usage breakdown")
		runner.SendInfo("  /compact [focus] Compact conversation (optional focus)")
		runner.SendInfo("  /sessions        List saved sessions")
		runner.SendInfo("  /resume [id]     Resume a session (last if no id)")
		runner.SendInfo("  /new             Start a new session")
		runner.SendInfo("  /delete <id>     Delete a session")
		runner.SendInfo("  /clear           Clear conversation")
		runner.SendInfo("  /exit            Exit interactive mode")
		runner.SendInfo("")
		runner.SendInfo("Debug: Run with --debug or -d flag")
		return true

	case "/exit", "/quit":
		runner.SendInfo("Goodbye!")
		return false

	case "/clear":
		a.contextMgr.Clear()
		a.shownContextWarning = false
		runner.GetAdapter().Clear()
		runner.ClearQueue()
		runner.SendSuccess("Conversation and queue cleared")
		return true

	case "/copy":
		text := runner.GetConversationText()
		if text == "" {
			runner.SendInfo("No conversation to copy")
			return true
		}

		if err := copyToClipboard(text); err != nil {
			// Fallback: save to file
			filename := fmt.Sprintf("vecai-conversation-%s.txt", time.Now().Format("20060102-150405"))
			if err := os.WriteFile(filename, []byte(text), 0644); err != nil {
				runner.SendError("Failed to copy: " + err.Error())
				return true
			}
			runner.SendSuccess(fmt.Sprintf("Saved conversation to %s", filename))
			return true
		}
		runner.SendSuccess("Conversation copied to clipboard")
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
			runner.SendSuccess("Switched to fast mode (" + a.config.GetModel(config.TierFast) + ")")
		case "smart":
			a.llm.SetTier(config.TierSmart)
			runner.SendSuccess("Switched to smart mode (" + a.config.GetModel(config.TierSmart) + ")")
		case "genius":
			a.llm.SetTier(config.TierGenius)
			runner.SendSuccess("Switched to genius mode (" + a.config.GetModel(config.TierGenius) + ")")
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

	case "/reindex":
		runner.SendInfo("Reindexing codebase with vecgrep...")
		ctx := context.Background()
		result, err := a.reindexVecgrep(ctx)
		if err != nil {
			runner.SendError("Reindex failed: " + err.Error())
		} else {
			runner.SendSuccess(result)
		}
		return true

	case "/sessions":
		if a.sessionMgr == nil {
			runner.SendError("Session manager not available")
			return true
		}
		sessions, err := a.sessionMgr.List()
		if err != nil {
			runner.SendError("Failed to list sessions: " + err.Error())
			return true
		}
		if len(sessions) == 0 {
			runner.SendInfo("No saved sessions")
			return true
		}
		runner.SendInfo("Saved sessions:")
		for _, s := range sessions {
			bullet := "○"
			suffix := ""
			if curr := a.sessionMgr.GetCurrentSession(); curr != nil && curr.ID == s.ID {
				bullet = "●"
				suffix = " ← current"
			}
			relTime := session.FormatRelativeTime(s.UpdatedAt)
			preview := s.Preview
			if len(preview) > 35 {
				preview = preview[:32] + "..."
			}
			if preview != "" {
				preview = fmt.Sprintf(" \"%s\"", preview)
			}
			runner.SendInfo(fmt.Sprintf("  %s %s  %-8s  %2d msgs%s%s",
				bullet, s.ID[:8], relTime, s.MsgCount, preview, suffix))
		}
		return true

	case "/resume":
		if a.sessionMgr == nil {
			runner.SendError("Session manager not available")
			return true
		}
		var sess *session.Session
		var err error
		if len(parts) > 1 {
			// Resume specific session by ID prefix
			sessions, listErr := a.sessionMgr.List()
			if listErr != nil {
				runner.SendError("Failed to list sessions: " + listErr.Error())
				return true
			}
			prefix := parts[1]
			for _, s := range sessions {
				if strings.HasPrefix(s.ID, prefix) {
					sess, err = a.sessionMgr.Load(s.ID)
					break
				}
			}
			if sess == nil && err == nil {
				runner.SendError("No session found with prefix: " + prefix)
				return true
			}
		} else {
			// Resume last session
			sess, err = a.sessionMgr.GetCurrent()
		}
		if err != nil {
			runner.SendError("Failed to load session: " + err.Error())
			return true
		}
		if sess == nil {
			runner.SendInfo("No session to resume")
			return true
		}
		a.contextMgr.RestoreMessages(sess.Messages)
		a.sessionMgr.SetCurrent(sess)
		runner.GetAdapter().SetSessionID(sess.ID[:8])
		runner.SendSuccess(fmt.Sprintf("Resumed session %s (%d messages)", sess.ID[:8], len(sess.Messages)))
		// Show last exchange as context
		if len(sess.Messages) > 0 {
			for i := len(sess.Messages) - 1; i >= 0; i-- {
				if sess.Messages[i].Role == "user" {
					lastMsg := sess.Messages[i].Content
					if len(lastMsg) > 60 {
						lastMsg = lastMsg[:57] + "..."
					}
					runner.SendInfo(fmt.Sprintf("Last message: \"%s\"", lastMsg))
					break
				}
			}
		}
		return true

	case "/new":
		if a.sessionMgr == nil {
			runner.SendError("Session manager not available")
			return true
		}
		// Current session is already saved via auto-save, just start fresh
		a.contextMgr.Clear()
		a.shownContextWarning = false
		sess, err := a.sessionMgr.StartNew()
		if err != nil {
			runner.SendError("Failed to start new session: " + err.Error())
			return true
		}
		runner.GetAdapter().Clear()
		runner.ClearQueue()
		runner.GetAdapter().SetSessionID(sess.ID[:8])
		runner.SendSuccess(fmt.Sprintf("Started new session (%s)", sess.ID[:8]))
		return true

	case "/software-architect", "/architect":
		if a.architectMode {
			// Exit architect mode
			a.architectMode = false
			a.permissions.SetMode(a.previousPermMode) // Restore permissions
			runner.GetModel().SetArchitectMode(false)
			runner.SendSuccess("Exited architect mode")

			// Log mode change event
			if log := logging.Global(); log != nil {
				log.Event(logging.EventAgentModeChange,
					logging.From("architect"),
					logging.To("interactive"),
				)
			}
		} else {
			// Enter architect mode
			a.architectMode = true
			a.previousPermMode = a.permissions.GetMode()
			a.permissions.SetMode(permissions.ModeAsk) // Auto-approve reads; still prompt for writes/execute
			runner.GetModel().SetArchitectMode(true)
			runner.SendSuccess("Entered architect mode (reads auto-approved, writes/execute still prompt)")
			runner.SendInfo("Use Shift+Tab to toggle between Plan and Chat modes")
			runner.SendInfo("  Plan mode: Design and explore the codebase")
			runner.SendInfo("  Chat mode: Ask questions and discuss")

			// Log mode change event
			if log := logging.Global(); log != nil {
				log.Event(logging.EventAgentModeChange,
					logging.From("interactive"),
					logging.To("architect"),
				)
			}
		}
		return true

	case "/delete":
		if a.sessionMgr == nil {
			runner.SendError("Session manager not available")
			return true
		}
		if len(parts) < 2 {
			runner.SendError("Usage: /delete <session-id> [--force]")
			return true
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
			runner.SendError("Failed to list sessions: " + err.Error())
			return true
		}
		var found string
		for _, s := range sessions {
			if strings.HasPrefix(s.ID, prefix) {
				found = s.ID
				break
			}
		}
		if found == "" {
			runner.SendError("No session found with prefix: " + prefix)
			return true
		}
		// Check if deleting current session
		if curr := a.sessionMgr.GetCurrentSession(); curr != nil && curr.ID == found {
			if !forceDelete {
				runner.SendWarning(fmt.Sprintf("Session %s is currently active", found[:8]))
				runner.SendInfo("Use /delete " + prefix + " --force to confirm")
				return true
			}
			// Clear context since we're deleting current session
			a.contextMgr.Clear()
			a.shownContextWarning = false
		}
		if err := a.sessionMgr.Delete(found); err != nil {
			runner.SendError("Failed to delete session: " + err.Error())
			return true
		}
		runner.SendSuccess(fmt.Sprintf("Deleted session %s", found[:8]))
		return true

	default:
		runner.SendError("Unknown command: " + parts[0] + ". Type /help for available commands.")
		return true
	}
}

// checkVecgrepStatusTUI checks vecgrep status with TUI output
// Auto-initializes and indexes if not already set up
func (a *Agent) checkVecgrepStatusTUI(adapter *tui.TUIAdapter) {
	ctx := context.Background()

	// Notify user if project instructions were loaded
	if a.projectInstructions != "" {
		// Determine which file was loaded
		instructionFile := "AGENTS.md"
		if _, err := os.Stat("VECAI.md"); err == nil {
			instructionFile = "VECAI.md"
		}
		adapter.Info(fmt.Sprintf("Loaded project instructions from %s", instructionFile))
	}

	// Check for stale files
	staleCount := a.getVecgrepStaleCount(ctx)
	if staleCount > 0 {
		adapter.Warning(fmt.Sprintf("vecgrep index has %d modified files. Run /reindex for best search results.", staleCount))
		return
	}

	// Check if initialized
	tool, _ := a.tools.Get("vecgrep_status")
	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil {
		// Status check failed - try auto-init
		a.autoInitVecgrepTUI(ctx, adapter)
		return
	}

	if strings.Contains(result, "not initialized") {
		// Auto-initialize and index
		a.autoInitVecgrepTUI(ctx, adapter)
	}
	// Don't show "vecgrep index is ready" - it clutters the viewport with no ongoing value
}

// autoInitVecgrepTUI automatically initializes and indexes vecgrep with TUI output
func (a *Agent) autoInitVecgrepTUI(ctx context.Context, adapter *tui.TUIAdapter) {
	adapter.Info("Vecgrep not initialized. Auto-initializing...")

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		adapter.Warning("Failed to get working directory: " + err.Error())
		return
	}

	// Run vecgrep_init
	initTool, ok := a.tools.Get("vecgrep_init")
	if !ok {
		adapter.Warning("vecgrep_init tool not available")
		return
	}

	result, err := initTool.Execute(ctx, map[string]any{
		"path": wd,
	})
	if err != nil {
		adapter.Warning("vecgrep init failed: " + err.Error())
		return
	}
	adapter.Info("vecgrep_init: " + truncateResult(result, 80))

	// Run vecgrep_index
	indexTool, ok := a.tools.Get("vecgrep_index")
	if !ok {
		adapter.Warning("vecgrep_index tool not available")
		return
	}

	adapter.Info("Indexing codebase (this may take a moment)...")
	result, err = indexTool.Execute(ctx, map[string]any{})
	if err != nil {
		adapter.Warning("vecgrep index failed: " + err.Error())
		return
	}
	adapter.Success("Indexing complete: " + truncateResult(result, 80))
}

// truncateResult truncates a result string to maxLen characters
func truncateResult(result string, maxLen int) string {
	// Remove newlines for cleaner display
	result = strings.ReplaceAll(result, "\n", " ")
	result = strings.TrimSpace(result)
	if len(result) > maxLen {
		return result[:maxLen-3] + "..."
	}
	return result
}

// getVecgrepStaleCount returns the number of files that need reindexing
func (a *Agent) getVecgrepStaleCount(ctx context.Context) int {
	cmd := exec.CommandContext(ctx, "vecgrep", "status", "--format", "json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return 0 // Silently fail - don't interrupt startup
	}

	// Parse JSON to get reindex stats
	var status struct {
		ReindexStatus struct {
			NewFiles      int `json:"new_files"`
			ModifiedFiles int `json:"modified_files"`
		} `json:"reindex_status"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		return 0
	}

	return status.ReindexStatus.NewFiles + status.ReindexStatus.ModifiedFiles
}

// reindexVecgrep triggers a vecgrep index update
func (a *Agent) reindexVecgrep(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "vecgrep", "index")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not initialized") {
			return "", fmt.Errorf("vecgrep not initialized. Run 'vecgrep init' first")
		}
		return "", fmt.Errorf("%s", stderr.String())
	}

	// Parse output to get stats
	output := stdout.String()
	if output == "" {
		output = "Index updated successfully"
	}
	return output, nil
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
			a.output.Success("Switched to fast mode (" + a.config.GetModel(config.TierFast) + ")")
		case "smart":
			a.llm.SetTier(config.TierSmart)
			a.output.Success("Switched to smart mode (" + a.config.GetModel(config.TierSmart) + ")")
		case "genius":
			a.llm.SetTier(config.TierGenius)
			a.output.Success("Switched to genius mode (" + a.config.GetModel(config.TierGenius) + ")")
		default:
			a.output.ErrorStr("Unknown mode: " + parts[1])
		}
		return true

	case "/software-architect", "/architect":
		if a.architectMode {
			// Exit architect mode
			a.architectMode = false
			a.permissions.SetMode(a.previousPermMode) // Restore permissions
			a.output.Success("Exited architect mode")

			// Log mode change event
			if log := logging.Global(); log != nil {
				log.Event(logging.EventAgentModeChange,
					logging.From("architect"),
					logging.To("interactive"),
				)
			}
		} else {
			// Enter architect mode
			a.architectMode = true
			a.previousPermMode = a.permissions.GetMode()
			a.permissions.SetMode(permissions.ModeAsk) // Auto-approve reads; still prompt for writes/execute
			a.output.Success("Entered architect mode (reads auto-approved, writes/execute still prompt)")
			a.output.Info("In TUI mode, use Shift+Tab to toggle between Plan and Chat modes")

			// Log mode change event
			if log := logging.Global(); log != nil {
				log.Event(logging.EventAgentModeChange,
					logging.From("interactive"),
					logging.To("architect"),
				)
			}
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

	case "/sessions":
		if a.sessionMgr == nil {
			a.output.ErrorStr("Session manager not available")
			return true
		}
		sessions, err := a.sessionMgr.List()
		if err != nil {
			a.output.ErrorStr("Failed to list sessions: " + err.Error())
			return true
		}
		if len(sessions) == 0 {
			a.output.Info("No saved sessions")
			return true
		}
		a.output.Header("Saved Sessions")
		for _, s := range sessions {
			bullet := "○"
			suffix := ""
			if curr := a.sessionMgr.GetCurrentSession(); curr != nil && curr.ID == s.ID {
				bullet = "●"
				suffix = " ← current"
			}
			relTime := session.FormatRelativeTime(s.UpdatedAt)
			preview := s.Preview
			if len(preview) > 35 {
				preview = preview[:32] + "..."
			}
			if preview != "" {
				preview = fmt.Sprintf(" \"%s\"", preview)
			}
			a.output.TextLn(fmt.Sprintf("  %s %s  %-8s  %2d msgs%s%s",
				bullet, s.ID[:8], relTime, s.MsgCount, preview, suffix))
		}
		return true

	case "/resume":
		if a.sessionMgr == nil {
			a.output.ErrorStr("Session manager not available")
			return true
		}
		var sess *session.Session
		var err error
		if len(parts) > 1 {
			sessions, listErr := a.sessionMgr.List()
			if listErr != nil {
				a.output.ErrorStr("Failed to list sessions: " + listErr.Error())
				return true
			}
			prefix := parts[1]
			for _, s := range sessions {
				if strings.HasPrefix(s.ID, prefix) {
					sess, err = a.sessionMgr.Load(s.ID)
					break
				}
			}
			if sess == nil && err == nil {
				a.output.ErrorStr("No session found with prefix: " + prefix)
				return true
			}
		} else {
			sess, err = a.sessionMgr.GetCurrent()
		}
		if err != nil {
			a.output.ErrorStr("Failed to load session: " + err.Error())
			return true
		}
		if sess == nil {
			a.output.Info("No session to resume")
			return true
		}
		a.contextMgr.RestoreMessages(sess.Messages)
		a.sessionMgr.SetCurrent(sess)
		a.output.Success(fmt.Sprintf("Resumed session %s (%d messages)", sess.ID[:8], len(sess.Messages)))
		return true

	case "/new":
		if a.sessionMgr == nil {
			a.output.ErrorStr("Session manager not available")
			return true
		}
		a.contextMgr.Clear()
		a.shownContextWarning = false
		a.input.Clear()
		if _, err := a.sessionMgr.StartNew(); err != nil {
			a.output.ErrorStr("Failed to start new session: " + err.Error())
			return true
		}
		a.output.Success("Started new session")
		return true

	case "/delete":
		if a.sessionMgr == nil {
			a.output.ErrorStr("Session manager not available")
			return true
		}
		if len(parts) < 2 {
			a.output.ErrorStr("Usage: /delete <session-id> [--force]")
			return true
		}
		// Check for --force flag
		forceDelete := false
		prefix := parts[1]
		if len(parts) > 2 && parts[2] == "--force" {
			forceDelete = true
		}
		sessions, err := a.sessionMgr.List()
		if err != nil {
			a.output.ErrorStr("Failed to list sessions: " + err.Error())
			return true
		}
		var found string
		for _, s := range sessions {
			if strings.HasPrefix(s.ID, prefix) {
				found = s.ID
				break
			}
		}
		if found == "" {
			a.output.ErrorStr("No session found with prefix: " + prefix)
			return true
		}
		// Check if deleting current session
		if curr := a.sessionMgr.GetCurrentSession(); curr != nil && curr.ID == found {
			if !forceDelete {
				a.output.Warning(fmt.Sprintf("Session %s is currently active", found[:8]))
				a.output.TextLn("Use /delete " + prefix + " --force to confirm")
				return true
			}
			// Clear context since we're deleting current session
			a.contextMgr.Clear()
			a.shownContextWarning = false
		}
		if err := a.sessionMgr.Delete(found); err != nil {
			a.output.ErrorStr("Failed to delete session: " + err.Error())
			return true
		}
		a.output.Success(fmt.Sprintf("Deleted session %s", found[:8]))
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
		stream := a.llm.ChatStream(ctx, a.contextMgr.GetMessages(), toolDefs, a.getSystemPrompt())

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
		// Log tool call to debug tracer
		debug.ToolCall(call.Name, call.Input)

		tool, ok := a.tools.Get(call.Name)
		if !ok {
			debug.ToolResult(call.Name, false, 0)
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
			debug.ToolResult(call.Name, false, 0)
			results = append(results, toolResult{
				Name:   call.Name,
				Result: fmt.Sprintf("Permission error: %s", err),
				Error:  true,
			})
			a.output.ToolResult(call.Name, "Permission error: "+err.Error(), true)
			continue
		}

		if !allowed {
			debug.ToolResult(call.Name, false, 0)
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
			debug.ToolResult(call.Name, false, 0)
			results = append(results, toolResult{
				Name:   call.Name,
				Result: fmt.Sprintf("Error: %s", err),
				Error:  true,
			})
			a.output.ToolResult(call.Name, err.Error(), true)
		} else {
			debug.ToolResult(call.Name, true, len(result))
			// Store in cache if result is large and we're in analysis mode
			contextResult := result
			if a.resultCache != nil && ctxmgr.ShouldCache(result) {
				summary, _ := a.resultCache.Store(call.Name, call.Input, result)
				contextResult = summary // Use summary for LLM context
			}
			results = append(results, toolResult{
				Name:   call.Name,
				Result: contextResult,
				Error:  false,
			})
			a.output.ToolResult(call.Name, result, false) // Show full result to user
		}
	}

	return results
}

// getToolDefinitions converts tools to LLM format
// Uses smart selection in analysis mode with SmartToolSelection enabled
func (a *Agent) getToolDefinitions() []llm.ToolDefinition {
	var registryDefs []tools.ToolDefinition

	// Use smart tool selection when in analysis mode with smart selection enabled
	if a.analysisMode && a.config.Analysis.SmartToolSelection && a.currentQuery != "" {
		registryDefs = a.toolSelector.SelectTools(a.currentQuery)
	} else {
		registryDefs = a.tools.GetDefinitions()
	}

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

	// Notify user if project instructions were loaded
	if a.projectInstructions != "" {
		instructionFile := "AGENTS.md"
		if _, err := os.Stat("VECAI.md"); err == nil {
			instructionFile = "VECAI.md"
		}
		a.output.Info(fmt.Sprintf("Loaded project instructions from %s", instructionFile))
	}

	tool, _ := a.tools.Get("vecgrep_status")
	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil {
		a.output.Warning("vecgrep status check failed: " + err.Error())
		return
	}

	if strings.Contains(result, "not initialized") {
		a.output.Warning("vecgrep is not initialized. Run 'vecgrep init' for semantic search.")
	}
	// Don't show "vecgrep index is ready" - it clutters the output with no ongoing value
}

// showHelp displays help information
func (a *Agent) showHelp() {
	a.output.Header("Commands")
	a.output.TextLn("/help            Show this help")
	a.output.TextLn("/mode <tier>     Switch model (fast/smart/genius)")
	a.output.TextLn("/architect       Toggle architect mode (Shift+Tab for plan/chat)")
	a.output.TextLn("/plan <goal>     Enter plan mode")
	a.output.TextLn("/skills          List available skills")
	a.output.TextLn("/status          Check vecgrep status")
	a.output.TextLn("/reindex         Update vecgrep search index")
	a.output.TextLn("/context         Show context usage breakdown")
	a.output.TextLn("/compact [focus] Compact conversation (optional focus prompt)")
	a.output.TextLn("/sessions        List saved sessions")
	a.output.TextLn("/resume [id]     Resume a session (last if no id)")
	a.output.TextLn("/new             Start a new session")
	a.output.TextLn("/delete <id>     Delete a session")
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

// getSystemPrompt returns the appropriate system prompt based on analysis mode
// Appends project-specific instructions from VECAI.md or AGENTS.md if present
// Also includes memory context enrichment when available
func (a *Agent) getSystemPrompt() string {
	base := systemPrompt
	if a.analysisMode {
		base = analysisSystemPrompt
	}

	var sections []string

	// Existing: project instructions
	if a.projectInstructions != "" {
		sections = append(sections, "## Project-Specific Instructions\n\n"+a.projectInstructions)
	}

	// NEW: memory context enrichment
	if a.memoryLayer != nil && a.currentQuery != "" {
		if ctx := a.memoryLayer.GetContextEnrichment(a.currentQuery); ctx != "" {
			sections = append(sections, ctx)
		}
	}

	if len(sections) > 0 {
		return base + "\n\n" + strings.Join(sections, "\n\n")
	}
	return base
}

// loadProjectInstructions looks for VECAI.md or AGENTS.md in the working directory
// Returns the file contents if found, empty string otherwise
func loadProjectInstructions() string {
	// Try VECAI.md first (project-specific)
	if content, err := os.ReadFile("VECAI.md"); err == nil {
		return string(content)
	}

	// Fall back to AGENTS.md (generic agent instructions)
	if content, err := os.ReadFile("AGENTS.md"); err == nil {
		return string(content)
	}

	return ""
}

// offerCapture prompts the user to save a response to notes
func (a *Agent) offerCapture(ctx context.Context, query, response string) error {
	// Skip if noted is not available
	tool, ok := a.tools.Get("noted_remember")
	if !ok {
		return nil
	}

	// Prompt user
	a.output.TextLn("")
	input, err := a.input.ReadInput("Save to notes? [y/N/e(dit)] ")
	if err != nil {
		return err
	}

	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" || input == "n" || input == "no" {
		return nil
	}

	// Auto-generate tags from query
	tags := generateTags(query)

	content := response
	if input == "e" || input == "edit" {
		// Let user edit the content
		content, err = a.input.ReadMultiLine("Enter content (empty line to finish): ")
		if err != nil {
			return err
		}
	}

	// Save to notes
	_, err = tool.Execute(ctx, map[string]any{
		"content":    content,
		"tags":       tags,
		"importance": 0.6, // Slightly above default for user-saved content
	})
	if err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	a.output.Success("Saved to notes")
	return nil
}

// generateTags creates tags from a query string
func generateTags(query string) []any {
	queryLower := strings.ToLower(query)

	var tags []any

	// Category tags based on keywords
	if strings.Contains(queryLower, "config") || strings.Contains(queryLower, "setting") {
		tags = append(tags, "config")
	}
	if strings.Contains(queryLower, "code") || strings.Contains(queryLower, "function") || strings.Contains(queryLower, "implement") {
		tags = append(tags, "code")
	}
	if strings.Contains(queryLower, "bug") || strings.Contains(queryLower, "fix") || strings.Contains(queryLower, "error") {
		tags = append(tags, "debug")
	}
	if strings.Contains(queryLower, "prefer") || strings.Contains(queryLower, "like") || strings.Contains(queryLower, "want") {
		tags = append(tags, "preference")
	}
	if strings.Contains(queryLower, "remember") || strings.Contains(queryLower, "note") {
		tags = append(tags, "note")
	}

	// Always add a general tag if no specific ones matched
	if len(tags) == 0 {
		tags = append(tags, "general")
	}

	return tags
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

// Close cleans up agent resources
func (a *Agent) Close() error {
	if a.memoryLayer != nil {
		return a.memoryLayer.Close()
	}
	return nil
}

// detectAndRecordCorrection checks if user message looks like a correction and records it
func (a *Agent) detectAndRecordCorrection(userMsg string) {
	if a.memoryLayer == nil {
		return
	}
	patterns := []string{"no,", "wrong", "that's not", "actually", "instead", "should be", "not correct", "incorrect"}
	lower := strings.ToLower(userMsg)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			a.memoryLayer.RecordError("agent_correction", userMsg)
			return
		}
	}
}

// copyToClipboard copies text to system clipboard
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		}
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := stdin.Write([]byte(text)); err != nil {
		_ = stdin.Close()
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	return cmd.Wait()
}

// logDebug logs a debug message using the new logging package.
// This is a helper to bridge printf-style calls to structured logging.
func logDebug(format string, args ...any) {
	if log := logging.Global(); log != nil {
		log.Debug(fmt.Sprintf(format, args...))
	}
}

// logWarn logs a warning message using the new logging package.
func logWarn(format string, args ...any) {
	if log := logging.Global(); log != nil {
		log.Warn(fmt.Sprintf(format, args...))
	}
}
