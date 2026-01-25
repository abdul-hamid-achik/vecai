package context

import (
	"strings"
	"testing"
	"time"
)

func TestNewToolResultCache(t *testing.T) {
	cache := NewToolResultCache(time.Minute)
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if cache.Size() != 0 {
		t.Errorf("expected empty cache, got size %d", cache.Size())
	}
}

func TestToolResultCache_StoreAndGet(t *testing.T) {
	cache := NewToolResultCache(time.Minute)

	toolName := "test_tool"
	input := map[string]any{"path": "/test/file.go"}
	result := "test result content"

	summary, key := cache.Store(toolName, input, result)

	// Summary should equal result for small content
	if summary != result {
		t.Errorf("expected summary to equal result for small content")
	}

	// Key should be non-empty
	if key == "" {
		t.Error("expected non-empty cache key")
	}

	// Should be able to retrieve
	retrieved, ok := cache.Get(key)
	if !ok {
		t.Error("expected to find cached result")
	}
	if retrieved != result {
		t.Errorf("expected %q, got %q", result, retrieved)
	}
}

func TestToolResultCache_GetByTool(t *testing.T) {
	cache := NewToolResultCache(time.Minute)

	toolName := "read_file"
	input := map[string]any{"path": "/test/file.go"}
	result := "file contents here"

	cache.Store(toolName, input, result)

	// Should be able to retrieve by tool name and input
	retrieved, ok := cache.GetByTool(toolName, input)
	if !ok {
		t.Error("expected to find cached result by tool")
	}
	if retrieved != result {
		t.Errorf("expected %q, got %q", result, retrieved)
	}

	// Different input should not match
	_, ok = cache.GetByTool(toolName, map[string]any{"path": "/other/file.go"})
	if ok {
		t.Error("expected not to find cached result for different input")
	}
}

func TestToolResultCache_LargeResultSummary(t *testing.T) {
	cache := NewToolResultCache(time.Minute)

	toolName := "vecgrep_search"
	input := map[string]any{"query": "test"}

	// Create a large result (>500 chars)
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("file.go:10: matching line content here\n")
	}
	largeResult := sb.String()

	summary, key := cache.Store(toolName, input, largeResult)

	// Summary should be shorter than original
	if len(summary) >= len(largeResult) {
		t.Errorf("expected summary to be shorter than original, got %d vs %d", len(summary), len(largeResult))
	}

	// Full result should still be retrievable
	retrieved, ok := cache.Get(key)
	if !ok {
		t.Error("expected to find cached result")
	}
	if retrieved != largeResult {
		t.Error("full result should be preserved in cache")
	}
}

func TestToolResultCache_Clear(t *testing.T) {
	cache := NewToolResultCache(time.Minute)

	cache.Store("tool1", map[string]any{"a": "1"}, "result1")
	cache.Store("tool2", map[string]any{"b": "2"}, "result2")

	if cache.Size() != 2 {
		t.Errorf("expected 2 entries, got %d", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", cache.Size())
	}
}

func TestToolResultCache_NotFound(t *testing.T) {
	cache := NewToolResultCache(time.Minute)

	_, ok := cache.Get("nonexistent-key")
	if ok {
		t.Error("expected not to find nonexistent key")
	}
}

func TestShouldCache(t *testing.T) {
	tests := []struct {
		name     string
		result   string
		expected bool
	}{
		{"small result", "short", false},
		{"medium result", strings.Repeat("x", 400), false},
		{"large result", strings.Repeat("x", 600), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldCache(tt.result); got != tt.expected {
				t.Errorf("ShouldCache(%d chars) = %v, want %v", len(tt.result), got, tt.expected)
			}
		})
	}
}

func TestToolResultCache_SummarizeVecgrep(t *testing.T) {
	cache := NewToolResultCache(time.Minute)

	// Create vecgrep-style output
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString("file.go:10: func TestSomething(t *testing.T)\n")
	}
	result := sb.String()

	summary, _ := cache.Store("vecgrep_search", map[string]any{"query": "test"}, result)

	// Summary should either mention truncation or be smaller than original
	if !strings.Contains(summary, "more results") && len(summary) >= len(result) {
		t.Errorf("expected summary to be truncated or smaller, got %d vs %d chars", len(summary), len(result))
	}
}

func TestToolResultCache_SummarizeFile(t *testing.T) {
	cache := NewToolResultCache(time.Minute)

	// Create file-style output
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("line of code here\n")
	}
	result := sb.String()

	summary, _ := cache.Store("read_file", map[string]any{"path": "test.go"}, result)

	// Summary should be shorter
	if len(summary) >= len(result) {
		t.Error("expected file summary to be shorter than original")
	}
}
