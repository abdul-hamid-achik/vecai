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
	return "Perform semantic, keyword, or hybrid search across the codebase using vector embeddings. Returns code chunks matching the query with rich filtering options."
}

func (t *VecgrepSearchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query. Can be natural language for semantic search or keywords for keyword search.",
			},
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"hybrid", "semantic", "keyword"},
				"description": "Search mode: 'hybrid' combines vector+text (default), 'semantic' uses pure vector similarity, 'keyword' uses text matching.",
				"default":     "hybrid",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default: 10).",
				"default":     10,
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Filter results by a single programming language (e.g., 'go', 'python').",
			},
			"languages": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Filter results by multiple programming languages (e.g., ['go', 'rust']).",
			},
			"chunk_type": map[string]any{
				"type":        "string",
				"description": "Filter by chunk type (e.g., 'function', 'class', 'block', 'method').",
			},
			"chunk_types": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Filter by multiple chunk types (e.g., ['function', 'method']).",
			},
			"file_pattern": map[string]any{
				"type":        "string",
				"description": "Filter results by file path pattern (glob, e.g., '**/*_test.go').",
			},
			"directory": map[string]any{
				"type":        "string",
				"description": "Filter results by directory prefix (e.g., 'internal/').",
			},
			"min_line": map[string]any{
				"type":        "integer",
				"description": "Filter by minimum start line number.",
			},
			"max_line": map[string]any{
				"type":        "integer",
				"description": "Filter by maximum start line number.",
			},
			"explain": map[string]any{
				"type":        "boolean",
				"description": "Return search diagnostics including index type, nodes visited, and timing.",
				"default":     false,
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

	// Search mode
	if mode, ok := input["mode"].(string); ok && mode != "" {
		args = append(args, "--mode", mode)
	}

	// Limit
	if limit, ok := input["limit"].(float64); ok {
		args = append(args, "--limit", fmt.Sprintf("%d", int(limit)))
	}

	// Language filters
	if lang, ok := input["language"].(string); ok && lang != "" {
		args = append(args, "--lang", lang)
	}
	if languages, ok := input["languages"].([]any); ok && len(languages) > 0 {
		langs := make([]string, 0, len(languages))
		for _, l := range languages {
			if s, ok := l.(string); ok {
				langs = append(langs, s)
			}
		}
		if len(langs) > 0 {
			args = append(args, "--languages", strings.Join(langs, ","))
		}
	}

	// Chunk type filters
	if chunkType, ok := input["chunk_type"].(string); ok && chunkType != "" {
		args = append(args, "--type", chunkType)
	}
	if chunkTypes, ok := input["chunk_types"].([]any); ok && len(chunkTypes) > 0 {
		types := make([]string, 0, len(chunkTypes))
		for _, t := range chunkTypes {
			if s, ok := t.(string); ok {
				types = append(types, s)
			}
		}
		if len(types) > 0 {
			args = append(args, "--types", strings.Join(types, ","))
		}
	}

	// File pattern
	if pattern, ok := input["file_pattern"].(string); ok && pattern != "" {
		args = append(args, "--file", pattern)
	}

	// Directory filter
	if dir, ok := input["directory"].(string); ok && dir != "" {
		args = append(args, "--dir", dir)
	}

	// Line range filter
	minLine, hasMin := input["min_line"].(float64)
	maxLine, hasMax := input["max_line"].(float64)
	if hasMin && hasMax {
		args = append(args, "--lines", fmt.Sprintf("%d-%d", int(minLine), int(maxLine)))
	} else if hasMin {
		args = append(args, "--lines", fmt.Sprintf("%d-", int(minLine)))
	} else if hasMax {
		args = append(args, "--lines", fmt.Sprintf("-%d", int(maxLine)))
	}

	// Explain mode
	if explain, ok := input["explain"].(bool); ok && explain {
		args = append(args, "--explain")
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
	fmt.Fprintf(&sb, "Found %d results:\n\n", len(results))

	for i, r := range results {
		fmt.Fprintf(&sb, "### Result %d (%.2f%% match)\n", i+1, r.Similarity*100)
		fmt.Fprintf(&sb, "**File:** %s:%d-%d\n", r.File, r.StartLine, r.EndLine)
		fmt.Fprintf(&sb, "**Language:** %s | **Type:** %s\n", r.Language, r.ChunkType)
		sb.WriteString("```" + r.Language + "\n")
		sb.WriteString(r.Content)
		if !strings.HasSuffix(r.Content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("```\n\n")
	}

	return sb.String(), nil
}

// VecgrepSimilarTool finds code similar to a given snippet or location
type VecgrepSimilarTool struct{}

func (t *VecgrepSimilarTool) Name() string {
	return "vecgrep_similar"
}

func (t *VecgrepSimilarTool) Description() string {
	return "Find code similar to a given snippet, file location, or chunk ID. Use this to discover related patterns, implementations, or potential duplicates."
}

func (t *VecgrepSimilarTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Code snippet to find similar code for.",
			},
			"file_location": map[string]any{
				"type":        "string",
				"description": "File:line location to find similar code (e.g., 'internal/agent/agent.go:50').",
			},
			"chunk_id": map[string]any{
				"type":        "integer",
				"description": "Chunk ID from a previous search result to find similar code for.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default: 5).",
				"default":     5,
			},
			"exclude_same_file": map[string]any{
				"type":        "boolean",
				"description": "Exclude results from the same file (default: true).",
				"default":     true,
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Filter results by a single programming language.",
			},
			"languages": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Filter results by multiple programming languages.",
			},
			"chunk_type": map[string]any{
				"type":        "string",
				"description": "Filter by chunk type (e.g., 'function', 'class', 'method').",
			},
			"chunk_types": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Filter by multiple chunk types.",
			},
			"file_pattern": map[string]any{
				"type":        "string",
				"description": "Filter results by file path pattern (glob).",
			},
			"directory": map[string]any{
				"type":        "string",
				"description": "Filter results by directory prefix.",
			},
			"min_line": map[string]any{
				"type":        "integer",
				"description": "Filter by minimum start line number.",
			},
			"max_line": map[string]any{
				"type":        "integer",
				"description": "Filter by maximum start line number.",
			},
		},
	}
}

