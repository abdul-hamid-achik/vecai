package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	vecgrepOnce sync.Once
	vecgrepBin  string
)

// autoRAGSearch performs a vecgrep search for the query and returns
// formatted code context suitable for injection before the first LLM call.
// Returns empty string if vecgrep is unavailable or no results found.
func (a *Agent) autoRAGSearch(ctx context.Context, query string) string {
	// Skip trivial queries: commands, very short, or greetings
	if strings.HasPrefix(query, "/") {
		return ""
	}
	trimmed := strings.TrimSpace(query)
	if len(trimmed) < 10 || len(strings.Fields(trimmed)) < 3 {
		return ""
	}

	// Check if vecgrep is available (cached)
	vecgrepOnce.Do(func() {
		vecgrepBin, _ = exec.LookPath("vecgrep")
	})
	if vecgrepBin == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "vecgrep", "search",
		"--query", query,
		"--limit", "3",
		"--mode", "hybrid",
		"--format", "json",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return ""
	}

	contextWindow := a.config.GetContextWindowForModel(a.llm.GetModel())
	return formatRAGResults(stdout.Bytes(), contextWindow)
}

// ragResult represents a single vecgrep search result
type ragResult struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
	Score     float64 `json:"score"`
	Language  string `json:"language"`
	ChunkType string `json:"chunk_type"`
}

// ragResponse represents the vecgrep search response
type ragResponse struct {
	Results []ragResult `json:"results"`
}

// formatRAGResults parses vecgrep JSON output and formats it as compact code context.
// Budget is 15% of the context window in chars, clamped to [3000, 12000].
func formatRAGResults(data []byte, contextWindow int) string {
	var resp ragResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return ""
	}

	if len(resp.Results) == 0 {
		return ""
	}

	// ~4 chars per token, budget = 15% of context window
	maxChars := contextWindow * 4 * 15 / 100
	if maxChars < 3000 {
		maxChars = 3000
	}
	if maxChars > 12000 {
		maxChars = 12000
	}
	var b strings.Builder

	for _, r := range resp.Results {
		if r.Content == "" {
			continue
		}

		// Format: file:startLine-endLine
		header := r.File
		if r.StartLine > 0 {
			if r.EndLine > 0 && r.EndLine != r.StartLine {
				header += ":" + strconv.Itoa(r.StartLine) + "-" + strconv.Itoa(r.EndLine)
			} else {
				header += ":" + strconv.Itoa(r.StartLine)
			}
		}

		snippet := header + "\n" + strings.TrimRight(r.Content, "\n") + "\n\n"

		if b.Len()+len(snippet) > maxChars {
			break
		}
		b.WriteString(snippet)
	}

	return strings.TrimRight(b.String(), "\n")
}

