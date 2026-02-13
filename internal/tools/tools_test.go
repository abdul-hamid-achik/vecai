package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdirTemp changes to the given directory for the duration of the test,
// restoring the original working directory and projectRoot on cleanup.
// This is needed because the file tools validate that paths are within
// the project directory (projectRoot).
func chdirTemp(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Resolve symlinks so that ValidatePath works correctly
	// on macOS where /var -> /private/var
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(resolved); err != nil {
		t.Fatal(err)
	}
	// Set projectRoot to the resolved temp dir so ValidatePath accepts paths here
	origRoot := projectRoot
	projectRoot = resolved
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		projectRoot = origRoot
	})
}

func TestRegistry(t *testing.T) {
	r := NewRegistry(nil)

	// Check default tools are registered
	expectedTools := []string{
		"vecgrep_search",
		"vecgrep_similar",
		"vecgrep_status",
		"vecgrep_index",
		"vecgrep_clean",
		"vecgrep_delete",
		"vecgrep_init",
		"read_file",
		"write_file",
		"edit_file",
		"list_files",
		"bash",
		"grep",
		// gpeek tools
		"gpeek_status",
		"gpeek_diff",
		"gpeek_log",
		"gpeek_summary",
		"gpeek_blame",
		"gpeek_branches",
		"gpeek_stashes",
		"gpeek_tags",
		"gpeek_changes_between",
		"gpeek_conflict_check",
		// smart tools
		"ast_parse",
		"lsp_query",
		"lint",
		"test_run",
	}

	for _, name := range expectedTools {
		if _, ok := r.Get(name); !ok {
			t.Errorf("expected tool %s to be registered", name)
		}
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry(nil)
	tools := r.List()

	// Base count is 27, plus up to 4 optional tools:
	// - web_search (if TAVILY_API_KEY is set)
	// - noted_remember, noted_recall, noted_forget (if noted CLI is installed)
	// Tools: vecgrep(7) + file(4) + bash(1) + grep(1) + gpeek(10) + smart(4) = 27
	minExpected := 27
	maxExpected := 31 // 27 + 1 (web) + 3 (noted)
	if len(tools) < minExpected || len(tools) > maxExpected {
		t.Errorf("expected %d-%d tools, got %d", minExpected, maxExpected, len(tools))
	}
}

func TestRegistryGetDefinitions(t *testing.T) {
	r := NewRegistry(nil)
	defs := r.GetDefinitions()

	// Base count is 27, plus up to 4 optional tools
	// Tools: vecgrep(7) + file(4) + bash(1) + grep(1) + gpeek(10) + smart(4) = 27
	minExpected := 27
	maxExpected := 31 // 27 + 1 (web) + 3 (noted)
	if len(defs) < minExpected || len(defs) > maxExpected {
		t.Errorf("expected %d-%d definitions, got %d", minExpected, maxExpected, len(defs))
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

	// Create temp file and chdir into it for path validation
	dir := t.TempDir()
	chdirTemp(t, dir)
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

	// Create temp directory and chdir into it for path validation
	dir := t.TempDir()
	chdirTemp(t, dir)
	relFile := filepath.Join("subdir", "test.txt")
	content := "test content"

	// Test writing file (creates directories)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":    relFile,
		"content": content,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "Successfully wrote") {
		t.Errorf("unexpected result: %s", result)
	}

	// Verify file was created
	data, err := os.ReadFile(relFile)
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

	// Create temp file and chdir into it for path validation
	dir := t.TempDir()
	chdirTemp(t, dir)
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

	// Create temp directory with files and set up project root
	dir := t.TempDir()
	chdirTemp(t, dir)
	_ = os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(dir, "file2.go"), []byte(""), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	// Resolve dir for path validation (macOS /tmp -> /private/tmp)
	resolvedDir, _ := filepath.EvalSymlinks(dir)

	// Test listing
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": resolvedDir,
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
		"path":    resolvedDir,
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

	// Create temp directory with files and set up project root
	dir := t.TempDir()
	chdirTemp(t, dir)
	_ = os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\nfoo bar\n"), 0644)

	// Resolve dir for path validation (macOS /tmp -> /private/tmp)
	resolvedDir, _ := filepath.EvalSymlinks(dir)

	// Test grep (this may use rg or grep depending on system)
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "hello",
		"path":    resolvedDir,
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
		"path":    resolvedDir,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "No matches") {
		t.Errorf("expected 'No matches', got %q", result)
	}
}

