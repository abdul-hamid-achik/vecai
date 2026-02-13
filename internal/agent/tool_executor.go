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
	tools         *tools.Registry
	permissions   *permissions.Policy
	resultCache   *ctxmgr.ToolResultCache
	parallelExec  *parallelExecutor
	analysisMode  bool
	checkpointMgr *CheckpointManager // Optional: records file state before writes
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

		// If the LLM produced unparseable arguments, return error feedback
		// so the model can learn the correct format and retry
		if call.ParseError != "" {
			debug.ToolResult(call.Name, false, 0)
			errMsg := fmt.Sprintf("Tool call '%s' failed: could not parse arguments (%s). "+
				"Please provide arguments as a valid JSON object. Example: {\"path\": \"file.go\"}", call.Name, call.ParseError)
			results = append(results, toolResult{
				Name:       call.Name,
				Result:     errMsg,
				Error:      true,
				ToolCallID: callID,
			})
			output.ToolResult(call.Name, errMsg, true)
			continue
		}

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

		// Save file state before write operations for /rewind
		if te.checkpointMgr != nil && (call.Name == "write_file" || call.Name == "edit_file") {
			if path, ok := call.Input["path"].(string); ok {
				te.checkpointMgr.SaveFileState(path)
			}
		}

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
	// Helper to extract a string field with truncation
	getStr := func(key string, maxLen int) string {
		if v, ok := input[key].(string); ok {
			if maxLen > 0 && len(v) > maxLen {
				return v[:maxLen] + "..."
			}
			return v
		}
		return ""
	}

	switch name {
	// File operations
	case "read_file":
		if p := getStr("path", 0); p != "" {
			return fmt.Sprintf("Read %s", p)
		}
	case "write_file":
		if p := getStr("path", 0); p != "" {
			return fmt.Sprintf("Write to %s", p)
		}
	case "edit_file":
		if p := getStr("path", 0); p != "" {
			return fmt.Sprintf("Edit %s", p)
		}
	case "list_files":
		path := "."
		if p := getStr("path", 0); p != "" {
			path = p
		}
		return fmt.Sprintf("List files in %s", path)

	// Search
	case "grep":
		if p := getStr("pattern", 60); p != "" {
			return fmt.Sprintf("Grep: %s", p)
		}

	// Execution
	case "bash":
		if cmd := getStr("command", 50); cmd != "" {
			return fmt.Sprintf("Run: %s", cmd)
		}

	// Analysis tools
	case "ast_parse":
		if p := getStr("path", 0); p != "" {
			return fmt.Sprintf("Parse AST: %s", p)
		}
	case "lsp_query":
		if q := getStr("query", 50); q != "" {
			return fmt.Sprintf("LSP: %s", q)
		}
	case "lint":
		if p := getStr("path", 0); p != "" {
			return fmt.Sprintf("Lint %s", p)
		}
	case "test_run":
		if p := getStr("path", 0); p != "" {
			return fmt.Sprintf("Test %s", p)
		}
		return "Run tests"

	// Vecgrep tools
	case "vecgrep_search":
		if q := getStr("query", 60); q != "" {
			return fmt.Sprintf("Search: %s", q)
		}
	case "vecgrep_similar":
		if f := getStr("file", 0); f != "" {
			return fmt.Sprintf("Similar to %s", f)
		}
	case "vecgrep_status":
		return "Check index status"
	case "vecgrep_overview":
		return "Codebase overview"
	case "vecgrep_related_files":
		if f := getStr("file", 0); f != "" {
			return fmt.Sprintf("Related to %s", f)
		}
	case "vecgrep_index":
		return "Index codebase"
	case "vecgrep_init":
		return "Initialize vecgrep"
	case "vecgrep_reset":
		return "Reset index"
	case "vecgrep_clean":
		return "Clean index"
	case "vecgrep_delete":
		if f := getStr("file_path", 0); f != "" {
			return fmt.Sprintf("Remove from index: %s", f)
		}
	case "vecgrep_batch_search":
		return "Batch search"

	// Gpeek tools
	case "gpeek_peek":
		if s := getStr("symbol", 50); s != "" {
			return fmt.Sprintf("Peek: %s", s)
		}
	case "gpeek_search":
		if q := getStr("query", 50); q != "" {
			return fmt.Sprintf("Peek search: %s", q)
		}
	case "gpeek_deps":
		if p := getStr("package", 0); p != "" {
			return fmt.Sprintf("Deps: %s", p)
		}

	// Noted tools
	case "noted_remember":
		return "Remember note"
	case "noted_recall":
		if q := getStr("query", 50); q != "" {
			return fmt.Sprintf("Recall: %s", q)
		}
	case "noted_forget":
		return "Forget note"
	case "noted_stats":
		return "Memory stats"

	// Web
	case "web_search":
		if q := getStr("query", 60); q != "" {
			return fmt.Sprintf("Web: %s", q)
		}
	}
	return ""
}
