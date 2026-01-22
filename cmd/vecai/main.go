package main

import (
	"fmt"
	"os"

	"github.com/abdulachik/vecai/internal/agent"
	"github.com/abdulachik/vecai/internal/config"
	"github.com/abdulachik/vecai/internal/llm"
	"github.com/abdulachik/vecai/internal/permissions"
	"github.com/abdulachik/vecai/internal/skills"
	"github.com/abdulachik/vecai/internal/tools"
	"github.com/abdulachik/vecai/internal/ui"
)

var Version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse command line args
	args := os.Args[1:]

	// Handle version flag
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-v" || args[0] == "version") {
		fmt.Printf("vecai version %s\n", Version)
		return nil
	}

	// Handle help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		printHelp()
		return nil
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine permission mode
	permMode := permissions.ModeAsk
	for i, arg := range args {
		if arg == "--auto" {
			permMode = permissions.ModeAuto
			args = append(args[:i], args[i+1:]...)
			break
		}
		if arg == "--strict" {
			permMode = permissions.ModeStrict
			args = append(args[:i], args[i+1:]...)
			break
		}
	}

	// Initialize components
	output := ui.NewOutputHandler()
	input := ui.NewInputHandler()
	llmClient := llm.NewClient(cfg)
	registry := tools.NewRegistry()
	policy := permissions.NewPolicy(permMode, input, output)
	skillLoader := skills.NewLoader()

	// Create agent
	a := agent.New(agent.Config{
		LLM:         llmClient,
		Tools:       registry,
		Permissions: policy,
		Skills:      skillLoader,
		Output:      output,
		Input:       input,
		Config:      cfg,
	})

	// Check for plan mode
	if len(args) > 0 && args[0] == "plan" {
		if len(args) < 2 {
			return fmt.Errorf("plan command requires a goal argument")
		}
		goal := args[1]
		return a.RunPlan(goal)
	}

	// One-shot mode if query provided
	if len(args) > 0 {
		query := joinArgs(args)
		return a.Run(query)
	}

	// Interactive mode
	return a.RunInteractive()
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
  --auto                  Auto-approve all tool executions
  --strict                Prompt for all tool executions (including reads)
  -v, --version           Show version
  -h, --help              Show help

Interactive Commands:
  /help                   Show help
  /plan <goal>            Create a plan
  /mode <fast|smart|genius>  Switch model tier
  /clear                  Clear conversation
  /exit                   Exit interactive mode

Environment:
  ANTHROPIC_API_KEY       Required: Your Anthropic API key

Config Files (in priority order):
  ./vecai.yaml
  ./.vecai/config.yaml
  ~/.config/vecai/config.yaml
`)
}