func TestToolInputSchemas(t *testing.T) {
	r := NewRegistry(nil)

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

// Gpeek tool tests

func TestGpeekStatusTool(t *testing.T) {
	tool := &GpeekStatusTool{}

	if tool.Name() != "gpeek_status" {
		t.Errorf("expected name 'gpeek_status', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}

	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}
	if _, ok := props["path"]; !ok {
		t.Error("expected 'path' in schema properties")
	}
}

func TestGpeekDiffTool(t *testing.T) {
	tool := &GpeekDiffTool{}

	if tool.Name() != "gpeek_diff" {
		t.Errorf("expected name 'gpeek_diff', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}

	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}

	expectedProps := []string{"path", "file", "staged", "commit"}
	for _, prop := range expectedProps {
		if _, ok := props[prop]; !ok {
			t.Errorf("expected '%s' in schema properties", prop)
		}
	}
}

func TestGpeekLogTool(t *testing.T) {
	tool := &GpeekLogTool{}

	if tool.Name() != "gpeek_log" {
		t.Errorf("expected name 'gpeek_log', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}

	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}

	expectedProps := []string{"path", "limit", "author", "since"}
	for _, prop := range expectedProps {
		if _, ok := props[prop]; !ok {
			t.Errorf("expected '%s' in schema properties", prop)
		}
	}
}

func TestGpeekSummaryTool(t *testing.T) {
	tool := &GpeekSummaryTool{}

	if tool.Name() != "gpeek_summary" {
		t.Errorf("expected name 'gpeek_summary', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}

	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}

	expectedProps := []string{"path", "commits"}
	for _, prop := range expectedProps {
		if _, ok := props[prop]; !ok {
			t.Errorf("expected '%s' in schema properties", prop)
		}
	}
}

func TestGpeekBlameTool(t *testing.T) {
	tool := &GpeekBlameTool{}

	if tool.Name() != "gpeek_blame" {
		t.Errorf("expected name 'gpeek_blame', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}

	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}

	expectedProps := []string{"path", "file", "start_line", "end_line"}
	for _, prop := range expectedProps {
		if _, ok := props[prop]; !ok {
			t.Errorf("expected '%s' in schema properties", prop)
		}
	}

	// Check required fields
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("expected required in schema")
	}
	if len(required) != 1 || required[0] != "file" {
		t.Error("expected 'file' to be required")
	}

	// Test missing required field
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing file parameter")
	}
}

func TestGpeekBranchesTool(t *testing.T) {
	tool := &GpeekBranchesTool{}

	if tool.Name() != "gpeek_branches" {
		t.Errorf("expected name 'gpeek_branches', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}

	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}

	expectedProps := []string{"path", "all"}
	for _, prop := range expectedProps {
		if _, ok := props[prop]; !ok {
			t.Errorf("expected '%s' in schema properties", prop)
		}
	}
}

func TestGpeekStashesTool(t *testing.T) {
	tool := &GpeekStashesTool{}

	if tool.Name() != "gpeek_stashes" {
		t.Errorf("expected name 'gpeek_stashes', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}
}

func TestGpeekTagsTool(t *testing.T) {
	tool := &GpeekTagsTool{}

	if tool.Name() != "gpeek_tags" {
		t.Errorf("expected name 'gpeek_tags', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}
}

func TestGpeekChangesBetweenTool(t *testing.T) {
	tool := &GpeekChangesBetweenTool{}

	if tool.Name() != "gpeek_changes_between" {
		t.Errorf("expected name 'gpeek_changes_between', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}

	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}

	expectedProps := []string{"path", "from", "to"}
	for _, prop := range expectedProps {
		if _, ok := props[prop]; !ok {
			t.Errorf("expected '%s' in schema properties", prop)
		}
	}

	// Check required fields
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("expected required in schema")
	}
	if len(required) != 1 || required[0] != "from" {
		t.Error("expected 'from' to be required")
	}

	// Test missing required field
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing from parameter")
	}
}

func TestGpeekConflictCheckTool(t *testing.T) {
	tool := &GpeekConflictCheckTool{}

	if tool.Name() != "gpeek_conflict_check" {
		t.Errorf("expected name 'gpeek_conflict_check', got %s", tool.Name())
	}

	if tool.Permission() != PermissionRead {
		t.Errorf("expected permission Read, got %v", tool.Permission())
	}

	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}

	expectedProps := []string{"path", "branch", "into"}
	for _, prop := range expectedProps {
		if _, ok := props[prop]; !ok {
			t.Errorf("expected '%s' in schema properties", prop)
		}
	}

	// Check required fields
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("expected required in schema")
	}
	if len(required) != 1 || required[0] != "branch" {
		t.Error("expected 'branch' to be required")
	}

	// Test missing required field
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing branch parameter")
	}
}

func TestGpeekFormatFunctions(t *testing.T) {
	// Test formatStatusResponse
	statusJSON := `{
		"repository": {"name": "test", "path": "/test", "branch": "main"},
		"staged": [],
		"unstaged": [],
		"untracked": [],
		"summary": {"staged_count": 0, "unstaged_count": 0, "untracked_count": 0, "is_clean": true, "has_conflicts": false}
	}`
	result, err := formatStatusResponse([]byte(statusJSON))
	if err != nil {
		t.Fatalf("formatStatusResponse failed: %v", err)
	}
	if !strings.Contains(result, "test") || !strings.Contains(result, "main") {
		t.Errorf("expected result to contain repo name and branch, got %q", result)
	}
	if !strings.Contains(result, "clean") {
		t.Errorf("expected result to indicate clean working tree, got %q", result)
	}

	// Test formatLogResponse
	logJSON := `{
		"commits": [
			{"hash": "abc123def", "short_hash": "abc123d", "message": "test commit", "author": "Test", "email": "test@test.com", "time_ago": "1 hour ago", "is_merge": false}
		],
		"total": 1
	}`
	result, err = formatLogResponse([]byte(logJSON))
	if err != nil {
		t.Fatalf("formatLogResponse failed: %v", err)
	}
	if !strings.Contains(result, "abc123d") || !strings.Contains(result, "test commit") {
		t.Errorf("expected result to contain commit info, got %q", result)
	}

	// Test formatLogResponse with empty commits
	emptyLogJSON := `{"commits": [], "total": 0}`
	result, err = formatLogResponse([]byte(emptyLogJSON))
	if err != nil {
		t.Fatalf("formatLogResponse failed: %v", err)
	}
	if !strings.Contains(result, "No commits") {
		t.Errorf("expected 'No commits found', got %q", result)
	}

	// Test formatDiffResponse with no changes
	emptyDiffJSON := `{"files": [], "stats": {"files_changed": 0, "additions": 0, "deletions": 0}}`
	result, err = formatDiffResponse([]byte(emptyDiffJSON))
	if err != nil {
		t.Fatalf("formatDiffResponse failed: %v", err)
	}
	if !strings.Contains(result, "No changes") {
		t.Errorf("expected 'No changes', got %q", result)
	}

	// Test formatStashesResponse with no stashes
	emptyStashesJSON := `{"stashes": [], "total": 0}`
	result, err = formatStashesResponse([]byte(emptyStashesJSON))
	if err != nil {
		t.Fatalf("formatStashesResponse failed: %v", err)
	}
	if !strings.Contains(result, "No stashes") {
		t.Errorf("expected 'No stashes found', got %q", result)
	}

	// Test formatTagsResponse with no tags
	emptyTagsJSON := `{"tags": [], "total": 0}`
	result, err = formatTagsResponse([]byte(emptyTagsJSON))
	if err != nil {
		t.Fatalf("formatTagsResponse failed: %v", err)
	}
	if !strings.Contains(result, "No tags") {
		t.Errorf("expected 'No tags found', got %q", result)
	}
}

func TestReadFileTool_MaxTokensParam(t *testing.T) {
	tool := &ReadFileTool{}

	// Verify schema has max_tokens parameter
	schema := tool.InputSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}

	if _, ok := props["max_tokens"]; !ok {
		t.Error("expected 'max_tokens' in schema properties")
	}
}

func TestReadFileTool_ChunkingLargeFile(t *testing.T) {
	tool := &ReadFileTool{}

	// Create a large temp file and chdir into it for path validation
	dir := t.TempDir()
	chdirTemp(t, dir)
	testFile := filepath.Join(dir, "large.txt")

	// Create content with many lines (to exceed default token limit)
	var sb strings.Builder
	for i := 0; i < 500; i++ {
		sb.WriteString("This is a line of code that is reasonably long to simulate real files.\n")
	}
	content := sb.String()

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test with default max_tokens (should chunk)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Result should be chunked (shorter than original)
	if len(result) >= len(content) {
		t.Errorf("expected chunked result to be shorter than original (%d vs %d)", len(result), len(content))
	}

	// Should contain truncation indicator
	if !strings.Contains(result, "CHUNKED") && !strings.Contains(result, "TRUNCATED") {
		t.Error("expected chunked result to contain truncation indicator")
	}
}

func TestReadFileTool_UnlimitedMaxTokens(t *testing.T) {
	tool := &ReadFileTool{}

	// Create a temp file and chdir into it for path validation
	dir := t.TempDir()
	chdirTemp(t, dir)
	testFile := filepath.Join(dir, "test.txt")

	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("Line of content.\n")
	}
	content := sb.String()

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test with max_tokens=0 (unlimited)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":       testFile,
		"max_tokens": float64(0),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Result should be the full content
	if result != content {
		t.Error("expected full content when max_tokens=0")
	}
}

func TestReadFileTool_CustomMaxTokens(t *testing.T) {
	tool := &ReadFileTool{}

	// Create a large temp file and chdir into it for path validation
	dir := t.TempDir()
	chdirTemp(t, dir)
	testFile := filepath.Join(dir, "large.txt")

	var sb strings.Builder
	for i := 0; i < 500; i++ {
		sb.WriteString("Line of content here.\n")
	}
	content := sb.String()

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test with high max_tokens (should not chunk)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":       testFile,
		"max_tokens": float64(100000),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Result should be full content
	if result != content {
		t.Error("expected full content with high max_tokens")
	}
}

func TestReadFileTool_SmallFileNoChunking(t *testing.T) {
	tool := &ReadFileTool{}

	// Create a small temp file and chdir into it for path validation
	dir := t.TempDir()
	chdirTemp(t, dir)
	testFile := filepath.Join(dir, "small.txt")
	content := "This is a small file.\n"

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Small files should not be chunked
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result != content {
		t.Errorf("expected small file to not be chunked, got %q", result)
	}
}
