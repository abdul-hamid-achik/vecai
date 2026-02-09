package agent

import (
	"context"
	"fmt"
	"strings"

	ctxmgr "github.com/abdul-hamid-achik/vecai/internal/context"
	"github.com/abdul-hamid-achik/vecai/internal/debug"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// maxToolOutput is the maximum size of tool output before truncation (50KB).
const maxToolOutput = 50000

// toolResult holds the result of a tool execution.
type toolResult struct {
	Name       string
	Result     string
	Error      bool
	ToolCallID string
}

// truncateToolOutput truncates tool output if it exceeds maxToolOutput bytes.
func truncateToolOutput(result string) string {
	if len(result) <= maxToolOutput {
		return result
	}
	return result[:maxToolOutput] + "\n\n... (output truncated, showing first 50KB)"
}

// ToolExecutor handles tool execution with unified output.
type ToolExecutor struct {
	tools        *tools.Registry
	permissions  *permissions.Policy
	resultCache  *ctxmgr.ToolResultCache
	parallelExec *parallelExecutor
	analysisMode bool
}

// NewToolExecutor creates a new ToolExecutor.
func NewToolExecutor(registry *tools.Registry, perms *permissions.Policy, cache *ctxmgr.ToolResultCache, analysisMode bool) *ToolExecutor {
	return &ToolExecutor{
		tools:        registry,
		permissions:  perms,
		resultCache:  cache,
		parallelExec: newParallelExecutor(registry, defaultMaxConcurrency),
		analysisMode: analysisMode,
	}
}

// canParallelize returns true if all tool calls are read-only and permission mode
// is auto (no prompts needed), making them safe to execute concurrently.
func (te *ToolExecutor) canParallelize(calls []llm.ToolCall) bool {
	if len(calls) < 2 {
		return false
	}
	if te.permissions.GetMode() != permissions.ModeAuto {
		return false
	}
	for _, call := range calls {
		tool, ok := te.tools.Get(call.Name)
		if !ok {
			return false
		}
		if tool.Permission() != tools.PermissionRead {
			return false
		}
	}
	return true
}

// ExecuteToolCalls executes tool calls with unified output and permission checking.
// It works for both CLI and TUI modes via the AgentOutput and AgentInput interfaces.
// When all calls are read-only and auto-permission is enabled, they run in parallel.
func (te *ToolExecutor) ExecuteToolCalls(ctx context.Context, calls []llm.ToolCall, output AgentOutput, input AgentInput) []toolResult {
	if te.canParallelize(calls) {
		return te.executeParallel(ctx, calls, output)
	}

	var results []toolResult

	for _, call := range calls {
		// Check for context cancellation between tool calls
		select {
		case <-ctx.Done():
			results = append(results, toolResult{
				Name:       call.Name,
				Result:     "Interrupted",
				Error:      true,
				ToolCallID: call.ID,
			})
			return results
		default:
		}

		callID := call.ID
		debug.ToolCall(call.Name, call.Input)

		tool, ok := te.tools.Get(call.Name)
		if !ok {
			debug.ToolResult(call.Name, false, 0)
			results = append(results, toolResult{
				Name:       call.Name,
				Result:     fmt.Sprintf("Unknown tool: %s", call.Name),
				Error:      true,
				ToolCallID: callID,
			})
			continue
		}

		description := formatToolDescription(call.Name, call.Input)

		// Check permission
		allowed, err := te.checkPermission(call.Name, tool.Permission(), description, output, input)
		if err != nil {
			debug.ToolResult(call.Name, false, 0)
			results = append(results, toolResult{
				Name:       call.Name,
				Result:     fmt.Sprintf("Permission error: %s", err),
				Error:      true,
				ToolCallID: callID,
			})
			output.ToolResult(call.Name, "Permission error: "+err.Error(), true)
			continue
		}

		if !allowed {
			debug.ToolResult(call.Name, false, 0)
			results = append(results, toolResult{
				Name:       call.Name,
				Result:     "Permission denied by user",
				Error:      true,
				ToolCallID: callID,
			})
			output.ToolResult(call.Name, "Permission denied", true)
			continue
		}

		// Show tool call after permission granted
		output.ToolCall(call.Name, description)

		// Execute tool
		result, err := tool.Execute(ctx, call.Input)
		if err != nil {
			debug.ToolResult(call.Name, false, 0)
			results = append(results, toolResult{
				Name:       call.Name,
				Result:     fmt.Sprintf("Error: %s", err),
				Error:      true,
				ToolCallID: callID,
			})
			output.ToolResult(call.Name, err.Error(), true)
		} else {
			debug.ToolResult(call.Name, true, len(result))
			// Truncate large tool outputs to prevent memory bloat
			result = truncateToolOutput(result)
			contextResult := result
			if te.resultCache != nil && ctxmgr.ShouldCache(result) {
				summary, _ := te.resultCache.Store(call.Name, call.Input, result)
				contextResult = summary
			}
			results = append(results, toolResult{
				Name:       call.Name,
				Result:     contextResult,
				Error:      false,
				ToolCallID: callID,
			})
			output.ToolResult(call.Name, result, false)
		}
	}

	return results
}

// checkPermission checks permission using the unified output/input interfaces.
func (te *ToolExecutor) checkPermission(toolName string, level tools.PermissionLevel, description string, output AgentOutput, input AgentInput) (bool, error) {
	// Auto mode: always allow
	if te.permissions.GetMode() == permissions.ModeAuto {
		return true, nil
	}

	// Check cache
	if decision, ok := te.permissions.GetCachedDecision(toolName); ok {
		switch decision {
		case permissions.DecisionAlwaysAllow:
			return true, nil
		case permissions.DecisionNeverAllow:
			return false, nil
		}
	}

	// Ask mode: auto-approve reads
	if te.permissions.GetMode() == permissions.ModeAsk && level == tools.PermissionRead {
		return true, nil
	}

	// Prompt user via output/input interfaces
	output.PermissionPrompt(toolName, level, description)

	response, err := input.ReadLine("")
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
		te.permissions.CacheDecision(toolName, permissions.DecisionAlwaysAllow)
		return true, nil
	case "v", "never":
		te.permissions.CacheDecision(toolName, permissions.DecisionNeverAllow)
		return false, nil
	default:
		return false, nil
	}
}

