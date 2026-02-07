package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// autoRAGSearch performs a vecgrep search for the query and returns
// formatted code context suitable for injection before the first LLM call.
// Returns empty string if vecgrep is unavailable or no results found.
func (a *Agent) autoRAGSearch(query string) string {
	// Skip trivial queries: commands, very short, or greetings
	if strings.HasPrefix(query, "/") {
		return ""
	}
	trimmed := strings.TrimSpace(query)
	if len(trimmed) < 10 || len(strings.Fields(trimmed)) < 3 {
		return ""
	}

	// Check if vecgrep is available
	vecgrepPath, err := exec.LookPath("vecgrep")
	if err != nil || vecgrepPath == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	return formatRAGResults(stdout.Bytes())
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
// Caps total output at ~8000 chars (~2000 tokens).
func formatRAGResults(data []byte) string {
	var resp ragResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return ""
	}

	if len(resp.Results) == 0 {
		return ""
	}

	const maxChars = 8000
	var b strings.Builder

	for _, r := range resp.Results {
		if r.Content == "" {
			continue
		}

		// Format: file:startLine-endLine
		header := r.File
		if r.StartLine > 0 {
			if r.EndLine > 0 && r.EndLine != r.StartLine {
				header += ":" + itoa(r.StartLine) + "-" + itoa(r.EndLine)
			} else {
				header += ":" + itoa(r.StartLine)
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

// itoa is a simple int-to-string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
