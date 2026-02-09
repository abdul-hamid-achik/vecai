package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	rtdebug "runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/agent"
	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/debug"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/logging"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/skills"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/ui"
)

var Version = "dev"

func main() {
	// Set Go runtime memory limit to 3GB to prevent unbounded growth
	rtdebug.SetMemoryLimit(3 << 30)

	// Check for --debug and --verbose flags early (before other parsing)
	debugMode := os.Getenv("VECAI_DEBUG") == "1"
	verboseMode := false
	for _, arg := range os.Args[1:] {
		if arg == "--debug" || arg == "-d" {
			debugMode = true
		}
		if arg == "--verbose" || arg == "-V" {
			verboseMode = true
		}
	}

	// Initialize the new unified logging system
	logConfig := logging.ConfigFromEnv()
	if debugMode {
		logConfig = logConfig.WithDebugMode(true)
	}
	if verboseMode {
		logConfig = logConfig.WithVerbose(true)
	}

	log, err := logging.Init(logConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to init logging: %v\n", err)
	}
	defer logging.Close()

	// Also initialize legacy debug tracer for backwards compatibility
	// This can be removed after full migration
	if debugMode {
		if err := debug.Init(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to init debug tracer: %v\n", err)
		}
		defer debug.Close()
	}

	// Log session start
	if log != nil {
		log.Event(logging.EventSessionStart,
			logging.F("version", Version),
			logging.F("debug_mode", debugMode),
			logging.F("verbose_mode", verboseMode),
		)
	}

	// Initialize secure project root for file path validation
	if err := tools.InitProjectRoot(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to init project root: %v\n", err)
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse command line args
	args := os.Args[1:]

	// Handle version flag (before logging to avoid log file for simple version checks)
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-v" || args[0] == "version") {
		fmt.Printf("vecai version %s\n", Version)
		return nil
	}

	// Initialize logging - log session start (after version check to avoid creating logs for --version)
	logDebug("vecai session started, args=%v", args)

	// Handle help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		printHelp()
		return nil
	}

	// Parse debug and verbose flags (already handled in main, just remove from args)
	for i := 0; i < len(args); i++ {
		if args[i] == "-d" || args[i] == "--debug" || args[i] == "-V" || args[i] == "--verbose" {
			args = append(args[:i], args[i+1:]...)
			i--
		}
	}

	// Parse quick mode flag (-q/--quick)
	quickMode := false
	for i := 0; i < len(args); i++ {
		if args[i] == "-q" || args[i] == "--quick" {
			quickMode = true
			args = append(args[:i], args[i+1:]...)
			i--
		}
	}

	// Parse capture mode flag (-c/--capture)
	captureMode := false
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" || args[i] == "--capture" {
			captureMode = true
			args = append(args[:i], args[i+1:]...)
			i--
		}
	}

	// Parse CLI flags
	var loadOpts config.LoadOptions
	for i := 0; i < len(args); i++ {
		if args[i] == "--model" && i+1 < len(args) {
			loadOpts.ModelOverride = args[i+1]
			args = append(args[:i], args[i+2:]...)
			i--
			continue
		}
		if strings.HasPrefix(args[i], "--model=") {
			loadOpts.ModelOverride = strings.TrimPrefix(args[i], "--model=")
			args = append(args[:i], args[i+1:]...)
			i--
			continue
		}
		if args[i] == "--ollama-url" && i+1 < len(args) {
			loadOpts.BaseURLOverride = args[i+1]
			args = append(args[:i], args[i+2:]...)
			i--
			continue
		}
		if strings.HasPrefix(args[i], "--ollama-url=") {
			loadOpts.BaseURLOverride = strings.TrimPrefix(args[i], "--ollama-url=")
			args = append(args[:i], args[i+1:]...)
			i--
			continue
		}
	}

	// Load configuration
	cfg, err := config.LoadWithOptions(loadOpts)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine permission mode and analysis mode
	permMode := permissions.ModeAsk
	analysisMode := cfg.Analysis.Enabled // Default from config
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--auto":
			permMode = permissions.ModeAuto
			args = append(args[:i], args[i+1:]...)
			i--
		case "--strict":
			permMode = permissions.ModeStrict
			args = append(args[:i], args[i+1:]...)
			i--
		case "--analyze", "-a":
			analysisMode = true
			permMode = permissions.ModeAnalysis
			args = append(args[:i], args[i+1:]...)
			i--
		}
	}

	// Initialize components
	output := ui.NewOutputHandler()
	input := ui.NewInputHandler()

	// Create Ollama client with resilience wrapper (retry + circuit breaker)
	rawClient := llm.NewClient(cfg)
	llmClient := llm.NewResilientClient(rawClient, cfg.RateLimit)

	// Health check for Ollama connectivity and model warming
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if version, err := rawClient.CheckHealthWithVersion(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Ollama is not running. Start with: ollama serve\n")
		} else {
			logDebug("Ollama connected: version %s", version)
			// Warm the model in the background so it's loaded by the time the user sends a query.
			// This is especially useful with short OLLAMA_KEEP_ALIVE values (e.g. 2m via launchctl).
			go func() {
				warmCtx, warmCancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer warmCancel()
				model := rawClient.GetModel()
				logDebug("Warming model %s...", model)
				if err := rawClient.WarmModel(warmCtx); err != nil {
					logDebug("Model warm failed (non-fatal): %v", err)
				} else {
					logDebug("Model %s warmed and ready", model)
				}
			}()
		}
	}

	// Select registry based on mode
	var registry *tools.Registry
	if analysisMode {
		registry = tools.NewAnalysisRegistry(&cfg.Tools)
		logDebug("Using analysis registry (read-only tools)")
	} else {
		registry = tools.NewRegistry(&cfg.Tools)
	}
	policy := permissions.NewPolicy(permMode, input, output)
	skillLoader := skills.NewLoader()

	// Create agent
	a := agent.New(agent.Config{
		LLM:          llmClient,
		Tools:        registry,
		Permissions:  policy,
		Skills:       skillLoader,
		Output:       output,
		Input:        input,
		Config:       cfg,
		AnalysisMode: analysisMode,
		AutoTier:     true,        // Enable smart tier selection by default
		CaptureMode:  captureMode, // Prompt to save responses to notes
	})
	defer func() {
		if err := a.Close(); err != nil {
			logDebug("error closing agent: %v", err)
		}
	}()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logDebug("received signal %v, shutting down gracefully...", sig)
		if err := a.Close(); err != nil {
			logDebug("error during signal shutdown: %v", err)
		}
		os.Exit(0)
	}()

	// Handle models subcommand
	if len(args) > 0 && args[0] == "models" {
		return handleModelsCommand(cfg, args[1:])
	}

	// Quick mode: fast response, no tools
	if quickMode && len(args) > 0 {
		query := joinArgs(args)
		logDebug("Quick mode with query: %s", query)
		return a.RunQuick(query)
	}

	// Check for plan mode
	if len(args) > 0 && args[0] == "plan" {
		if len(args) < 2 {
			return fmt.Errorf("plan command requires a goal argument")
		}
		goal := args[1]
		logDebug("Entering plan mode with goal: %s", goal)
		return a.RunPlan(goal)
	}

	// One-shot mode if query provided
	if len(args) > 0 {
		query := joinArgs(args)
		logDebug("One-shot mode with query: %s", query)
		return a.Run(query)
	}

	// Interactive mode - use TUI if available, otherwise fall back to line mode
	logDebug("Entering interactive mode")
	return a.RunInteractiveTUI()
}

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

