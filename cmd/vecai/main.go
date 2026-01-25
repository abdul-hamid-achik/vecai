package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/agent"
	"github.com/abdul-hamid-achik/vecai/internal/config"
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

	// Parse --token flag before config load
	var token string
	for i := 0; i < len(args); i++ {
		if args[i] == "--token" && i+1 < len(args) {
			token = args[i+1]
			args = append(args[:i], args[i+2:]...)
			break
		}
		if strings.HasPrefix(args[i], "--token=") {
			token = strings.TrimPrefix(args[i], "--token=")
			args = append(args[:i], args[i+1:]...)
			break
		}
	}

	// Load configuration with token override
	cfg, err := config.LoadWithOptions(config.LoadOptions{
		TokenOverride: token,
	})
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
	baseClient := llm.NewClient(cfg)

	// Wrap with rate limiting if enabled
	var llmClient llm.LLMClient = baseClient
	if cfg.RateLimit.EnableRateLimiting {
		rateLimitedClient := llm.NewRateLimitedClient(baseClient, &cfg.RateLimit)

		// Set up spinner for rate limit feedback
		spinner := ui.NewSpinner(output)
		rateLimitedClient.SetWaitCallback(func(ctx context.Context, info llm.WaitInfo) error {
			return spinner.Start(ctx, ui.SpinnerConfig{
				Message:     "Rate limited",
				Reason:      info.Reason,
				Duration:    info.Duration,
				Attempt:     info.Attempt,
				MaxAttempts: info.MaxAttempts,
			})
		})

		llmClient = rateLimitedClient
	}

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
	fmt.Print(`vecai - AI-powered codebase assistant

Usage:
  vecai [query]           Run a one-shot query
  vecai                   Start interactive mode
  vecai plan <goal>       Create and execute a plan
  vecai version           Show version
  vecai help              Show this help

Flags:
  --token <key>           API key (overrides ANTHROPIC_API_KEY env var)
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

Environment:
  ANTHROPIC_API_KEY       API key (can be overridden with --token flag)

Config Files (in priority order):
  ./vecai.yaml
  ./.vecai/config.yaml
  ~/.config/vecai/config.yaml
`)
}
