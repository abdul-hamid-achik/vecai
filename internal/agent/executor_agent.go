package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// ExecutionResult represents the result of executing a step
type ExecutionResult struct {
	StepID    string
	Success   bool
	Output    string
	Error     error
	ToolCalls []ToolCallResult
}

// ToolCallResult represents the result of a single tool call
type ToolCallResult struct {
	Tool   string
	Input  map[string]any
	Output string
	Error  error
}

// ExecutorAgent executes individual plan steps
type ExecutorAgent struct {
	client      llm.LLMClient
	config      *config.Config
	registry    *tools.Registry
	permissions *permissions.Policy
}

// NewExecutorAgent creates a new executor agent
func NewExecutorAgent(client llm.LLMClient, cfg *config.Config, registry *tools.Registry, perms *permissions.Policy) *ExecutorAgent {
	return &ExecutorAgent{
		client:      client,
		config:      cfg,
		registry:    registry,
		permissions: perms,
	}
}

// ExecuteStep executes a single plan step
func (e *ExecutorAgent) ExecuteStep(ctx context.Context, step *PlanStep, previousContext string) (*ExecutionResult, error) {
	logDebug("ExecutorAgent: executing step %s: %s", step.ID, step.Description)

	// Use smart model for code execution
	originalModel := e.client.GetModel()
	e.client.SetTier(config.TierSmart)
	defer e.client.SetModel(originalModel)

	result := &ExecutionResult{
		StepID: step.ID,
	}

	// Build system prompt based on step type
	systemPrompt := e.buildSystemPrompt(step.Type)

	// Build user prompt with context
	userPrompt := e.buildUserPrompt(step, previousContext)

	messages := []llm.Message{
		{Role: "user", Content: userPrompt},
	}

	// Get appropriate tools based on step type
	toolDefs := e.getToolsForStepType(step.Type)

	// Execute with tool loop
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		resp, err := e.client.Chat(ctx, messages, toolDefs, systemPrompt)
		if err != nil {
			result.Error = fmt.Errorf("LLM call failed: %w", err)
			return result, nil
		}

		// Add assistant response
		if resp.Content != "" {
			result.Output += resp.Content
		}

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 {
			result.Success = true
			break
		}

		// Process tool calls
		for _, tc := range resp.ToolCalls {
			toolResult := e.executeToolCall(ctx, tc)
			result.ToolCalls = append(result.ToolCalls, toolResult)

			if toolResult.Error != nil {
				logWarn("ExecutorAgent: tool %s failed: %v", tc.Name, toolResult.Error)
			}

			// Add tool result to conversation
			messages = append(messages, llm.Message{
				Role:    "assistant",
				Content: fmt.Sprintf("[Called %s]", tc.Name),
			})
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool result for %s:\n%s", tc.Name, toolResult.Output),
			})
		}
	}

	logDebug("ExecutorAgent: step %s completed with %d tool calls", step.ID, len(result.ToolCalls))
	return result, nil
}

// ExecuteDirectTask executes a task without a structured plan
func (e *ExecutorAgent) ExecuteDirectTask(ctx context.Context, task string, intent Intent) (*ExecutionResult, error) {
	logDebug("ExecutorAgent: executing direct task with intent %s", intent)

	// Select model based on intent
	originalModel := e.client.GetModel()
	switch intent {
	case IntentCode:
		e.client.SetTier(config.TierSmart)
	case IntentQuestion:
		e.client.SetTier(config.TierFast)
	default:
		e.client.SetTier(config.TierSmart)
	}
	defer e.client.SetModel(originalModel)

	result := &ExecutionResult{
		StepID: "direct",
	}

	systemPrompt := e.buildSystemPromptForIntent(intent)
	toolDefs := e.getToolsForIntent(intent)

	messages := []llm.Message{
		{Role: "user", Content: task},
	}

	// Execute with tool loop
	maxIterations := 15
	for i := 0; i < maxIterations; i++ {
		resp, err := e.client.Chat(ctx, messages, toolDefs, systemPrompt)
		if err != nil {
			result.Error = fmt.Errorf("LLM call failed: %w", err)
			return result, nil
		}

		if resp.Content != "" {
			result.Output += resp.Content
		}

		if len(resp.ToolCalls) == 0 {
			result.Success = true
			break
		}

		// Process tool calls
		for _, tc := range resp.ToolCalls {
			toolResult := e.executeToolCall(ctx, tc)
			result.ToolCalls = append(result.ToolCalls, toolResult)

			messages = append(messages, llm.Message{
				Role:    "assistant",
				Content: fmt.Sprintf("[Called %s]", tc.Name),
			})
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool result for %s:\n%s", tc.Name, toolResult.Output),
			})
		}
	}

	return result, nil
}

