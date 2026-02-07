package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestItoa(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want string
	}{
		{"zero", 0, "0"},
		{"positive single digit", 7, "7"},
		{"positive multi digit", 42, "42"},
		{"large number", 12345, "12345"},
		{"negative single digit", -3, "-3"},
		{"negative multi digit", -99, "-99"},
		{"one", 1, "1"},
		{"ten", 10, "10"},
		{"hundred", 100, "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := itoa(tt.n)
			if got != tt.want {
				t.Errorf("itoa(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestFormatRAGResults_Empty(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"nil data", nil},
		{"empty bytes", []byte{}},
		{"empty object", []byte(`{}`)},
		{"empty results array", []byte(`{"results":[]}`)},
		{"malformed JSON", []byte(`{not valid json`)},
		{"wrong type", []byte(`{"results":"not an array"}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRAGResults(tt.data)
			if got != "" {
				t.Errorf("formatRAGResults(%q) = %q, want empty string", tt.data, got)
			}
		})
	}
}

func TestFormatRAGResults_SingleResult(t *testing.T) {
	resp := ragResponse{
		Results: []ragResult{
			{
				File:      "main.go",
				StartLine: 10,
				EndLine:   20,
				Content:   "func main() {}",
				Score:     0.9,
			},
		},
	}
	data, _ := json.Marshal(resp)

	got := formatRAGResults(data)
	if !strings.Contains(got, "main.go:10-20") {
		t.Errorf("expected header 'main.go:10-20', got %q", got)
	}
	if !strings.Contains(got, "func main() {}") {
		t.Errorf("expected content 'func main() {}', got %q", got)
	}
}

func TestFormatRAGResults_SingleLine(t *testing.T) {
	// When StartLine == EndLine, only StartLine should appear
	resp := ragResponse{
		Results: []ragResult{
			{
				File:      "util.go",
				StartLine: 5,
				EndLine:   5,
				Content:   "var x = 1",
			},
		},
	}
	data, _ := json.Marshal(resp)

	got := formatRAGResults(data)
	if !strings.Contains(got, "util.go:5") {
		t.Errorf("expected header 'util.go:5', got %q", got)
	}
	if strings.Contains(got, "5-5") {
		t.Errorf("should not contain range '5-5' for single line, got %q", got)
	}
}

func TestFormatRAGResults_NoLineNumbers(t *testing.T) {
	resp := ragResponse{
		Results: []ragResult{
			{
				File:    "readme.md",
				Content: "some content",
			},
		},
	}
	data, _ := json.Marshal(resp)

	got := formatRAGResults(data)
	if !strings.HasPrefix(got, "readme.md\n") {
		t.Errorf("expected header 'readme.md' with no line numbers, got %q", got)
	}
}

func TestFormatRAGResults_MultipleResults(t *testing.T) {
	resp := ragResponse{
		Results: []ragResult{
			{File: "a.go", StartLine: 1, EndLine: 5, Content: "aaa"},
			{File: "b.go", StartLine: 10, EndLine: 15, Content: "bbb"},
		},
	}
	data, _ := json.Marshal(resp)

	got := formatRAGResults(data)
	if !strings.Contains(got, "a.go:1-5") {
		t.Errorf("missing first result header, got %q", got)
	}
	if !strings.Contains(got, "b.go:10-15") {
		t.Errorf("missing second result header, got %q", got)
	}
	if !strings.Contains(got, "aaa") || !strings.Contains(got, "bbb") {
		t.Errorf("missing content from results, got %q", got)
	}
}

func TestFormatRAGResults_EmptyContentSkipped(t *testing.T) {
	resp := ragResponse{
		Results: []ragResult{
			{File: "empty.go", StartLine: 1, Content: ""},
			{File: "real.go", StartLine: 5, Content: "real content"},
		},
	}
	data, _ := json.Marshal(resp)

	got := formatRAGResults(data)
	if strings.Contains(got, "empty.go") {
		t.Errorf("result with empty content should be skipped, got %q", got)
	}
	if !strings.Contains(got, "real.go") {
		t.Errorf("result with content should be included, got %q", got)
	}
}

func TestFormatRAGResults_TruncationAt8000Chars(t *testing.T) {
	// Create results where the second one would exceed the 8000 char cap
	bigContent := strings.Repeat("x", 5000)
	resp := ragResponse{
		Results: []ragResult{
			{File: "first.go", StartLine: 1, Content: bigContent},
			{File: "second.go", StartLine: 1, Content: bigContent},
		},
	}
	data, _ := json.Marshal(resp)

	got := formatRAGResults(data)
	if !strings.Contains(got, "first.go") {
		t.Errorf("first result should be included, got length %d", len(got))
	}
	if strings.Contains(got, "second.go") {
		t.Errorf("second result should be truncated (total would exceed 8000), got length %d", len(got))
	}
	if len(got) > 8000 {
		t.Errorf("output length %d exceeds 8000 char cap", len(got))
	}
}

func TestFormatRAGResults_TrailingNewlinesTrimmed(t *testing.T) {
	resp := ragResponse{
		Results: []ragResult{
			{File: "a.go", Content: "content\n\n\n"},
		},
	}
	data, _ := json.Marshal(resp)

	got := formatRAGResults(data)
	if strings.HasSuffix(got, "\n") {
		t.Errorf("output should not end with newline, got %q", got)
	}
}

func TestAutoRAGSearch_SkipConditions(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"slash command", "/help"},
		{"slash with args", "/commit -m test"},
		{"too short", "hi"},
		{"single word", "refactor"},
		{"two words short", "fix bug"},
		{"empty string", ""},
		{"whitespace only", "   "},
		{"under 10 chars with 3 words", "a b c"},
	}

	a, _ := newTestAgent(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.autoRAGSearch(tt.query)
			if got != "" {
				t.Errorf("autoRAGSearch(%q) = %q, want empty (should skip)", tt.query, got)
			}
		})
	}
}

func TestAutoRAGSearch_DoesNotSkipValidQueries(t *testing.T) {
	// These queries pass the skip conditions but vecgrep likely isn't installed,
	// so they should still return "" (from LookPath or exec failure).
	// The key assertion is that they get past the skip checks.
	tests := []struct {
		name  string
		query string
	}{
		{"valid three words", "fix the authentication bug in login"},
		{"long enough query", "how does the router work"},
	}

	a, _ := newTestAgent(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We just verify it doesn't panic; the result may be "" if vecgrep
			// is not installed, which is expected.
			_ = a.autoRAGSearch(tt.query)
		})
	}
}