// executeParallel runs all tool calls concurrently via parallelExecutor.
// Called only when canParallelize() returns true (all read-only, auto-permission).
func (te *ToolExecutor) executeParallel(ctx context.Context, calls []llm.ToolCall, output AgentOutput) []toolResult {
	output.Activity(fmt.Sprintf("Running %d tools in parallel...", len(calls)))

	// Show each tool call announcement before starting execution
	for _, call := range calls {
		description := formatToolDescription(call.Name, call.Input)
		debug.ToolCall(call.Name, call.Input)
		output.ToolCall(call.Name, description)
	}

	// Run all tools concurrently
	results := te.parallelExec.executeParallel(ctx, calls,
		func(name string) (bool, error) {
			// All calls are auto-approved (canParallelize verified ModeAuto)
			return true, nil
		},
		nil, // No per-result callback; we show results in order below
	)

	// Show results in order and apply caching
	for i, r := range results {
		if r.Error {
			debug.ToolResult(r.Name, false, 0)
			output.ToolResult(r.Name, r.Result, true)
		} else {
			debug.ToolResult(r.Name, true, len(r.Result))
			// Truncate large tool outputs
			results[i].Result = truncateToolOutput(r.Result)
			displayResult := results[i].Result
			if te.resultCache != nil && ctxmgr.ShouldCache(r.Result) {
				summary, _ := te.resultCache.Store(r.Name, calls[i].Input, r.Result)
				results[i].Result = summary
			}
			output.ToolResult(r.Name, displayResult, false)
		}
		// Set ToolCallID from original call
		results[i].ToolCallID = calls[i].ID
	}

	return results
}

// formatToolDescription creates a human-readable description of a tool call.
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
