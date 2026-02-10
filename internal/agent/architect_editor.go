package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
)

// writeToolNames lists tools that modify files.
var writeToolNames = map[string]bool{
	"write_file": true,
	"edit_file":  true,
}

// hasWriteToolCalls returns true if any tool call is a write operation.
func hasWriteToolCalls(calls []llm.ToolCall) bool {
	for _, tc := range calls {
		if writeToolNames[tc.Name] {
			return true
		}
	}
	return false
}

// architectPhase runs the genius model with read-only tools to produce a change plan.
// Returns a natural language description of what changes are needed.
func (a *Agent) architectPhase(ctx context.Context, output AgentOutput) (string, error) {
	output.Activity("Architect phase: analyzing with genius model...")

	// Switch to genius tier for the architect
	originalModel := a.llm.GetModel()
	a.llm.SetTier(config.TierGenius)
	a.syncContextWindow()
	defer func() {
		a.llm.SetModel(originalModel)
		a.syncContextWindow()
	}()

	architectPrompt := `You are an architect agent. Your job is to ANALYZE what changes are needed and describe them precisely.

DO NOT call write_file or edit_file. Instead:
1. Use read-only tools (read_file, vecgrep_search, grep, list_files) to understand the codebase
2. Describe EXACTLY what changes should be made:
   - Which files to modify
   - What to change in each file (old code → new code)
   - Any new files to create and their contents
3. Be specific about line numbers, function names, and exact code changes.

Output your change plan as structured text that an editor agent can follow.`

	// Use read-only tool definitions
	readOnlyDefs := a.getReadOnlyToolDefs()

	// Use temperature 0.4 for planning
	planCtx := llm.WithTemperature(ctx, 0.4)

	messages := a.contextMgr.GetMessagesWithMasking()

	// Run a mini agent loop for the architect (max 5 iterations for reading)
	var planText strings.Builder
	for i := 0; i < 5; i++ {
		resp, err := a.llm.Chat(planCtx, messages, readOnlyDefs, architectPrompt)
		if err != nil {
			return "", fmt.Errorf("architect phase failed: %w", err)
		}

		if resp.Content != "" {
			planText.WriteString(resp.Content)
		}

		if len(resp.ToolCalls) == 0 {
			break
		}

		// Execute read-only tool calls
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})
		for _, tc := range resp.ToolCalls {
			result, execErr := a.tools.Execute(ctx, tc.Name, tc.Input)
			if execErr != nil {
				result = "Error: " + execErr.Error()
			}
			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	plan := planText.String()
	if plan == "" {
		return "", fmt.Errorf("architect produced no output")
	}

	output.Info("Architect phase complete — handing off to editor")
	return plan, nil
}

// editorPhase runs the smart model with the architect's plan to execute changes.
func (a *Agent) editorPhase(ctx context.Context, plan string, output AgentOutput, input AgentInput) error {
	output.Activity("Editor phase: executing changes with smart model...")

	// Switch to smart tier for the editor
	originalModel := a.llm.GetModel()
	a.llm.SetTier(config.TierSmart)
	a.syncContextWindow()
	defer func() {
		a.llm.SetModel(originalModel)
		a.syncContextWindow()
	}()

	// Inject the architect's plan into context
	a.contextMgr.AddMessage(llm.Message{
		Role:    "user",
		Content: "[Architect's Change Plan]\n\n" + plan + "\n\nExecute these changes now using the available tools. Follow the plan precisely.",
	})

	// Use low temperature for precise edits
	editCtx := llm.WithTemperature(ctx, 0.1)

	// Run the normal agent loop which handles tool execution
	toolDefs := a.getToolDefinitions()
	maxIter := 10

	for i := 0; i < maxIter; i++ {
		stream := a.llm.ChatStream(editCtx, a.contextMgr.GetMessagesWithMasking(), toolDefs, a.getSystemPrompt())

		var textContent strings.Builder
		var toolCalls []llm.ToolCall

		for chunk := range stream {
			if err := a.processStreamChunk(chunk, output, &textContent, &toolCalls); err != nil {
				return err
			}
		}

		if textContent.Len() > 0 || len(toolCalls) > 0 {
			a.contextMgr.AddMessage(llm.Message{
				Role:      "assistant",
				Content:   textContent.String(),
				ToolCalls: toolCalls,
			})
		}

		if len(toolCalls) == 0 {
			return nil
		}

		// Execute tool calls
		toolResults := a.toolExecutor.ExecuteToolCalls(ctx, toolCalls, output, input)
		for _, result := range toolResults {
			a.contextMgr.AddMessage(llm.Message{
				Role:       "tool",
				Content:    result.Result,
				ToolCallID: result.ToolCallID,
			})
		}
	}

	return nil
}

// getReadOnlyToolDefs returns tool definitions for read-only operations.
func (a *Agent) getReadOnlyToolDefs() []llm.ToolDefinition {
	allDefs := a.tools.GetDefinitions()
	var readOnly []llm.ToolDefinition
	for _, d := range allDefs {
		if readOnlyToolNames[d.Name] {
			readOnly = append(readOnly, llm.ToolDefinition{
				Name:        d.Name,
				Description: d.Description,
				InputSchema: d.InputSchema,
			})
		}
	}
	return readOnly
}
