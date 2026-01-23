package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// runGpeek executes gpeek with the given arguments and returns the output
func runGpeek(ctx context.Context, args ...string) ([]byte, error) {
	// Always request JSON output
	args = append(args, "--format", "json")

	cmd := exec.CommandContext(ctx, "gpeek", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if gpeek is not installed
		if strings.Contains(err.Error(), "executable file not found") {
			return nil, fmt.Errorf("gpeek is not installed. Install it with: go install github.com/abdul-hamid-achik/gpeek@latest")
		}
		// Check for not a git repository
		if strings.Contains(stderr.String(), "not a git repository") {
			return nil, fmt.Errorf("not a git repository")
		}
		return nil, fmt.Errorf("gpeek failed: %s", stderr.String())
	}

	return stdout.Bytes(), nil
}

// GpeekStatusTool shows repository status (staged/unstaged/untracked)
type GpeekStatusTool struct{}

func (t *GpeekStatusTool) Name() string {
	return "gpeek_status"
}

func (t *GpeekStatusTool) Description() string {
	return "Get git repository status showing staged, unstaged, and untracked files."
}

func (t *GpeekStatusTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the repository (defaults to current directory).",
			},
		},
	}
}

func (t *GpeekStatusTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GpeekStatusTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	args := []string{"status"}

	if path, ok := input["path"].(string); ok && path != "" {
		args = append(args, "-C", path)
	}

	output, err := runGpeek(ctx, args...)
	if err != nil {
		return "", err
	}

	return formatStatusResponse(output)
}

func formatStatusResponse(jsonOutput []byte) (string, error) {
	var status struct {
		Repository struct {
			Name   string `json:"name"`
			Path   string `json:"path"`
			Branch string `json:"branch"`
		} `json:"repository"`
		Staged []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"staged"`
		Unstaged []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"unstaged"`
		Untracked []string `json:"untracked"`
		Summary   struct {
			StagedCount    int  `json:"staged_count"`
			UnstagedCount  int  `json:"unstaged_count"`
			UntrackedCount int  `json:"untracked_count"`
			IsClean        bool `json:"is_clean"`
			HasConflicts   bool `json:"has_conflicts"`
		} `json:"summary"`
	}

	if err := json.Unmarshal(jsonOutput, &status); err != nil {
		return string(jsonOutput), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Repository: %s (branch: %s)\n\n", status.Repository.Name, status.Repository.Branch))

	if status.Summary.IsClean {
		sb.WriteString("Working tree is clean.\n")
		return sb.String(), nil
	}

	if status.Summary.HasConflicts {
		sb.WriteString("⚠️ **Merge conflicts detected!**\n\n")
	}

	if len(status.Staged) > 0 {
		sb.WriteString("### Staged Changes\n")
		for _, f := range status.Staged {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", f.Status, f.Path))
		}
		sb.WriteString("\n")
	}

	if len(status.Unstaged) > 0 {
		sb.WriteString("### Unstaged Changes\n")
		for _, f := range status.Unstaged {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", f.Status, f.Path))
		}
		sb.WriteString("\n")
	}

	if len(status.Untracked) > 0 {
		sb.WriteString("### Untracked Files\n")
		for _, f := range status.Untracked {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	return sb.String(), nil
}

// GpeekDiffTool shows structured diffs
type GpeekDiffTool struct{}

func (t *GpeekDiffTool) Name() string {
	return "gpeek_diff"
}

func (t *GpeekDiffTool) Description() string {
	return "Get structured diffs showing changes in the repository. Can show staged, unstaged, or commit diffs."
}

func (t *GpeekDiffTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the repository (defaults to current directory).",
			},
			"file": map[string]any{
				"type":        "string",
				"description": "Specific file to diff.",
			},
			"staged": map[string]any{
				"type":        "boolean",
				"description": "Show staged changes instead of unstaged.",
			},
			"commit": map[string]any{
				"type":        "string",
				"description": "Show diff for a specific commit.",
			},
		},
	}
}