func (t *VecgrepSimilarTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *VecgrepSimilarTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	args := []string{"similar"}

	// One of text, file_location, or chunk_id must be provided
	text, hasText := input["text"].(string)
	fileLoc, hasFileLoc := input["file_location"].(string)
	chunkID, hasChunkID := input["chunk_id"].(float64)

	if !hasText && !hasFileLoc && !hasChunkID {
		return "", fmt.Errorf("one of 'text', 'file_location', or 'chunk_id' is required")
	}

	if hasText && text != "" {
		args = append(args, "--text", text)
	} else if hasFileLoc && fileLoc != "" {
		args = append(args, fileLoc)
	} else if hasChunkID {
		args = append(args, fmt.Sprintf("%d", int(chunkID)))
	}

	// Limit
	if limit, ok := input["limit"].(float64); ok {
		args = append(args, "--limit", fmt.Sprintf("%d", int(limit)))
	} else {
		args = append(args, "--limit", "5")
	}

	// Exclude same file
	excludeSameFile := true
	if exclude, ok := input["exclude_same_file"].(bool); ok {
		excludeSameFile = exclude
	}
	if excludeSameFile {
		args = append(args, "--exclude-same-file")
	}

	// Language filters
	if lang, ok := input["language"].(string); ok && lang != "" {
		args = append(args, "--lang", lang)
	}
	if languages, ok := input["languages"].([]any); ok && len(languages) > 0 {
		langs := make([]string, 0, len(languages))
		for _, l := range languages {
			if s, ok := l.(string); ok {
				langs = append(langs, s)
			}
		}
		if len(langs) > 0 {
			args = append(args, "--languages", strings.Join(langs, ","))
		}
	}

	// Chunk type filters
	if chunkType, ok := input["chunk_type"].(string); ok && chunkType != "" {
		args = append(args, "--type", chunkType)
	}
	if chunkTypes, ok := input["chunk_types"].([]any); ok && len(chunkTypes) > 0 {
		types := make([]string, 0, len(chunkTypes))
		for _, t := range chunkTypes {
			if s, ok := t.(string); ok {
				types = append(types, s)
			}
		}
		if len(types) > 0 {
			args = append(args, "--types", strings.Join(types, ","))
		}
	}

	// File pattern
	if pattern, ok := input["file_pattern"].(string); ok && pattern != "" {
		args = append(args, "--file", pattern)
	}

	// Directory filter
	if dir, ok := input["directory"].(string); ok && dir != "" {
		args = append(args, "--dir", dir)
	}

	// Line range filter
	minLine, hasMin := input["min_line"].(float64)
	maxLine, hasMax := input["max_line"].(float64)
	if hasMin && hasMax {
		args = append(args, "--lines", fmt.Sprintf("%d-%d", int(minLine), int(maxLine)))
	} else if hasMin {
		args = append(args, "--lines", fmt.Sprintf("%d-", int(minLine)))
	} else if hasMax {
		args = append(args, "--lines", fmt.Sprintf("-%d", int(maxLine)))
	}

	args = append(args, "--format", "json")

	cmd := exec.CommandContext(ctx, "vecgrep", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not initialized") || strings.Contains(stderr.String(), "no .vecgrep") {
			return "vecgrep is not initialized for this project. Please run 'vecgrep init' to set up semantic search.", nil
		}
		return "", fmt.Errorf("vecgrep similar failed: %s", stderr.String())
	}

	return formatSimilarResults(stdout.String())
}

