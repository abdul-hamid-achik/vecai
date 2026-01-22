package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// VecgrepSearchTool performs semantic search using vecgrep
type VecgrepSearchTool struct{}

func (t *VecgrepSearchTool) Name() string {
	return "vecgrep_search"
}

func (t *VecgrepSearchTool) Description() string {
	return "Perform semantic search across the codebase using vector embeddings. Returns code chunks that are semantically similar to the query."
}

func (t *VecgrepSearchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query. Can be natural language description of what you're looking for.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default: 10).",
				"default":     10,
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Filter results by programming language (e.g., 'go', 'python', 'javascript').",
			},
			"file_pattern": map[string]any{
				"type":        "string",
				"description": "Filter results by file path pattern (glob).",
			},
		},
		"required": []string{"query"},
	}
}

func (t *VecgrepSearchTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *VecgrepSearchTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	query, ok := input["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query is required")
	}

	args := []string{"search", query}

	if limit, ok := input["limit"].(float64); ok {
		args = append(args, "--limit", fmt.Sprintf("%d", int(limit)))
	}

	if lang, ok := input["language"].(string); ok && lang != "" {
		args = append(args, "--language", lang)
	}

	if pattern, ok := input["file_pattern"].(string); ok && pattern != "" {
		args = append(args, "--file-pattern", pattern)
	}

	// Add JSON output format
	args = append(args, "--format", "json")

	cmd := exec.CommandContext(ctx, "vecgrep", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if vecgrep is not initialized
		if strings.Contains(stderr.String(), "not initialized") || strings.Contains(stderr.String(), "no .vecgrep") {
			return "vecgrep is not initialized for this project. Please run 'vecgrep init' to set up semantic search.", nil
		}
		return "", fmt.Errorf("vecgrep search failed: %s", stderr.String())
	}

	// Parse JSON output and format for LLM
	return formatSearchResults(stdout.String())
}

func formatSearchResults(jsonOutput string) (string, error) {
	if strings.TrimSpace(jsonOutput) == "" {
		return "No results found.", nil
	}

	var results []struct {
		File       string  `json:"file"`
		StartLine  int     `json:"start_line"`
		EndLine    int     `json:"end_line"`
		Language   string  `json:"language"`
		ChunkType  string  `json:"chunk_type"`
		Content    string  `json:"content"`
		Similarity float64 `json:"similarity"`
	}

	if err := json.Unmarshal([]byte(jsonOutput), &results); err != nil {
		// If JSON parsing fails, return raw output
		return jsonOutput, nil
	}

	if len(results) == 0 {
		return "No results found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d results:\n\n", len(results)))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("### Result %d (%.2f%% match)\n", i+1, r.Similarity*100))
		sb.WriteString(fmt.Sprintf("**File:** %s:%d-%d\n", r.File, r.StartLine, r.EndLine))
		sb.WriteString(fmt.Sprintf("**Language:** %s | **Type:** %s\n", r.Language, r.ChunkType))
		sb.WriteString("```" + r.Language + "\n")
		sb.WriteString(r.Content)
		if !strings.HasSuffix(r.Content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("```\n\n")
	}

	return sb.String(), nil
}

// VecgrepStatusTool checks vecgrep index status
type VecgrepStatusTool struct{}

func (t *VecgrepStatusTool) Name() string {
	return "vecgrep_status"
}

func (t *VecgrepStatusTool) Description() string {
	return "Get the status of the vecgrep search index, including number of indexed files and languages."
}

func (t *VecgrepStatusTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *VecgrepStatusTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *VecgrepStatusTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	cmd := exec.CommandContext(ctx, "vecgrep", "status", "--format", "json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not initialized") || strings.Contains(stderr.String(), "no .vecgrep") {
			return "vecgrep is not initialized for this project. Run 'vecgrep init' to enable semantic search.", nil
		}
		return "", fmt.Errorf("vecgrep status failed: %s", stderr.String())
	}

	// Parse and format status
	var status struct {
		Initialized   bool              `json:"initialized"`
		TotalFiles    int               `json:"total_files"`
		TotalChunks   int               `json:"total_chunks"`
		Languages     map[string]int    `json:"languages"`
		LastIndexed   string            `json:"last_indexed"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		// Return raw output if parsing fails
		return stdout.String(), nil
	}

	var sb strings.Builder
	sb.WriteString("## vecgrep Index Status\n\n")
	sb.WriteString(fmt.Sprintf("- **Initialized:** %v\n", status.Initialized))
	sb.WriteString(fmt.Sprintf("- **Total Files:** %d\n", status.TotalFiles))
	sb.WriteString(fmt.Sprintf("- **Total Chunks:** %d\n", status.TotalChunks))
	if status.LastIndexed != "" {
		sb.WriteString(fmt.Sprintf("- **Last Indexed:** %s\n", status.LastIndexed))
	}

	if len(status.Languages) > 0 {
		sb.WriteString("\n**Languages:**\n")
		for lang, count := range status.Languages {
			sb.WriteString(fmt.Sprintf("- %s: %d files\n", lang, count))
		}
	}

	return sb.String(), nil
}