func (t *GpeekDiffTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GpeekDiffTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	args := []string{"diff"}

	// File is a positional argument
	if file, ok := input["file"].(string); ok && file != "" {
		args = append(args, file)
	}

	if path, ok := input["path"].(string); ok && path != "" {
		args = append(args, "-C", path)
	}
	if staged, ok := input["staged"].(bool); ok && staged {
		args = append(args, "--staged")
	}
	if commit, ok := input["commit"].(string); ok && commit != "" {
		args = append(args, "--commit", commit)
	}

	output, err := runGpeek(ctx, args...)
	if err != nil {
		return "", err
	}

	return formatDiffResponse(output)
}

func formatDiffResponse(jsonOutput []byte) (string, error) {
	var diff struct {
		File   string `json:"file,omitempty"`
		Commit string `json:"commit,omitempty"`
		Staged bool   `json:"staged"`
		Files  []struct {
			OldName   string `json:"old_name"`
			NewName   string `json:"new_name"`
			IsBinary  bool   `json:"is_binary"`
			IsNew     bool   `json:"is_new"`
			IsDelete  bool   `json:"is_delete"`
			IsRename  bool   `json:"is_rename"`
			Additions int    `json:"additions"`
			Deletions int    `json:"deletions"`
			Hunks     []struct {
				Header string `json:"header"`
				Lines  []struct {
					Type      string `json:"type"`
					Content   string `json:"content"`
					OldNumber int    `json:"old_number,omitempty"`
					NewNumber int    `json:"new_number,omitempty"`
				} `json:"lines"`
			} `json:"hunks"`
		} `json:"files"`
		Stats struct {
			FilesChanged int `json:"files_changed"`
			Additions    int `json:"additions"`
			Deletions    int `json:"deletions"`
		} `json:"stats"`
	}

	if err := json.Unmarshal(jsonOutput, &diff); err != nil {
		return string(jsonOutput), nil
	}

	if len(diff.Files) == 0 {
		return "No changes.", nil
	}

	var sb strings.Builder

	if diff.Commit != "" {
		sb.WriteString(fmt.Sprintf("## Diff for commit: %s\n\n", diff.Commit))
	} else if diff.Staged {
		sb.WriteString("## Staged Changes\n\n")
	} else {
		sb.WriteString("## Unstaged Changes\n\n")
	}

	sb.WriteString(fmt.Sprintf("**Stats:** %d files changed, +%d/-%d lines\n\n",
		diff.Stats.FilesChanged, diff.Stats.Additions, diff.Stats.Deletions))

	for _, f := range diff.Files {
		name := f.NewName
		if f.IsRename {
			sb.WriteString(fmt.Sprintf("### %s → %s (+%d/-%d)\n", f.OldName, f.NewName, f.Additions, f.Deletions))
		} else if f.IsNew {
			sb.WriteString(fmt.Sprintf("### %s (new file, +%d)\n", name, f.Additions))
		} else if f.IsDelete {
			sb.WriteString(fmt.Sprintf("### %s (deleted, -%d)\n", f.OldName, f.Deletions))
		} else {
			sb.WriteString(fmt.Sprintf("### %s (+%d/-%d)\n", name, f.Additions, f.Deletions))
		}

		if f.IsBinary {
			sb.WriteString("Binary file changed\n\n")
			continue
		}

		sb.WriteString("```diff\n")
		for _, hunk := range f.Hunks {
			sb.WriteString(hunk.Header + "\n")
			for _, line := range hunk.Lines {
				switch line.Type {
				case "add":
					sb.WriteString(fmt.Sprintf("+%s\n", line.Content))
				case "remove":
					sb.WriteString(fmt.Sprintf("-%s\n", line.Content))
				default:
					sb.WriteString(fmt.Sprintf(" %s\n", line.Content))
				}
			}
		}
		sb.WriteString("```\n\n")
	}

	return sb.String(), nil
}

// GpeekLogTool shows commit history
type GpeekLogTool struct{}

func (t *GpeekLogTool) Name() string {
	return "gpeek_log"
}

func (t *GpeekLogTool) Description() string {
	return "Get commit history for the repository with optional filtering by author or date."
}

func (t *GpeekLogTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the repository (defaults to current directory).",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of commits to return (default: 10).",
				"default":     10,
			},
			"author": map[string]any{
				"type":        "string",
				"description": "Filter commits by author name or email.",
			},
			"since": map[string]any{
				"type":        "string",
				"description": "Show commits after this date (e.g., '2024-01-01', '1 week ago').",
			},
		},
	}
}

