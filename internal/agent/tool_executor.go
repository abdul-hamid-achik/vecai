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

// toolResult holds the result of a tool execution.
type toolResult struct {
	Name       string
	Result     string
	Error      bool
	ToolCallID string
}

// ToolExecutor handles tool execution with unified output.
type ToolExecutor struct {
	tools        *tools.Registry
	permissions  *permissions.Policy
	resultCache  *ctxmgr.ToolResultCache
	analysisMode bool
}

// NewToolExecutor creates a new ToolExecutor.
func NewToolExecutor(registry *tools.Registry, perms *permissions.Policy, cache *ctxmgr.ToolResultCache, analysisMode bool) *ToolExecutor {
	return &ToolExecutor{
		tools:        registry,
		permissions:  perms,
		resultCache:  cache,
		analysisMode: analysisMode,
	}
}

// ExecuteToolCalls executes tool calls with unified output and permission checking.
// It works for both CLI and TUI modes via the AgentOutput and AgentInput interfaces.
func (te *ToolExecutor) ExecuteToolCalls(ctx context.Context, calls []llm.ToolCall, output AgentOutput, input AgentInput) []toolResult {
	var results []toolResult

	for _, call := range calls {
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
