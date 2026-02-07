package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/tui"
)

// checkVecgrepStatusTUI checks vecgrep status with TUI output
// Auto-initializes and indexes if not already set up
func (a *Agent) checkVecgrepStatusTUI(adapter *tui.TUIAdapter) {
	ctx := context.Background()

	// Notify user if project instructions were loaded
	if a.projectInstructions != "" {
		// Determine which file was loaded
		instructionFile := "AGENTS.md"
		if _, err := os.Stat("VECAI.md"); err == nil {
			instructionFile = "VECAI.md"
		}
		adapter.Info(fmt.Sprintf("Loaded project instructions from %s", instructionFile))
	}

	// Check for stale files
	staleCount := a.getVecgrepStaleCount(ctx)
	if staleCount > 0 {
		adapter.Warning(fmt.Sprintf("vecgrep index has %d modified files. Run /reindex for best search results.", staleCount))
		return
	}

	// Check if initialized
	tool, _ := a.tools.Get("vecgrep_status")
	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil {
		// Status check failed - try auto-init
		a.autoInitVecgrepTUI(ctx, adapter)
		return
	}

	if strings.Contains(result, "not initialized") {
		// Auto-initialize and index
		a.autoInitVecgrepTUI(ctx, adapter)
	}
	// Don't show "vecgrep index is ready" - it clutters the viewport with no ongoing value
}

// autoInitVecgrepTUI automatically initializes and indexes vecgrep with TUI output
func (a *Agent) autoInitVecgrepTUI(ctx context.Context, adapter *tui.TUIAdapter) {
	adapter.Info("Vecgrep not initialized. Auto-initializing...")

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		adapter.Warning("Failed to get working directory: " + err.Error())
		return
	}

	// Run vecgrep_init
	initTool, ok := a.tools.Get("vecgrep_init")
	if !ok {
		adapter.Warning("vecgrep_init tool not available")
		return
	}

	result, err := initTool.Execute(ctx, map[string]any{
		"path": wd,
	})
	if err != nil {
		adapter.Warning("vecgrep init failed: " + err.Error())
		return
	}
	adapter.Info("vecgrep_init: " + truncateResult(result, 80))

	// Run vecgrep_index
	indexTool, ok := a.tools.Get("vecgrep_index")
	if !ok {
		adapter.Warning("vecgrep_index tool not available")
		return
	}

	adapter.Info("Indexing codebase (this may take a moment)...")
	result, err = indexTool.Execute(ctx, map[string]any{})
	if err != nil {
		adapter.Warning("vecgrep index failed: " + err.Error())
		return
	}
	adapter.Success("Indexing complete: " + truncateResult(result, 80))
}

// checkVecgrepStatus checks if vecgrep is initialized
func (a *Agent) checkVecgrepStatus() {
	ctx := context.Background()

	// Notify user if project instructions were loaded
	if a.projectInstructions != "" {
		instructionFile := "AGENTS.md"
		if _, err := os.Stat("VECAI.md"); err == nil {
			instructionFile = "VECAI.md"
		}
		a.output.Info(fmt.Sprintf("Loaded project instructions from %s", instructionFile))
	}

	tool, _ := a.tools.Get("vecgrep_status")
	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil {
		a.output.Warning("vecgrep status check failed: " + err.Error())
		return
	}

	if strings.Contains(result, "not initialized") {
		a.output.Warning("vecgrep is not initialized. Run 'vecgrep init' for semantic search.")
	}
	// Don't show "vecgrep index is ready" - it clutters the output with no ongoing value
}

// getVecgrepStaleCount returns the number of files that need reindexing
func (a *Agent) getVecgrepStaleCount(ctx context.Context) int {
	cmd := exec.CommandContext(ctx, "vecgrep", "status", "--format", "json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return 0 // Silently fail - don't interrupt startup
	}

	// Parse JSON to get reindex stats
	var status struct {
		ReindexStatus struct {
			NewFiles      int `json:"new_files"`
			ModifiedFiles int `json:"modified_files"`
		} `json:"reindex_status"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		return 0
	}

	return status.ReindexStatus.NewFiles + status.ReindexStatus.ModifiedFiles
}

// reindexVecgrep triggers a vecgrep index update
func (a *Agent) reindexVecgrep(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "vecgrep", "index")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not initialized") {
			return "", fmt.Errorf("vecgrep not initialized. Run 'vecgrep init' first")
		}
		return "", fmt.Errorf("%s", stderr.String())
	}

	// Parse output to get stats
	output := stdout.String()
	if output == "" {
		output = "Index updated successfully"
	}
	return output, nil
}

// truncateResult truncates a result string to maxLen characters
func truncateResult(result string, maxLen int) string {
	// Remove newlines for cleaner display
	result = strings.ReplaceAll(result, "\n", " ")
	result = strings.TrimSpace(result)
	if len(result) > maxLen {
		return result[:maxLen-3] + "..."
	}
	return result
}

// IsTTY checks if stdout is a terminal
func IsTTY() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