func (t *GpeekLogTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GpeekLogTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	args := []string{"log"}

	if path, ok := input["path"].(string); ok && path != "" {
		args = append(args, "-C", path)
	}
	if limit, ok := input["limit"].(float64); ok {
		args = append(args, "-n", fmt.Sprintf("%d", int(limit)))
	}
	if author, ok := input["author"].(string); ok && author != "" {
		args = append(args, "-a", author)
	}
	if since, ok := input["since"].(string); ok && since != "" {
		args = append(args, "--since", since)
	}

	output, err := runGpeek(ctx, args...)
	if err != nil {
		return "", err
	}

	return formatLogResponse(output)
}

func formatLogResponse(jsonOutput []byte) (string, error) {
	var log struct {
		Commits []struct {
			Hash      string `json:"hash"`
			ShortHash string `json:"short_hash"`
			Message   string `json:"message"`
			Author    string `json:"author"`
			Email     string `json:"email"`
			TimeAgo   string `json:"time_ago"`
			IsMerge   bool   `json:"is_merge"`
		} `json:"commits"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(jsonOutput, &log); err != nil {
		return string(jsonOutput), nil
	}

	if len(log.Commits) == 0 {
		return "No commits found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Commit History (%d commits)\n\n", log.Total))

	for _, c := range log.Commits {
		mergeIndicator := ""
		if c.IsMerge {
			mergeIndicator = " [merge]"
		}
		sb.WriteString(fmt.Sprintf("**%s** - %s%s\n", c.ShortHash, c.Message, mergeIndicator))
		sb.WriteString(fmt.Sprintf("  Author: %s <%s> | %s\n\n", c.Author, c.Email, c.TimeAgo))
	}

	return sb.String(), nil
}

// GpeekSummaryTool provides a complete repository snapshot
type GpeekSummaryTool struct{}

func (t *GpeekSummaryTool) Name() string {
	return "gpeek_summary"
}

func (t *GpeekSummaryTool) Description() string {
	return "Get a complete repository snapshot including status, recent commits, branches, stashes, and tags. Best for getting a quick overview of repository state."
}

func (t *GpeekSummaryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the repository (defaults to current directory).",
			},
			"commits": map[string]any{
				"type":        "integer",
				"description": "Number of recent commits to include (default: 5).",
				"default":     5,
			},
		},
	}
}

func (t *GpeekSummaryTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GpeekSummaryTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	args := []string{"summary"}

	if path, ok := input["path"].(string); ok && path != "" {
		args = append(args, "-C", path)
	}
	if commits, ok := input["commits"].(float64); ok {
		args = append(args, "-n", fmt.Sprintf("%d", int(commits)))
	}

	output, err := runGpeek(ctx, args...)
	if err != nil {
		return "", err
	}

	return formatSummaryResponse(output)
}

func formatSummaryResponse(jsonOutput []byte) (string, error) {
	var summary struct {
		Repository struct {
			Name   string `json:"name"`
			Path   string `json:"path"`
			Branch string `json:"branch"`
		} `json:"repository"`
		Status struct {
			Staged []struct {
				Path   string `json:"path"`
				Status string `json:"status"`
			} `json:"staged"`
			Unstaged []struct {
				Path   string `json:"path"`
				Status string `json:"status"`
			} `json:"unstaged"`
			Untracked      []string `json:"untracked"`
			StagedCount    int      `json:"staged_count"`
			UnstagedCount  int      `json:"unstaged_count"`
			UntrackedCount int      `json:"untracked_count"`
			IsClean        bool     `json:"is_clean"`
			HasConflicts   bool     `json:"has_conflicts"`
		} `json:"status"`
		RecentCommits []struct {
			ShortHash string `json:"short_hash"`
			Message   string `json:"message"`
			Author    string `json:"author"`
			TimeAgo   string `json:"time_ago"`
		} `json:"recent_commits"`
		Branches struct {
			Current string `json:"current"`
			Local   []struct {
				Name      string `json:"name"`
				IsCurrent bool   `json:"is_current"`
			} `json:"local"`
			Count int `json:"count"`
		} `json:"branches"`
		Stashes struct {
			Count   int `json:"count"`
			Entries []struct {
				Index   int    `json:"index"`
				Message string `json:"message"`
			} `json:"entries"`
		} `json:"stashes"`
		Tags struct {
			Count int `json:"count"`
			Tags  []struct {
				Name string `json:"name"`
			} `json:"tags"`
		} `json:"tags"`
	}

	if err := json.Unmarshal(jsonOutput, &summary); err != nil {
		return string(jsonOutput), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Repository Summary: %s\n\n", summary.Repository.Name))
	sb.WriteString(fmt.Sprintf("**Branch:** %s | **Path:** %s\n\n", summary.Repository.Branch, summary.Repository.Path))

	// Status section
	sb.WriteString("## Status\n")
	if summary.Status.IsClean {
		sb.WriteString("Working tree is clean.\n\n")
	} else {
		if summary.Status.HasConflicts {
			sb.WriteString("⚠️ **Merge conflicts detected!**\n")
		}
		sb.WriteString(fmt.Sprintf("- Staged: %d | Unstaged: %d | Untracked: %d\n\n",
			summary.Status.StagedCount, summary.Status.UnstagedCount, summary.Status.UntrackedCount))
	}

	// Recent commits
	if len(summary.RecentCommits) > 0 {
		sb.WriteString("## Recent Commits\n")
		for _, c := range summary.RecentCommits {
			sb.WriteString(fmt.Sprintf("- **%s** %s (%s, %s)\n", c.ShortHash, c.Message, c.Author, c.TimeAgo))
		}
		sb.WriteString("\n")
	}

	// Branches
	sb.WriteString("## Branches\n")
	sb.WriteString(fmt.Sprintf("Current: **%s** | Total: %d\n", summary.Branches.Current, summary.Branches.Count))
	if len(summary.Branches.Local) > 0 {
		names := make([]string, 0, len(summary.Branches.Local))
		for _, b := range summary.Branches.Local {
			if b.IsCurrent {
				names = append(names, fmt.Sprintf("*%s*", b.Name))
			} else {
				names = append(names, b.Name)
			}
		}
		sb.WriteString(fmt.Sprintf("Branches: %s\n", strings.Join(names, ", ")))
	}
	sb.WriteString("\n")

	// Stashes
	if summary.Stashes.Count > 0 {
		sb.WriteString(fmt.Sprintf("## Stashes (%d)\n", summary.Stashes.Count))
		for _, s := range summary.Stashes.Entries {
			sb.WriteString(fmt.Sprintf("- stash@{%d}: %s\n", s.Index, s.Message))
		}
		sb.WriteString("\n")
	}

	// Tags
	if summary.Tags.Count > 0 {
		sb.WriteString(fmt.Sprintf("## Tags (%d)\n", summary.Tags.Count))
		names := make([]string, 0, len(summary.Tags.Tags))
		for _, t := range summary.Tags.Tags {
			names = append(names, t.Name)
		}
		sb.WriteString(strings.Join(names, ", ") + "\n")
	}

	return sb.String(), nil
}

// GpeekBlameTool shows line attribution
type GpeekBlameTool struct{}

func (t *GpeekBlameTool) Name() string {
	return "gpeek_blame"
}

func (t *GpeekBlameTool) Description() string {
	return "Get line-by-line attribution showing who last modified each line of a file."
}

func (t *GpeekBlameTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the repository (defaults to current directory).",
			},
			"file": map[string]any{
				"type":        "string",
				"description": "File to blame (required).",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "Start line number for range.",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "End line number for range.",
			},
		},
		"required": []string{"file"},
	}
}

func (t *GpeekBlameTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GpeekBlameTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	file, ok := input["file"].(string)
	if !ok || file == "" {
		return "", fmt.Errorf("file is required")
	}

	// File is a positional argument
	args := []string{"blame", file}

	if path, ok := input["path"].(string); ok && path != "" {
		args = append(args, "-C", path)
	}
	if startLine, ok := input["start_line"].(float64); ok {
		args = append(args, "--start", fmt.Sprintf("%d", int(startLine)))
	}
	if endLine, ok := input["end_line"].(float64); ok {
		args = append(args, "--end", fmt.Sprintf("%d", int(endLine)))
	}

	output, err := runGpeek(ctx, args...)
	if err != nil {
		return "", err
	}

	return formatBlameResponse(output)
}

func formatBlameResponse(jsonOutput []byte) (string, error) {
	var blame struct {
		File  string `json:"file"`
		Lines []struct {
			LineNum int    `json:"line_num"`
			Hash    string `json:"hash"`
			Author  string `json:"author"`
			TimeAgo string `json:"time_ago"`
			Content string `json:"content"`
		} `json:"lines"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(jsonOutput, &blame); err != nil {
		return string(jsonOutput), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Blame: %s (%d lines)\n\n", blame.File, blame.Total))
	sb.WriteString("```\n")

	for _, l := range blame.Lines {
		sb.WriteString(fmt.Sprintf("%4d | %s | %-15s | %s\n", l.LineNum, l.Hash, l.Author, l.Content))
	}

	sb.WriteString("```\n")
	return sb.String(), nil
}

// GpeekBranchesTool lists branches
type GpeekBranchesTool struct{}

func (t *GpeekBranchesTool) Name() string {
	return "gpeek_branches"
}

func (t *GpeekBranchesTool) Description() string {
	return "List all branches in the repository, showing which is current and optionally including remote branches."
}

func (t *GpeekBranchesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the repository (defaults to current directory).",
			},
			"all": map[string]any{
				"type":        "boolean",
				"description": "Include remote branches.",
			},
		},
	}
}