func formatSimilarResults(jsonOutput string) (string, error) {
	if strings.TrimSpace(jsonOutput) == "" {
		return "No similar code found.", nil
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
		return jsonOutput, nil
	}

	if len(results) == 0 {
		return "No similar code found.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d similar code patterns:\n\n", len(results))

	for i, r := range results {
		fmt.Fprintf(&sb, "### Similar #%d (%.1f%% similarity)\n", i+1, r.Similarity*100)
		fmt.Fprintf(&sb, "**File:** %s:%d-%d\n", r.File, r.StartLine, r.EndLine)
		fmt.Fprintf(&sb, "**Language:** %s | **Type:** %s\n", r.Language, r.ChunkType)
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
	fmt.Fprintf(&sb, "- **Initialized:** %v\n", status.Initialized)
	fmt.Fprintf(&sb, "- **Total Files:** %d\n", status.TotalFiles)
	fmt.Fprintf(&sb, "- **Total Chunks:** %d\n", status.TotalChunks)
	if status.LastIndexed != "" {
		fmt.Fprintf(&sb, "- **Last Indexed:** %s\n", status.LastIndexed)
	}

	if len(status.Languages) > 0 {
		sb.WriteString("\n**Languages:**\n")
		for lang, count := range status.Languages {
			fmt.Fprintf(&sb, "- %s: %d files\n", lang, count)
		}
	}

	return sb.String(), nil
}

// VecgrepIndexTool triggers re-indexing of the codebase
type VecgrepIndexTool struct{}

func (t *VecgrepIndexTool) Name() string {
	return "vecgrep_index"
}

func (t *VecgrepIndexTool) Description() string {
	return "Index or re-index files in the project for semantic search. Only re-indexes files that have changed unless force is set."
}

func (t *VecgrepIndexTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"paths": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Specific paths to index. If empty, indexes the entire project.",
			},
			"force": map[string]any{
				"type":        "boolean",
				"description": "Force re-indexing of all files even if unchanged (default: false).",
				"default":     false,
			},
		},
	}
}

func (t *VecgrepIndexTool) Permission() PermissionLevel {
	return PermissionWrite // Modifies the index database
}

