package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	// Check default tools are registered
	expectedTools := []string{
		"vecgrep_search",
		"vecgrep_status",
		"read_file",
		"write_file",
		"edit_file",
		"list_files",
		"bash",
		"grep",
	}

	for _, name := range expectedTools {
		if _, ok := r.Get(name); !ok {
			t.Errorf("expected tool %s to be registered", name)
		}
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	tools := r.List()

	if len(tools) != 8 {
		t.Errorf("expected 8 tools, got %d", len(tools))
	}
}

func TestRegistryGetDefinitions(t *testing.T) {
	r := NewRegistry()
	defs := r.GetDefinitions()

	if len(defs) != 8 {
		t.Errorf("expected 8 definitions, got %d", len(defs))
	}

	// Check that definitions have required fields
	for _, def := range defs {
		if def.Name == "" {
			t.Error("tool definition missing name")
		}
		if def.Description == "" {
			t.Errorf("tool %s missing description", def.Name)
		}
		if def.InputSchema == nil {
			t.Errorf("tool %s missing input schema", def.Name)
		}
	}
}

func TestPermissionLevelString(t *testing.T) {
	tests := []struct {
		level    PermissionLevel
		expected string
	}{
		{PermissionRead, "read"},
		{PermissionWrite, "write"},
		{PermissionExecute, "execute"},
		{PermissionLevel(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("PermissionLevel(%d).String() = %s, want %s", tt.level, got, tt.expected)
		}
	}
}

func TestReadFileTool(t *testing.T) {
	tool := &ReadFileTool{}

	if tool.Name() != "read_file" {
		t.Errorf("expected name 'read_file', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}

	// Create temp file
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	content := "line 1\nline 2\nline 3\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test reading entire file
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != content {
		t.Errorf("expected %q, got %q", content, result)
	}

	// Test reading with line range
	result, err = tool.Execute(context.Background(), map[string]any{
		"path":       testFile,
		"start_line": float64(2),
		"end_line":   float64(2),
	})
	if err != nil {
		t.Fatalf("Execute with line range failed: %v", err)
	}
	if !strings.Contains(result, "line 2") {
		t.Errorf("expected result to contain 'line 2', got %q", result)
	}

	// Test file not found
	_, err = tool.Execute(context.Background(), map[string]any{
		"path": "/nonexistent/file.txt",
	})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}

	// Test missing path
	_, err = tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestWriteFileTool(t *testing.T) {
	tool := &WriteFileTool{}

	if tool.Name() != "write_file" {
		t.Errorf("expected name 'write_file', got %s", tool.Name())
	}

	if tool.Permission() != PermissionWrite {
		t.Errorf("expected permission Write, got %v", tool.Permission())
	}

	// Create temp directory
	dir := t.TempDir()
	testFile := filepath.Join(dir, "subdir", "test.txt")
	content := "test content"

	// Test writing file (creates directories)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":    testFile,
		"content": content,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "Successfully wrote") {
		t.Errorf("unexpected result: %s", result)
	}

	// Verify file was created
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}

	// Test missing path
	_, err = tool.Execute(context.Background(), map[string]any{
		"content": "test",
	})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestEditFileTool(t *testing.T) {
	tool := &EditFileTool{}

	if tool.Name() != "edit_file" {
		t.Errorf("expected name 'edit_file', got %s", tool.Name())
	}

	if tool.Permission() != PermissionWrite {
		t.Errorf("expected permission Write, got %v", tool.Permission())
	}

	// Create temp file
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test editing file
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":     testFile,
		"old_text": "world",
		"new_text": "universe",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "Successfully edited") {
		t.Errorf("unexpected result: %s", result)
	}

	// Verify edit
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello universe" {
		t.Errorf("expected 'hello universe', got %q", string(data))
	}

	// Test old_text not found
	_, err = tool.Execute(context.Background(), map[string]any{
		"path":     testFile,
		"old_text": "nonexistent",
		"new_text": "replacement",
	})
	if err == nil {
		t.Error("expected error when old_text not found")
	}

	// Test multiple occurrences
	if err := os.WriteFile(testFile, []byte("foo foo foo"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = tool.Execute(context.Background(), map[string]any{
		"path":     testFile,
		"old_text": "foo",
		"new_text": "bar",
	})
	if err == nil {
		t.Error("expected error for multiple occurrences")
	}
}

func TestListFilesTool(t *testing.T) {
	tool := &ListFilesTool{}

	if tool.Name() != "list_files" {
		t.Errorf("expected name 'list_files', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}

	// Create temp directory with files
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(dir, "file2.go"), []byte(""), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	// Test listing
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": dir,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "file1.txt") {
		t.Errorf("expected result to contain 'file1.txt', got %q", result)
	}
	if !strings.Contains(result, "subdir") {
		t.Errorf("expected result to contain 'subdir', got %q", result)
	}

	// Test with pattern
	result, err = tool.Execute(context.Background(), map[string]any{
		"path":    dir,
		"pattern": "*.go",
	})
	if err != nil {
		t.Fatalf("Execute with pattern failed: %v", err)
	}
	if !strings.Contains(result, "file2.go") {
		t.Errorf("expected result to contain 'file2.go', got %q", result)
	}
	if strings.Contains(result, "file1.txt") {
		t.Errorf("result should not contain 'file1.txt' with *.go pattern")
	}
}

func TestBashTool(t *testing.T) {
	tool := &BashTool{}

	if tool.Name() != "bash" {
		t.Errorf("expected name 'bash', got %s", tool.Name())
	}

	if tool.Permission() != PermissionExecute {
		t.Errorf("expected permission Execute, got %v", tool.Permission())
	}

	// Test simple command
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("expected 'hello' in result, got %q", result)
	}

	// Test missing command
	_, err = tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing command")
	}

	// Test command with error
	result, err = tool.Execute(context.Background(), map[string]any{
		"command": "exit 1",
	})
	if err != nil {
		t.Fatalf("Execute should not return error for exit code: %v", err)
	}
	if !strings.Contains(result, "Exit code") {
		t.Errorf("expected exit code in result, got %q", result)
	}
}

func TestGrepTool(t *testing.T) {
	tool := &GrepTool{}

	if tool.Name() != "grep" {
		t.Errorf("expected name 'grep', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}

	// Create temp directory with files
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\nfoo bar\n"), 0644)

	// Test grep (this may use rg or grep depending on system)
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "hello",
		"path":    dir,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "hello") && !strings.Contains(result, "No matches") {
		t.Errorf("unexpected result: %q", result)
	}

	// Test no matches
	result, err = tool.Execute(context.Background(), map[string]any{
		"pattern": "nonexistent_pattern_xyz",
		"path":    dir,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "No matches") {
		t.Errorf("expected 'No matches', got %q", result)
	}
}

func TestToolInputSchemas(t *testing.T) {
	r := NewRegistry()

	for _, tool := range r.List() {
		schema := tool.InputSchema()

		// Check schema has type
		if schemaType, ok := schema["type"].(string); !ok || schemaType != "object" {
			t.Errorf("tool %s schema should have type 'object'", tool.Name())
		}

		// Check schema has properties
		if _, ok := schema["properties"].(map[string]any); !ok {
			t.Errorf("tool %s schema should have properties", tool.Name())
		}
	}
}