func (t *GpeekBranchesTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GpeekBranchesTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	args := []string{"branches"}

	if path, ok := input["path"].(string); ok && path != "" {
		args = append(args, "-C", path)
	}
	if all, ok := input["all"].(bool); ok && all {
		args = append(args, "-a")
	}

	output, err := runGpeek(ctx, args...)
	if err != nil {
		return "", err
	}

	return formatBranchesResponse(output)
}

func formatBranchesResponse(jsonOutput []byte) (string, error) {
	var branches struct {
		Current  string `json:"current"`
		Branches []struct {
			Name      string `json:"name"`
			ShortHash string `json:"short_hash"`
			IsRemote  bool   `json:"is_remote"`
			IsCurrent bool   `json:"is_current"`
			Upstream  string `json:"upstream,omitempty"`
		} `json:"branches"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(jsonOutput, &branches); err != nil {
		return string(jsonOutput), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Branches (%d total)\n\n", branches.Total))
	sb.WriteString(fmt.Sprintf("Current branch: **%s**\n\n", branches.Current))

	for _, b := range branches.Branches {
		indicator := ""
		if b.IsCurrent {
			indicator = "* "
		}
		remote := ""
		if b.IsRemote {
			remote = " [remote]"
		}
		upstream := ""
		if b.Upstream != "" {
			upstream = fmt.Sprintf(" → %s", b.Upstream)
		}
		sb.WriteString(fmt.Sprintf("%s%s (%s)%s%s\n", indicator, b.Name, b.ShortHash, remote, upstream))
	}

	return sb.String(), nil
}

// GpeekStashesTool lists stashes
type GpeekStashesTool struct{}

func (t *GpeekStashesTool) Name() string {
	return "gpeek_stashes"
}

func (t *GpeekStashesTool) Description() string {
	return "List all stashes in the repository."
}

func (t *GpeekStashesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the repository (defaults to current directory).",
			},
		},
	}
}

func (t *GpeekStashesTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GpeekStashesTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	args := []string{"stashes"}

	if path, ok := input["path"].(string); ok && path != "" {
		args = append(args, "-C", path)
	}

	output, err := runGpeek(ctx, args...)
	if err != nil {
		return "", err
	}

	return formatStashesResponse(output)
}

func formatStashesResponse(jsonOutput []byte) (string, error) {
	var stashes struct {
		Stashes []struct {
			Index   int    `json:"index"`
			Message string `json:"message"`
			Branch  string `json:"branch,omitempty"`
			Hash    string `json:"hash"`
			TimeAgo string `json:"time_ago"`
		} `json:"stashes"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(jsonOutput, &stashes); err != nil {
		return string(jsonOutput), nil
	}

	if len(stashes.Stashes) == 0 {
		return "No stashes found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Stashes (%d)\n\n", stashes.Total))

	for _, s := range stashes.Stashes {
		branch := ""
		if s.Branch != "" {
			branch = fmt.Sprintf(" on %s", s.Branch)
		}
		sb.WriteString(fmt.Sprintf("**stash@{%d}**%s: %s\n", s.Index, branch, s.Message))
		sb.WriteString(fmt.Sprintf("  %s | %s\n\n", s.Hash[:8], s.TimeAgo))
	}

	return sb.String(), nil
}