func (t *VecgrepIndexTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	args := []string{"index"}

	// Force flag
	if force, ok := input["force"].(bool); ok && force {
		args = append(args, "--full")
	}

	// Specific paths
	if paths, ok := input["paths"].([]any); ok && len(paths) > 0 {
		for _, p := range paths {
			if s, ok := p.(string); ok {
				args = append(args, s)
			}
		}
	}

	// Add verbose for progress
	args = append(args, "--verbose")

	cmd := exec.CommandContext(ctx, "vecgrep", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not initialized") || strings.Contains(stderr.String(), "no .vecgrep") {
			return "vecgrep is not initialized for this project. Run 'vecgrep init' first.", nil
		}
		return "", fmt.Errorf("vecgrep index failed: %s", stderr.String())
	}

	output := stdout.String()
	if output == "" {
		output = "Indexing complete."
	}
	return output, nil
}

// VecgrepCleanTool removes orphaned data from the index
type VecgrepCleanTool struct{}

func (t *VecgrepCleanTool) Name() string {
	return "vecgrep_clean"
}

func (t *VecgrepCleanTool) Description() string {
	return "Remove orphaned data (chunks without files, embeddings without chunks) and optimize the database."
}

func (t *VecgrepCleanTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *VecgrepCleanTool) Permission() PermissionLevel {
	return PermissionWrite // Modifies the index database
}

func (t *VecgrepCleanTool) Execute(ctx context.Context, _ map[string]any) (string, error) {
	cmd := exec.CommandContext(ctx, "vecgrep", "clean")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not initialized") || strings.Contains(stderr.String(), "no .vecgrep") {
			return "vecgrep is not initialized for this project.", nil
		}
		return "", fmt.Errorf("vecgrep clean failed: %s", stderr.String())
	}

	output := stdout.String()
	if output == "" {
		output = "Database cleaned successfully."
	}
	return output, nil
}

// VecgrepDeleteTool removes a file from the index
type VecgrepDeleteTool struct{}

func (t *VecgrepDeleteTool) Name() string {
	return "vecgrep_delete"
}

func (t *VecgrepDeleteTool) Description() string {
	return "Delete a file and all its chunks from the search index."
}

func (t *VecgrepDeleteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The file path to delete from the index.",
			},
		},
		"required": []string{"file_path"},
	}
}

func (t *VecgrepDeleteTool) Permission() PermissionLevel {
	return PermissionWrite // Modifies the index database
}

func (t *VecgrepDeleteTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok || filePath == "" {
		return "", fmt.Errorf("file_path is required")
	}

	cmd := exec.CommandContext(ctx, "vecgrep", "delete", filePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not initialized") || strings.Contains(stderr.String(), "no .vecgrep") {
			return "vecgrep is not initialized for this project.", nil
		}
		return "", fmt.Errorf("vecgrep delete failed: %s", stderr.String())
	}

	output := stdout.String()
	if output == "" {
		output = fmt.Sprintf("File '%s' deleted from index.", filePath)
	}
	return output, nil
}

// VecgrepInitTool initializes vecgrep in the current project
type VecgrepInitTool struct{}

func (t *VecgrepInitTool) Name() string {
	return "vecgrep_init"
}

func (t *VecgrepInitTool) Description() string {
	return "Initialize vecgrep for semantic search in the current project. Creates a .vecgrep directory with configuration and database."
}

func (t *VecgrepInitTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"force": map[string]any{
				"type":        "boolean",
				"description": "Overwrite existing configuration if present.",
				"default":     false,
			},
		},
	}
}

func (t *VecgrepInitTool) Permission() PermissionLevel {
	return PermissionWrite // Creates files
}

func (t *VecgrepInitTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	args := []string{"init"}

	if force, ok := input["force"].(bool); ok && force {
		args = append(args, "--force")
	}

	cmd := exec.CommandContext(ctx, "vecgrep", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("vecgrep init failed: %s", stderr.String())
	}

	output := stdout.String()
	if output == "" {
		output = "vecgrep initialized successfully. Run 'vecgrep index' to index your codebase."
	}
	return output, nil
}
