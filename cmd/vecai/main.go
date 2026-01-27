package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/agent"
	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/debug"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/logger"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/skills"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/ui"
)

var Version = "dev"

func main() {
	// Ensure log file is closed on exit
	defer logger.CloseLogFile()

	// Initialize debug tracer from env var
	if os.Getenv("VECAI_DEBUG") == "1" {
		if err := debug.Init(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to init debug tracer: %v\n", err)
		}
		defer debug.Close()
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
	logger.Debug("vecai session started, args=%v", args)

	// Handle help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		printHelp()
		return nil
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

	// Create Ollama client
	llmClient := llm.NewClient(cfg)

	// Select registry based on mode
	var registry *tools.Registry
	if analysisMode {
		registry = tools.NewAnalysisRegistry()
		logger.Debug("Using analysis registry (read-only tools)")
	} else {
		registry = tools.NewRegistry()
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
	})

	// Check for plan mode
	if len(args) > 0 && args[0] == "plan" {
		if len(args) < 2 {
			return fmt.Errorf("plan command requires a goal argument")
		}
		goal := args[1]
		logger.Debug("Entering plan mode with goal: %s", goal)
		return a.RunPlan(goal)
	}

	// One-shot mode if query provided
	if len(args) > 0 {
		query := joinArgs(args)
		logger.Debug("One-shot mode with query: %s", query)
		return a.Run(query)
	}

	// Interactive mode - use TUI if available, otherwise fall back to line mode
	logger.Debug("Entering interactive mode")
	return a.RunInteractiveTUI()
}

func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}

func printHelp() {
	fmt.Print(`vecai - Local AI-powered codebase assistant (Ollama)

Usage:
  vecai [query]           Run a one-shot query
  vecai                   Start interactive mode
  vecai plan <goal>       Create and execute a plan
  vecai version           Show version
  vecai help              Show this help

Flags:
  --model <name>          Override model (e.g., "qwen3:8b", "cogito:14b")
  --ollama-url <url>      Override Ollama URL (default: http://localhost:11434)
  --auto                  Auto-approve all tool executions
  --strict                Prompt for all tool executions (including reads)
  --analyze, -a           Token-efficient analysis mode (read-only, minimal prompt)
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
  fast     qwen3:8b          Fast responses, good for simple tasks
  smart    qwen2.5-coder:7b  Balanced, code-focused
  genius   cogito:14b        Most capable, complex reasoning

Environment:
  OLLAMA_HOST             Ollama server URL (overrides config)
  VECAI_DEBUG=1           Enable debug tracing to /tmp/vecai-debug/
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