func (e *ExecutorAgent) buildSystemPrompt(stepType string) string {
	base := `You are an executor agent that performs specific tasks as part of a larger plan.

Your current task type is: %s

Guidelines:
- Focus ONLY on the current step
- Use tools to accomplish the task
- Be thorough but efficient
- Report any issues encountered
- Do not deviate from the task description
`
	return fmt.Sprintf(base, stepType)
}

func (e *ExecutorAgent) buildSystemPromptForIntent(intent Intent) string {
	switch intent {
	case IntentCode:
		return `You are a code-focused AI assistant. Write clean, well-documented code.
Use tools to read existing code, then write or modify files as needed.
Follow existing patterns in the codebase.`

	case IntentQuestion:
		return `You are a helpful AI assistant that answers questions about code.
Use tools to explore the codebase and find relevant information.
Provide clear, concise answers with references to specific files and lines.`

	case IntentDebug:
		return `You are a debugging assistant. Help identify and fix issues.
Use tools to read code, run tests, and check for errors.
Explain what you find and suggest fixes.`

	default:
		return `You are a helpful AI coding assistant. Use the available tools to help the user.`
	}
}

func (e *ExecutorAgent) buildUserPrompt(step *PlanStep, previousContext string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Execute the following step:\n\n**%s**\n\n", step.Description))

	if len(step.Files) > 0 {
		sb.WriteString(fmt.Sprintf("Relevant files: %s\n\n", strings.Join(step.Files, ", ")))
	}

	if previousContext != "" {
		sb.WriteString(fmt.Sprintf("Context from previous steps:\n%s\n\n", previousContext))
	}

	sb.WriteString("Complete this step using the available tools. Report what you did when finished.")

	return sb.String()
}

func (e *ExecutorAgent) getToolsForStepType(stepType string) []llm.ToolDefinition {
	var toolNames []string

	switch stepType {
	case "read":
		toolNames = []string{
			"read_file", "list_files", "grep",
			"vecgrep_search", "vecgrep_similar",
			"ast_parse", "lsp_query",
		}
	case "code":
		toolNames = []string{
			"read_file", "list_files", "grep",
			"write_file", "edit_file",
			"ast_parse", "lsp_query",
		}
	case "test":
		toolNames = []string{
			"read_file", "test_run", "lint",
		}
	case "verify":
		toolNames = []string{
			"read_file", "lint", "test_run",
			"ast_parse",
		}
	default:
		// Full access
		return e.getAllToolDefs()
	}

	return e.getToolDefsByName(toolNames)
}

func (e *ExecutorAgent) getToolsForIntent(intent Intent) []llm.ToolDefinition {
	switch intent {
	case IntentQuestion:
		return e.getToolDefsByName([]string{
			"read_file", "list_files", "grep",
			"vecgrep_search", "vecgrep_similar",
			"ast_parse", "lsp_query",
		})
	case IntentDebug:
		return e.getToolDefsByName([]string{
			"read_file", "list_files", "grep",
			"test_run", "lint",
			"ast_parse", "lsp_query",
		})
	default:
		return e.getAllToolDefs()
	}
}

func (e *ExecutorAgent) getToolDefsByName(names []string) []llm.ToolDefinition {
	var defs []llm.ToolDefinition
	for _, name := range names {
		if tool, ok := e.registry.Get(name); ok {
			defs = append(defs, llm.ToolDefinition{
				Name:        tool.Name(),
				Description: tool.Description(),
				InputSchema: tool.InputSchema(),
			})
		}
	}
	return defs
}

func (e *ExecutorAgent) getAllToolDefs() []llm.ToolDefinition {
	allTools := e.registry.List()
	defs := make([]llm.ToolDefinition, len(allTools))
	for i, tool := range allTools {
		defs[i] = llm.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		}
	}
	return defs
}

func (e *ExecutorAgent) executeToolCall(ctx context.Context, tc llm.ToolCall) ToolCallResult {
	result := ToolCallResult{
		Tool:  tc.Name,
		Input: tc.Input,
	}

	// Get tool
	tool, ok := e.registry.Get(tc.Name)
	if !ok {
		result.Error = fmt.Errorf("unknown tool: %s", tc.Name)
		result.Output = result.Error.Error()
		return result
	}

	// Check permission
	description := fmt.Sprintf("Execute %s", tc.Name)
	allowed, err := e.permissions.Check(tc.Name, tool.Permission(), description)
	if err != nil {
		result.Error = fmt.Errorf("permission check failed: %w", err)
		result.Output = result.Error.Error()
		return result
	}
	if !allowed {
		result.Error = fmt.Errorf("permission denied for tool: %s", tc.Name)
		result.Output = result.Error.Error()
		return result
	}

	// Execute tool
	output, err := e.registry.Execute(ctx, tc.Name, tc.Input)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: %s", err.Error())
	} else {
		result.Output = output
	}

	return result
}