// GpeekTagsTool lists tags
type GpeekTagsTool struct{}

func (t *GpeekTagsTool) Name() string {
	return "gpeek_tags"
}

func (t *GpeekTagsTool) Description() string {
	return "List all tags in the repository."
}

func (t *GpeekTagsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the repository (defaults to current directory).",
			},
		},
	}
}

func (t *GpeekTagsTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GpeekTagsTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	args := []string{"tags"}

	if path, ok := input["path"].(string); ok && path != "" {
		args = append(args, "-C", path)
	}

	output, err := runGpeek(ctx, args...)
	if err != nil {
		return "", err
	}

	return formatTagsResponse(output)
}

func formatTagsResponse(jsonOutput []byte) (string, error) {
	var tags struct {
		Tags []struct {
			Name        string `json:"name"`
			ShortHash   string `json:"short_hash"`
			Message     string `json:"message,omitempty"`
			Tagger      string `json:"tagger,omitempty"`
			TimeAgo     string `json:"time_ago,omitempty"`
			IsAnnotated bool   `json:"is_annotated"`
		} `json:"tags"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(jsonOutput, &tags); err != nil {
		return string(jsonOutput), nil
	}

	if len(tags.Tags) == 0 {
		return "No tags found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Tags (%d)\n\n", tags.Total))

	for _, t := range tags.Tags {
		tagType := "lightweight"
		if t.IsAnnotated {
			tagType = "annotated"
		}
		sb.WriteString(fmt.Sprintf("**%s** (%s) - %s\n", t.Name, t.ShortHash, tagType))
		if t.Message != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", t.Message))
		}
		if t.Tagger != "" && t.TimeAgo != "" {
			sb.WriteString(fmt.Sprintf("  by %s, %s\n", t.Tagger, t.TimeAgo))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// GpeekChangesBetweenTool shows changes between refs
type GpeekChangesBetweenTool struct{}

func (t *GpeekChangesBetweenTool) Name() string {
	return "gpeek_changes_between"
}

func (t *GpeekChangesBetweenTool) Description() string {
	return "Show changes between two git references (commits, branches, or tags)."
}

func (t *GpeekChangesBetweenTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the repository (defaults to current directory).",
			},
			"from": map[string]any{
				"type":        "string",
				"description": "Source reference (commit, branch, or tag) - required.",
			},
			"to": map[string]any{
				"type":        "string",
				"description": "Target reference (defaults to HEAD).",
			},
		},
		"required": []string{"from"},
	}
}

func (t *GpeekChangesBetweenTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GpeekChangesBetweenTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	from, ok := input["from"].(string)
	if !ok || from == "" {
		return "", fmt.Errorf("from reference is required")
	}

	args := []string{"summarize-changes", "--from", from}

	if path, ok := input["path"].(string); ok && path != "" {
		args = append(args, "-C", path)
	}
	if to, ok := input["to"].(string); ok && to != "" {
		args = append(args, "--to", to)
	}

	output, err := runGpeek(ctx, args...)
	if err != nil {
		return "", err
	}

	return formatChangesBetweenResponse(output)
}

func formatChangesBetweenResponse(jsonOutput []byte) (string, error) {
	var changes struct {
		From        string            `json:"from"`
		To          string            `json:"to"`
		Title       string            `json:"title"`
		Description string            `json:"description"`
		ChangeTypes map[string]int    `json:"change_types"`
		FilesByArea map[string][]string `json:"files_by_area"`
		Stats       struct {
			CommitCount  int `json:"commit_count"`
			FilesChanged int `json:"files_changed"`
			Additions    int `json:"additions"`
			Deletions    int `json:"deletions"`
		} `json:"stats"`
		Commits []struct {
			Hash    string `json:"hash"`
			Message string `json:"message"`
		} `json:"commits"`
	}

	if err := json.Unmarshal(jsonOutput, &changes); err != nil {
		return string(jsonOutput), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Changes: %s → %s\n\n", changes.From, changes.To))

	if changes.Title != "" {
		sb.WriteString(fmt.Sprintf("**%s**\n\n", changes.Title))
	}

	sb.WriteString(fmt.Sprintf("**Stats:** %d commits, %d files changed, +%d/-%d lines\n\n",
		changes.Stats.CommitCount, changes.Stats.FilesChanged,
		changes.Stats.Additions, changes.Stats.Deletions))

	if len(changes.ChangeTypes) > 0 {
		sb.WriteString("### Change Types\n")
		for changeType, count := range changes.ChangeTypes {
			sb.WriteString(fmt.Sprintf("- %s: %d\n", changeType, count))
		}
		sb.WriteString("\n")
	}

	if len(changes.Commits) > 0 {
		sb.WriteString("### Commits\n")
		for _, c := range changes.Commits {
			sb.WriteString(fmt.Sprintf("- **%s** %s\n", c.Hash, c.Message))
		}
		sb.WriteString("\n")
	}

	if len(changes.FilesByArea) > 0 {
		sb.WriteString("### Files by Area\n")
		for area, files := range changes.FilesByArea {
			sb.WriteString(fmt.Sprintf("**%s:**\n", area))
			for _, f := range files {
				sb.WriteString(fmt.Sprintf("- %s\n", f))
			}
		}
	}

	return sb.String(), nil
}

// GpeekConflictCheckTool predicts merge conflicts
type GpeekConflictCheckTool struct{}

func (t *GpeekConflictCheckTool) Name() string {
	return "gpeek_conflict_check"
}

func (t *GpeekConflictCheckTool) Description() string {
	return "Predict potential merge conflicts between branches without actually merging."
}

func (t *GpeekConflictCheckTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the repository (defaults to current directory).",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Branch to check for conflicts (required).",
			},
			"into": map[string]any{
				"type":        "string",
				"description": "Target branch to merge into (defaults to current branch).",
			},
		},
		"required": []string{"branch"},
	}
}

func (t *GpeekConflictCheckTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GpeekConflictCheckTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	branch, ok := input["branch"].(string)
	if !ok || branch == "" {
		return "", fmt.Errorf("branch is required")
	}

	args := []string{"check-conflicts", "-b", branch}

	if path, ok := input["path"].(string); ok && path != "" {
		args = append(args, "-C", path)
	}
	if into, ok := input["into"].(string); ok && into != "" {
		args = append(args, "--into", into)
	}

	output, err := runGpeek(ctx, args...)
	if err != nil {
		return "", err
	}

	return formatConflictCheckResponse(output)
}

func formatConflictCheckResponse(jsonOutput []byte) (string, error) {
	var check struct {
		Branch         string   `json:"branch"`
		Into           string   `json:"into"`
		WouldConflict  bool     `json:"would_conflict"`
		SafeToMerge    bool     `json:"safe_to_merge"`
		Recommendation string   `json:"recommendation"`
		ConflictFiles  []string `json:"conflict_files,omitempty"`
	}

	if err := json.Unmarshal(jsonOutput, &check); err != nil {
		return string(jsonOutput), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Conflict Check: %s → %s\n\n", check.Branch, check.Into))

	if check.SafeToMerge {
		sb.WriteString("✅ **Safe to merge!**\n\n")
	} else if check.WouldConflict {
		sb.WriteString("⚠️ **Conflicts detected!**\n\n")
	}

	if check.Recommendation != "" {
		sb.WriteString(fmt.Sprintf("**Recommendation:** %s\n", check.Recommendation))
	}

	if len(check.ConflictFiles) > 0 {
		sb.WriteString("\n### Conflicting Files\n")
		for _, f := range check.ConflictFiles {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	return sb.String(), nil
}