func printHelp() {
	fmt.Print(`vecai - Local AI-powered codebase assistant (Ollama)

Usage:
  vecai [query]           Run a one-shot query
  vecai                   Start interactive mode
  vecai plan <goal>       Create and execute a plan
  vecai models <cmd>      Manage Ollama models (list/test/pull)
  vecai version           Show version
  vecai help              Show this help

Flags:
  -q, --quick             Quick mode: fast response, no tools (for simple questions)
  -c, --capture           Capture mode: prompt to save responses to notes
  --model <name>          Override model (e.g., "qwen3:8b", "qwen3:14b")
  --ollama-url <url>      Override Ollama URL (default: http://localhost:11434)
  --auto                  Auto-approve all tool executions
  --strict                Prompt for all tool executions (including reads)
  --analyze, -a           Token-efficient analysis mode (read-only, minimal prompt)
  --debug, -d             Enable debug tracing to /tmp/vecai-debug/
  --verbose, -V           Enable verbose logging (debug level without full tracing)
  -v, --version           Show version
  -h, --help              Show help

Analysis Mode (--analyze):
  Optimized for code reviews and understanding:
  - Minimal system prompt (~300 vs ~2000 tokens)
  - Read-only tools only (no writes/executes)
  - Aggressive context compaction (70% vs 95%)
  - Auto-approve all reads, block all writes

Interactive Commands:
  /help                   Show help
  /plan <goal>            Create a plan
  /mode <fast|smart|genius>  Switch model tier
  /clear                  Clear conversation
  /exit                   Exit interactive mode

Model Tiers (default models):
  fast     llama3.2:3b       Fast responses, good for simple tasks
  smart    qwen3:8b          Balanced, code-focused
  genius   qwen3:14b         Most capable, complex reasoning

Environment:
  OLLAMA_HOST             Ollama server URL (overrides config)
  VECAI_DEBUG=1           Enable debug tracing (prefer --debug flag)
  VECAI_DEBUG_DIR         Override debug log directory
  VECAI_DEBUG_LLM=1       Enable full LLM payload logging

Config Files (in priority order):
  ./vecai.yaml
  ./.vecai/config.yaml
  ~/.config/vecai/config.yaml

Prerequisites:
  1. Install Ollama: https://ollama.ai
  2. Start Ollama: ollama serve
  3. Pull a model: ollama pull qwen3:8b
`)
}

// logDebug logs a debug message using the new logging package.
// This is a helper to bridge printf-style calls to structured logging.
func logDebug(format string, args ...any) {
	if log := logging.Global(); log != nil {
		log.Debug(fmt.Sprintf(format, args...))
	}
}
