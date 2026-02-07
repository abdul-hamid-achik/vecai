package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	ctxmgr "github.com/abdul-hamid-achik/vecai/internal/context"
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
	toolExecutor        *ToolExecutor
	commandHandler      *CommandHandler
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

	// Graceful shutdown
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
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

	// Create shutdown context for graceful cleanup
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

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
		shutdownCtx:         shutdownCtx,
		shutdownCancel:      shutdownCancel,
	}
	a.toolExecutor = NewToolExecutor(cfg.Tools, cfg.Permissions, resultCache, cfg.AnalysisMode)
	a.commandHandler = NewCommandHandler(a)
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
	cliOutput := &CLIOutput{Out: a.output, In: a.input}
	err := a.runAgentLoop(ctx, cliOutput, cliOutput)
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
			if !a.commandHandler.Handle(input, &TUIOutput{Adapter: runner.GetAdapter()}, &TUICommandContext{Runner: runner}) {
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
			if !a.commandHandler.Handle(input, &CLIOutput{Out: a.output, In: a.input}, &CLICommandContext{Input: a.input}) {
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
			if !a.commandHandler.Handle(input, &TUIOutput{Adapter: runner.GetAdapter()}, &TUICommandContext{Runner: runner}) {
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

	tuiOutput := &TUIOutput{Adapter: adapter}
	return a.runAgentLoop(ctx, tuiOutput, tuiOutput)
}

// RunPlan runs in plan mode
func (a *Agent) RunPlan(goal string) error {
	return a.planner.Execute(goal)
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

// ClearHistory clears conversation history
func (a *Agent) ClearHistory() {
	a.contextMgr.Clear()
	a.shownContextWarning = false
}

// GetHistory returns conversation history
func (a *Agent) GetHistory() []llm.Message {
	return a.contextMgr.GetMessages()
}

// Close cleans up agent resources gracefully.
func (a *Agent) Close() error {
	// Signal all goroutines to stop
	if a.shutdownCancel != nil {
		a.shutdownCancel()
	}

	// Save session if available
	if a.sessionMgr != nil {
		msgs := a.contextMgr.GetMessages()
		if len(msgs) > 0 {
			if err := a.sessionMgr.Save(msgs, a.llm.GetModel()); err != nil {
				if log := logging.Global(); log != nil {
					log.Warn("failed to save session during shutdown", logging.Error(err))
				}
			}
		}
	}

	// Close memory layer
	if a.memoryLayer != nil {
		if err := a.memoryLayer.Close(); err != nil {
			if log := logging.Global(); log != nil {
				log.Warn("failed to close memory layer", logging.Error(err))
			}
		}
	}

	// Close LLM client
	if a.llm != nil {
		if err := a.llm.Close(); err != nil {
			if log := logging.Global(); log != nil {
				log.Warn("failed to close LLM client", logging.Error(err))
			}
		}
	}

	// Flush logs
	if log := logging.Global(); log != nil {
		log.Debug("agent shutdown complete")
	}

	return nil
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
