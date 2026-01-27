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

// LinterTool runs golangci-lint and parses the results
type LinterTool struct{}

func (t *LinterTool) Name() string {
	return "lint"
}

func (t *LinterTool) Description() string {
	return "Run golangci-lint on Go code and return issues found. Requires golangci-lint to be installed. Can lint a specific file, directory, or the entire project."
}

func (t *LinterTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to lint: file, directory, or '.' for entire project. Default: '.'",
				"default":     ".",
			},
			"fix": map[string]any{
				"type":        "boolean",
				"description": "Attempt to auto-fix issues where possible. Default: false",
				"default":     false,
			},
			"fast": map[string]any{
				"type":        "boolean",
				"description": "Run only fast linters. Default: false",
				"default":     false,
			},
		},
	}
}

func (t *LinterTool) Permission() PermissionLevel {
	// Read by default, but Write if fix=true
	return PermissionRead
}

// LintIssue represents a single lint issue from golangci-lint
type LintIssue struct {
	FromLinter  string `json:"FromLinter"`
	Text        string `json:"Text"`
	Severity    string `json:"Severity"`
	SourceLines []string `json:"SourceLines"`
	Pos         struct {
		Filename string `json:"Filename"`
		Line     int    `json:"Line"`
		Column   int    `json:"Column"`
	} `json:"Pos"`
}

// LintReport represents the full golangci-lint JSON output
type LintReport struct {
	Issues []LintIssue `json:"Issues"`
	Report struct {
		Linters []struct {
			Name string `json:"Name"`
		} `json:"Linters"`
	} `json:"Report"`
}

func (t *LinterTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	// Get path
	path := "."
	if p, ok := input["path"].(string); ok && p != "" {
		path = p
	}

	// Check for fix mode
	fix := false
	if f, ok := input["fix"].(bool); ok {
		fix = f
	}

	// Check for fast mode
	fast := false
	if f, ok := input["fast"].(bool); ok {
		fast = f
	}

	// Check if golangci-lint is available
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		return "", fmt.Errorf("golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
	}

	// Resolve path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Build command
	args := []string{"run", "--out-format", "json"}
	if fix {
		args = append(args, "--fix")
	}
	if fast {
		args = append(args, "--fast")
	}

	// If path is a specific file, lint that file's directory with --path-prefix
	if strings.HasSuffix(absPath, ".go") {
		dir := filepath.Dir(absPath)
		args = append(args, dir)
	} else {
		args = append(args, absPath+"/...")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "golangci-lint", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// golangci-lint exits with 1 if issues found, which is not an error for us
	_ = cmd.Run()

	// Check for actual errors (not lint issues)
	if stderr.Len() > 0 {
		stderrStr := stderr.String()
		// Ignore some common non-error messages
		if !strings.Contains(stderrStr, "level=warning") && !strings.Contains(stderrStr, "level=info") {
			if strings.Contains(stderrStr, "no go files") {
				return "No Go files found in the specified path.", nil
			}
			// Only return error if it looks like a real error
			if strings.Contains(stderrStr, "level=error") || strings.Contains(stderrStr, "ERRO") {
				return "", fmt.Errorf("golangci-lint error: %s", strings.TrimSpace(stderrStr))
			}
		}
	}

	// Parse JSON output
	if stdout.Len() == 0 {
		return "No lint issues found.", nil
	}

	var report LintReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		// Fallback: maybe it's not JSON (older version?)
		result := strings.TrimSpace(stdout.String())
		if result == "" {
			return "No lint issues found.", nil
		}
		return fmt.Sprintf("## Lint Results\n%s", result), nil
	}

	if len(report.Issues) == 0 {
		return "No lint issues found.", nil
	}

	// Format output
	return t.formatReport(&report, absPath), nil
}

func (t *LinterTool) formatReport(report *LintReport, basePath string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Lint Report: %d issues found\n\n", len(report.Issues)))

	// Group by file
	byFile := make(map[string][]LintIssue)
	for _, issue := range report.Issues {
		filename := issue.Pos.Filename
		// Make path relative if possible
		if rel, err := filepath.Rel(basePath, filename); err == nil && !strings.HasPrefix(rel, "..") {
			filename = rel
		}
		byFile[filename] = append(byFile[filename], issue)
	}

	// Group by severity
	var errors, warnings, info []string

	for filename, issues := range byFile {
		for _, issue := range issues {
			line := fmt.Sprintf("  %s:%d:%d [%s] %s",
				filename,
				issue.Pos.Line,
				issue.Pos.Column,
				issue.FromLinter,
				issue.Text,
			)

			switch strings.ToLower(issue.Severity) {
			case "error":
				errors = append(errors, line)
			case "warning":
				warnings = append(warnings, line)
			default:
				info = append(info, line)
			}
		}
	}

	// Output by severity
	if len(errors) > 0 {
		sb.WriteString("### Errors\n")
		for _, e := range errors {
			sb.WriteString(e + "\n")
		}
		sb.WriteString("\n")
	}

	if len(warnings) > 0 {
		sb.WriteString("### Warnings\n")
		for _, w := range warnings {
			sb.WriteString(w + "\n")
		}
		sb.WriteString("\n")
	}

	if len(info) > 0 {
		sb.WriteString("### Info\n")
		for _, i := range info {
			sb.WriteString(i + "\n")
		}
		sb.WriteString("\n")
	}

	// Show sample source lines if available
	for _, issue := range report.Issues {
		if len(issue.SourceLines) > 0 {
			sb.WriteString(fmt.Sprintf("Example from %s:%d:\n", issue.Pos.Filename, issue.Pos.Line))
			for _, srcLine := range issue.SourceLines {
				sb.WriteString(fmt.Sprintf("  %s\n", srcLine))
			}
			sb.WriteString("\n")
			break // Just show one example
		}
	}

	return sb.String()
}
