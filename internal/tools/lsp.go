package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LSPTool provides LSP functionality by shelling out to gopls
type LSPTool struct{}

func (t *LSPTool) Name() string {
	return "lsp_query"
}

func (t *LSPTool) Description() string {
	return "Query Go language server (gopls) for code intelligence: go to definition, find references, find implementations, and hover info. Requires gopls to be installed (go install golang.org/x/tools/gopls@latest)."
}

func (t *LSPTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "The LSP action to perform: 'definition', 'references', 'implementation', 'hover'.",
				"enum":        []string{"definition", "references", "implementation", "hover"},
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the Go file.",
			},
			"line": map[string]any{
				"type":        "integer",
				"description": "Line number (1-indexed).",
			},
			"column": map[string]any{
				"type":        "integer",
				"description": "Column number (1-indexed, byte offset).",
			},
		},
		"required": []string{"action", "path", "line", "column"},
	}
}

func (t *LSPTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *LSPTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	action, ok := input["action"].(string)
	if !ok || action == "" {
		return "", fmt.Errorf("action is required")
	}

	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	line, ok := input["line"].(float64)
	if !ok || line < 1 {
		return "", fmt.Errorf("line is required and must be positive")
	}

	column, ok := input["column"].(float64)
	if !ok || column < 1 {
		return "", fmt.Errorf("column is required and must be positive")
	}

	// Resolve path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check if gopls is available
	if _, err := exec.LookPath("gopls"); err != nil {
		return "", fmt.Errorf("gopls not found. Install with: go install golang.org/x/tools/gopls@latest")
	}

	// Build position string for gopls
	position := fmt.Sprintf("%s:%d:%d", absPath, int(line), int(column))

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	switch action {
	case "definition":
		return t.definition(ctx, position)
	case "references":
		return t.references(ctx, position)
	case "implementation":
		return t.implementation(ctx, position)
	case "hover":
		return t.hover(ctx, position)
	default:
		return "", fmt.Errorf("unknown action: %s (valid: definition, references, implementation, hover)", action)
	}
}

func (t *LSPTool) definition(ctx context.Context, position string) (string, error) {
	cmd := exec.CommandContext(ctx, "gopls", "definition", position)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("gopls definition failed: %s", strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("gopls definition failed: %w", err)
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "No definition found.", nil
	}

	return fmt.Sprintf("## Definition\n%s", result), nil
}

func (t *LSPTool) references(ctx context.Context, position string) (string, error) {
	cmd := exec.CommandContext(ctx, "gopls", "references", position)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("gopls references failed: %s", strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("gopls references failed: %w", err)
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "No references found.", nil
	}

	// Format output
	lines := strings.Split(result, "\n")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## References (%d found)\n", len(lines)))
	for i, line := range lines {
		if i >= 50 { // Limit to 50 references
			sb.WriteString(fmt.Sprintf("\n... and %d more references", len(lines)-50))
			break
		}
		sb.WriteString(fmt.Sprintf("  %s\n", line))
	}

	return sb.String(), nil
}

func (t *LSPTool) implementation(ctx context.Context, position string) (string, error) {
	cmd := exec.CommandContext(ctx, "gopls", "implementation", position)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("gopls implementation failed: %s", strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("gopls implementation failed: %w", err)
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "No implementations found.", nil
	}

	// Format output
	lines := strings.Split(result, "\n")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Implementations (%d found)\n", len(lines)))
	for _, line := range lines {
		sb.WriteString(fmt.Sprintf("  %s\n", line))
	}

	return sb.String(), nil
}

func (t *LSPTool) hover(ctx context.Context, position string) (string, error) {
	// gopls hover outputs JSON when used programmatically
	cmd := exec.CommandContext(ctx, "gopls", "hover", "-json", position)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Try non-JSON mode as fallback
		cmd2 := exec.CommandContext(ctx, "gopls", "hover", position)
		var stdout2 bytes.Buffer
		cmd2.Stdout = &stdout2
		if err2 := cmd2.Run(); err2 == nil && stdout2.Len() > 0 {
			return fmt.Sprintf("## Hover Info\n%s", strings.TrimSpace(stdout2.String())), nil
		}

		if stderr.Len() > 0 {
			return "", fmt.Errorf("gopls hover failed: %s", strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("gopls hover failed: %w", err)
	}

	// Try to parse JSON
	var hoverResult struct {
		Synopsis string `json:"synopsis"`
		FullDoc  string `json:"fullDocumentation"`
		Sig      string `json:"signature"`
		LinkPath string `json:"linkPath"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &hoverResult); err != nil {
		// Fallback to raw output
		result := strings.TrimSpace(stdout.String())
		if result == "" {
			return "No hover information available.", nil
		}
		return fmt.Sprintf("## Hover Info\n%s", result), nil
	}

	var sb strings.Builder
	sb.WriteString("## Hover Info\n")

	if hoverResult.Sig != "" {
		sb.WriteString(fmt.Sprintf("**Signature:**\n```go\n%s\n```\n\n", hoverResult.Sig))
	}

	if hoverResult.Synopsis != "" {
		sb.WriteString(fmt.Sprintf("**Synopsis:** %s\n\n", hoverResult.Synopsis))
	}

	if hoverResult.FullDoc != "" {
		sb.WriteString(fmt.Sprintf("**Documentation:**\n%s\n", hoverResult.FullDoc))
	}

	if hoverResult.LinkPath != "" {
		sb.WriteString(fmt.Sprintf("\n**Package:** %s\n", hoverResult.LinkPath))
	}

	return sb.String(), nil
}
