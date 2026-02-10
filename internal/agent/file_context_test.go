package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/tui"
)

func TestFormatTaggedFileContextEmpty(t *testing.T) {
	result := formatTaggedFileContext(nil)
	if result != "" {
		t.Errorf("Expected empty for nil files, got %q", result)
	}

	result = formatTaggedFileContext([]tui.TaggedFile{})
	if result != "" {
		t.Errorf("Expected empty for empty files, got %q", result)
	}
}

func TestFormatTaggedFileContextSingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	files := []tui.TaggedFile{
		{RelPath: "main.go", AbsPath: path, Language: "Go"},
	}

	result := formatTaggedFileContext(files)
	if result == "" {
		t.Fatal("Expected non-empty result")
	}

	// Should contain the file path
	if !strings.Contains(result, "main.go") {
		t.Error("Expected result to contain file path")
	}

	// Should contain the file content
	if !strings.Contains(result, "package main") {
		t.Error("Expected result to contain file content")
	}

	// Should have code fence
	if !strings.Contains(result, "```go") {
		t.Error("Expected result to have code fence with language")
	}
}

func TestFormatTaggedFileContextMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	goPath := filepath.Join(tmpDir, "handler.go")
	if err := os.WriteFile(goPath, []byte("package api\n\nfunc Handle() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	pyPath := filepath.Join(tmpDir, "script.py")
	if err := os.WriteFile(pyPath, []byte("def main():\n    pass"), 0644); err != nil {
		t.Fatal(err)
	}

	files := []tui.TaggedFile{
		{RelPath: "handler.go", AbsPath: goPath, Language: "Go"},
		{RelPath: "script.py", AbsPath: pyPath, Language: "Python"},
	}

	result := formatTaggedFileContext(files)
	if !strings.Contains(result, "handler.go") {
		t.Error("Expected handler.go in result")
	}
	if !strings.Contains(result, "script.py") {
		t.Error("Expected script.py in result")
	}
}

func TestFormatTaggedFileContextMissingFile(t *testing.T) {
	files := []tui.TaggedFile{
		{RelPath: "nonexistent.go", AbsPath: "/nonexistent/path/file.go", Language: "Go"},
	}

	result := formatTaggedFileContext(files)
	if result != "" {
		t.Errorf("Expected empty for missing file, got %q", result)
	}
}

func TestFormatTaggedFileContextTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a large file (> maxSingleFileChars)
	largePath := filepath.Join(tmpDir, "large.go")
	content := strings.Repeat("x", maxSingleFileChars+1000)
	if err := os.WriteFile(largePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	files := []tui.TaggedFile{
		{RelPath: "large.go", AbsPath: largePath, Language: "Go"},
	}

	result := formatTaggedFileContext(files)
	if !strings.Contains(result, "truncated") {
		t.Error("Expected truncation indicator for large file")
	}
}

func TestFormatTaggedFileContextBudget(t *testing.T) {
	tmpDir := t.TempDir()

	// Create enough files to exceed the budget
	var files []tui.TaggedFile
	for i := 0; i < 10; i++ {
		name := "file" + strings.Repeat("x", 100) + ".go"
		path := filepath.Join(tmpDir, name)
		// Each file ~5000 chars, total budget is 20000
		if err := os.WriteFile(path, []byte(strings.Repeat("code\n", 1000)), 0644); err != nil {
			t.Fatal(err)
		}
		files = append(files, tui.TaggedFile{
			RelPath: name,
			AbsPath: path,
			Language: "Go",
		})
	}

	result := formatTaggedFileContext(files)
	// Should not exceed budget by much (allow for headers/fences)
	if len(result) > maxFileContextChars+2000 {
		t.Errorf("Result too large: %d chars (budget: %d)", len(result), maxFileContextChars)
	}
}

func TestFormatTaggedFileContextUsesRelPath(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(path, []byte("print('hello')"), 0644); err != nil {
		t.Fatal(err)
	}

	// AbsPath empty, should fall back to RelPath
	files := []tui.TaggedFile{
		{RelPath: path, AbsPath: "", Language: "Python"},
	}

	result := formatTaggedFileContext(files)
	if !strings.Contains(result, "print") {
		t.Error("Expected file content when using RelPath fallback")
	}
}

func TestFormatFileSnippet(t *testing.T) {
	f := tui.TaggedFile{RelPath: "internal/router.go", Language: "Go"}
	snippet := formatFileSnippet(f, "package agent\n\nfunc Route() {}")

	if !strings.Contains(snippet, "### internal/router.go") {
		t.Error("Expected header with path")
	}
	if !strings.Contains(snippet, "(Go)") {
		t.Error("Expected language label")
	}
	if !strings.Contains(snippet, "```go") {
		t.Error("Expected code fence with language")
	}
	if !strings.Contains(snippet, "func Route()") {
		t.Error("Expected file content in snippet")
	}
}
