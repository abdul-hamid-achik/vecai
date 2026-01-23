package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/skills"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
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
	messages    []llm.Message
	planner     *Planner
}

// New creates a new agent
func New(cfg Config) *Agent {
	a := &Agent{
		llm:         cfg.LLM,
		tools:       cfg.Tools,
		permissions: cfg.Permissions,
		skills:      cfg.Skills,
		output:      cfg.Output,
		input:       cfg.Input,
		config:      cfg.Config,
		messages:    []llm.Message{},
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
	a.messages = append(a.messages, llm.Message{
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
		a.messages = []llm.Message{}
		a.input.Clear()
		a.output.Success("Conversation cleared")
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
		stream := a.llm.ChatStream(ctx, a.messages, toolDefs, systemPrompt)

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
			a.messages = append(a.messages, llm.Message{
				Role:    "assistant",
				Content: response.Content,
			})
		}

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

		a.messages = append(a.messages, llm.Message{
			Role:    "user",
			Content: resultContent.String(),
		})
	}

	return fmt.Errorf("max iterations reached")
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
	a.output.TextLn("/help          Show this help")
	a.output.TextLn("/mode <tier>   Switch model (fast/smart/genius)")
	a.output.TextLn("/plan <goal>   Enter plan mode")
	a.output.TextLn("/skills        List available skills")
	a.output.TextLn("/status        Check vecgrep status")
	a.output.TextLn("/clear         Clear conversation")
	a.output.TextLn("/exit          Exit interactive mode")
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
	a.messages = []llm.Message{}
}

// GetHistory returns conversation history
func (a *Agent) GetHistory() []llm.Message {
	return a.messages
}
