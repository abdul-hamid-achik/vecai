package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BashTool executes bash commands
type BashTool struct {
	Sandbox    Sandbox // OS-level sandbox; nil means no sandboxing
	ProjectDir string  // Project root for sandbox filesystem restrictions
}

func (t *BashTool) Name() string {
	return "bash"
}

func (t *BashTool) Description() string {
	return "Execute a bash command. Use for running builds, tests, git operations, and other shell commands. Output is captured and returned."
}

func (t *BashTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The bash command to execute.",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds (default: 60).",
				"default":     60,
			},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) Permission() PermissionLevel {
	return PermissionExecute
}

// maxBashTimeout is the maximum allowed timeout for bash commands (5 minutes)
const maxBashTimeout = 300

func (t *BashTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	command, ok := input["command"].(string)
	if !ok || command == "" {
		return "", fmt.Errorf("command is required")
	}

	// Multi-layer command safety check (blocklist + obfuscation + evasion)
	if err := CheckCommandSafety(command); err != nil {
		return "", err
	}

	timeout := 60
	if tv, ok := input["timeout"].(float64); ok && tv > 0 {
		timeout = int(tv)
	}
	// Cap timeout to prevent indefinitely long-running commands
	if timeout > maxBashTimeout {
		timeout = maxBashTimeout
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Determine command execution: sandboxed or direct
	exe := "bash"
	args := []string{"-c", command}
	if t.Sandbox != nil && t.Sandbox.Available() {
		projectDir := t.ProjectDir
		if projectDir == "" {
			projectDir = "."
		}
		var err error
		exe, args, err = t.Sandbox.Wrap(command, projectDir)
		if err != nil {
			return "", fmt.Errorf("sandbox wrap failed: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = SanitizedEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Build output
	var result strings.Builder

	if stdout.Len() > 0 {
		result.WriteString(stdout.String())
	}

	if stderr.Len() > 0 {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString("STDERR:\n")
		result.WriteString(stderr.String())
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after %d seconds", timeout)
		}

		// Include exit code in output, not as error
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(fmt.Sprintf("Exit code: %v", err))
	}

	output := result.String()
	if output == "" {
		output = "(no output)"
	}

	// Truncate very long output
	const maxOutput = 50000
	if len(output) > maxOutput {
		output = output[:maxOutput] + "\n... (output truncated)"
	}

	return output, nil
}

// GrepTool searches for patterns in files
type GrepTool struct{}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Description() string {
	return "Search for a pattern in files using ripgrep (rg). Fast, respects .gitignore."
}

func (t *GrepTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The regex pattern to search for.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Path to search in (default: current directory).",
				"default":     ".",
			},
			"file_type": map[string]any{
				"type":        "string",
				"description": "Filter by file type (e.g., 'go', 'ts', 'py').",
			},
			"context": map[string]any{
				"type":        "integer",
				"description": "Number of context lines around matches.",
				"default":     0,
			},
			"case_sensitive": map[string]any{
				"type":        "boolean",
				"description": "Case sensitive search (default: false).",
				"default":     false,
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GrepTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	pattern, ok := input["pattern"].(string)
	if !ok || pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	path := "."
	if p, ok := input["path"].(string); ok && p != "" {
		path = p
	}

	args := []string{"--color=never", "-n"} // No color, show line numbers

	// Case sensitivity
	if caseSensitive, ok := input["case_sensitive"].(bool); !ok || !caseSensitive {
		args = append(args, "-i")
	}

	// File type
	if fileType, ok := input["file_type"].(string); ok && fileType != "" {
		args = append(args, "-t", fileType)
	}

	// Context lines
	if ctx, ok := input["context"].(float64); ok && ctx > 0 {
		args = append(args, "-C", fmt.Sprintf("%d", int(ctx)))
	}

	args = append(args, pattern, path)

	cmd := exec.CommandContext(ctx, "rg", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// rg returns exit code 1 for no matches, which is not an error
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return "No matches found.", nil
			}
		}
		// Check if rg is not installed, fall back to grep
		if strings.Contains(stderr.String(), "not found") || strings.Contains(err.Error(), "executable file not found") {
			return t.fallbackGrep(ctx, input)
		}
		return "", fmt.Errorf("search failed: %s", stderr.String())
	}

	output := stdout.String()
	if output == "" {
		return "No matches found.", nil
	}

	// Truncate very long output
	const maxOutput = 50000
	if len(output) > maxOutput {
		output = output[:maxOutput] + "\n... (output truncated)"
	}

	return output, nil
}

func (t *GrepTool) fallbackGrep(ctx context.Context, input map[string]any) (string, error) {
	pattern := input["pattern"].(string)
	path := "."
	if p, ok := input["path"].(string); ok && p != "" {
		path = p
	}

	args := []string{"-r", "-n"}

	if caseSensitive, ok := input["case_sensitive"].(bool); !ok || !caseSensitive {
		args = append(args, "-i")
	}

	args = append(args, pattern, path)

	cmd := exec.CommandContext(ctx, "grep", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	_ = cmd.Run() // Ignore error - grep returns 1 for no matches

	output := stdout.String()
	if output == "" {
		return "No matches found.", nil
	}

	return output, nil
}
