package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

var (
	vecgrepAvailOnce sync.Once
	vecgrepAvail     bool
)

// checkVecgrepAvailable checks if vecgrep CLI is installed (cached)
func checkVecgrepAvailable() bool {
	vecgrepAvailOnce.Do(func() {
		_, err := exec.LookPath("vecgrep")
		vecgrepAvail = err == nil
	})
	return vecgrepAvail
}

// vecgrepResult represents a single vecgrep search result
type vecgrepResult struct {
	File       string  `json:"file"`
	StartLine  int     `json:"start_line"`
	EndLine    int     `json:"end_line"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Similarity float64 `json:"similarity"`
	Language   string  `json:"language"`
	ChunkType  string  `json:"chunk_type"`
}

// vecgrepResponse represents the vecgrep search response envelope
type vecgrepResponse struct {
	Results []vecgrepResult `json:"results"`
}

// SearchVecgrep runs a vecgrep semantic search and returns CompletionItems.
// queryContext is optional surrounding text to improve relevance (e.g., the query before @).
// Safe to call from a goroutine. Returns nil on any error.
func SearchVecgrep(ctx context.Context, query string, queryContext string, projectRoot string, limit int) []CompletionItem {
	if !checkVecgrepAvailable() {
		return nil
	}
	if len(query) < 2 && len(queryContext) < 5 {
		return nil
	}
	if limit <= 0 {
		limit = 8
	}

	// Combine @ query with surrounding context for better semantic search
	searchQuery := query
	if queryContext != "" {
		searchQuery = queryContext + " " + query
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "vecgrep", "search",
		"--query", searchQuery,
		"--limit", itoa(limit),
		"--mode", "hybrid",
		"--format", "json",
	)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil
	}

	var resp vecgrepResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil
	}

	if len(resp.Results) == 0 {
		return nil
	}

	// Deduplicate by file path, keeping highest-scoring entry per file
	seen := make(map[string]bool)
	var items []CompletionItem
	for _, r := range resp.Results {
		if r.File == "" || seen[r.File] {
			continue
		}
		seen[r.File] = true

		score := r.Score
		if score == 0 {
			score = r.Similarity
		}

		lang := r.Language
		if lang == "" {
			lang = langFromExt(filepath.Ext(r.File))
		}

		dir := filepath.Dir(r.File)
		if dir == "." {
			dir = ""
		}

		absPath := r.File
		if projectRoot != "" && !filepath.IsAbs(r.File) {
			absPath = filepath.Join(projectRoot, r.File)
		}

		items = append(items, CompletionItem{
			Label:      filepath.Base(r.File),
			Detail:     dir,
			Kind:       KindFile,
			InsertText: "@" + r.File + " ",
			Score:      score,
			FilePath:   absPath,
			Language:   lang,
		})
	}

	return items
}
