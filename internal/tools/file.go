package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileTool reads file contents
type ReadFileTool struct{}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file. Use this to examine code, configuration files, or any text file. Large files are automatically chunked to save tokens."
}

func (t *ReadFileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The path to the file to read (relative or absolute).",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "Optional: Start reading from this line number (1-indexed).",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "Optional: Stop reading at this line number (inclusive).",
			},
			"max_tokens": map[string]any{
				"type":        "integer",
				"description": "Optional: Maximum tokens to return (default: 2000, set to 0 for unlimited). Large files are chunked with a summary.",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Optional: 'signatures' to return only function/type signatures (Go files only). More efficient for understanding API surface.",
				"enum":        []string{"full", "signatures"},
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Permission() PermissionLevel {
	return PermissionRead
}

// DefaultMaxFileTokens is the default token limit for file reads (roughly 8000 chars)
const DefaultMaxFileTokens = 2000

// ChunkPreviewLines is the number of lines to show in chunked preview
const ChunkPreviewLines = 50

func (t *ReadFileTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Resolve path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Validate path is within project directory
	if err := ValidatePath(absPath); err != nil {
		return "", err
	}

	// Use Lstat to detect symlinks before reading
	info, err := os.Lstat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("cannot access file: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Check for signatures mode (Go files only)
	if contextMode, ok := input["context"].(string); ok && contextMode == "signatures" {
		if strings.HasSuffix(absPath, ".go") {
			astTool := &ASTTool{}
			return astTool.Execute(ctx, map[string]any{
				"path":    absPath,
				"include": []any{"functions", "types"},
			})
		}
		// Non-Go files: fall through to normal read
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Get max tokens limit (default: 2000)
	maxTokens := DefaultMaxFileTokens
	if mt, ok := input["max_tokens"].(float64); ok && mt > 0 {
		maxTokens = int(mt)
	} else if mt, ok := input["max_tokens"].(float64); ok && mt == 0 {
		maxTokens = 0 // Unlimited
	}

	// Handle line range if specified
	startLine, hasStart := input["start_line"].(float64)
	endLine, hasEnd := input["end_line"].(float64)

	if hasStart || hasEnd {
		lines := strings.Split(string(content), "\n")
		start := 0
		end := len(lines)

		if hasStart && int(startLine) > 0 {
			start = int(startLine) - 1
		}
		if hasEnd && int(endLine) > 0 && int(endLine) <= len(lines) {
			end = int(endLine)
		}

		if start >= end || start >= len(lines) {
			return "", fmt.Errorf("invalid line range")
		}

		// Format with line numbers
		var sb strings.Builder
		for i := start; i < end && i < len(lines); i++ {
			fmt.Fprintf(&sb, "%4d | %s\n", i+1, lines[i])
		}
		return sb.String(), nil
	}

	// Apply token-based chunking if file is large
	if maxTokens > 0 {
		// Estimate tokens: ~4 chars per token
		estimatedTokens := len(content) / 4
		if estimatedTokens > maxTokens {
			return t.chunkFile(string(content), path, maxTokens)
		}
	}

	return string(content), nil
}

// chunkFile returns a chunked preview of a large file with summary
func (t *ReadFileTool) chunkFile(content string, path string, maxTokens int) (string, error) {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// Calculate how many lines we can show (roughly maxTokens * 4 chars / avg line length)
	avgLineLen := len(content) / max(totalLines, 1)
	if avgLineLen == 0 {
		avgLineLen = 40 // Default average line length
	}
	maxChars := maxTokens * 4
	maxLines := min(max(maxChars/avgLineLen, ChunkPreviewLines), totalLines)

	var sb strings.Builder

	// Write header
	fmt.Fprintf(&sb, "=== File: %s ===\n", path)
	fmt.Fprintf(&sb, "=== CHUNKED: Showing first %d of %d lines (file too large) ===\n\n", maxLines, totalLines)

	// Write first N lines with line numbers
	for i := 0; i < maxLines && i < totalLines; i++ {
		fmt.Fprintf(&sb, "%4d | %s\n", i+1, lines[i])
	}

	// Write summary footer
	remainingLines := totalLines - maxLines
	if remainingLines > 0 {
		sb.WriteString("\n=== TRUNCATED ===\n")
		fmt.Fprintf(&sb, "Remaining: %d lines not shown\n", remainingLines)
		sb.WriteString("Use start_line/end_line to read specific sections, or max_tokens=0 for full file\n")
	}

	return sb.String(), nil
}

// WriteFileTool writes content to a file
type WriteFileTool struct{}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Write content to a file. Creates the file if it doesn't exist, or overwrites if it does. Creates parent directories as needed."
}

func (t *WriteFileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The path to the file to write (relative or absolute).",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The content to write to the file.",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFileTool) Permission() PermissionLevel {
	return PermissionWrite
}

func (t *WriteFileTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	content, ok := input["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	// Resolve path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Validate path is within project directory (with symlink write protection)
	if err := ValidatePathForWrite(absPath); err != nil {
		return "", err
	}

	// Create parent directories
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}

	// Write file using openNoFollow to prevent symlink race attacks
	f, err := openNoFollow(absPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		// Fallback for new files where O_NOFOLLOW may fail on some systems
		if os.IsNotExist(err) || os.IsPermission(err) {
			if writeErr := os.WriteFile(absPath, []byte(content), 0644); writeErr != nil {
				return "", fmt.Errorf("failed to write file: %w", writeErr)
			}
			return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
		}
		return "", fmt.Errorf("failed to open file for writing: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
}

// EditFileTool performs targeted edits on a file
type EditFileTool struct{}

func (t *EditFileTool) Name() string {
	return "edit_file"
}

func (t *EditFileTool) Description() string {
	return "Edit a file by replacing specific text. Use this for targeted modifications instead of rewriting entire files."
}

func (t *EditFileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The path to the file to edit.",
			},
			"old_text": map[string]any{
				"type":        "string",
				"description": "The exact text to find and replace.",
			},
			"new_text": map[string]any{
				"type":        "string",
				"description": "The text to replace it with.",
			},
		},
		"required": []string{"path", "old_text", "new_text"},
	}
}

func (t *EditFileTool) Permission() PermissionLevel {
	return PermissionWrite
}

func (t *EditFileTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	oldText, ok := input["old_text"].(string)
	if !ok {
		return "", fmt.Errorf("old_text is required")
	}

	newText, ok := input["new_text"].(string)
	if !ok {
		return "", fmt.Errorf("new_text is required")
	}

	// Resolve path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Validate path is within project directory (with symlink write protection)
	if err := ValidatePathForWrite(absPath); err != nil {
		return "", err
	}

	// Read current content (use Lstat first to check for symlinks)
	info, err := os.Lstat(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("access denied: refusing to edit through symlink %q", path)
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Check if old_text exists
	if !strings.Contains(string(content), oldText) {
		return "", fmt.Errorf("old_text not found in file")
	}

	// Count occurrences
	count := strings.Count(string(content), oldText)
	if count > 1 {
		return "", fmt.Errorf("old_text found %d times, must be unique; provide more context", count)
	}

	// Replace
	oldContent := string(content)
	newContent := strings.Replace(oldContent, oldText, newText, 1)

	// Write back
	if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Generate unified diff for TUI visualization
	diff := GenerateUnifiedDiff(path, oldContent, newContent, 3)
	if diff != "" {
		return fmt.Sprintf("Successfully edited %s\n\n%s", path, diff), nil
	}

	return fmt.Sprintf("Successfully edited %s", path), nil
}

// ListFilesTool lists files in a directory
type ListFilesTool struct{}

func (t *ListFilesTool) Name() string {
	return "list_files"
}

func (t *ListFilesTool) Description() string {
	return "List files and directories at a given path. Useful for exploring project structure."
}

func (t *ListFilesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The directory path to list (defaults to current directory).",
				"default":     ".",
			},
			"recursive": map[string]any{
				"type":        "boolean",
				"description": "If true, list files recursively.",
				"default":     false,
			},
			"pattern": map[string]any{
				"type":        "string",
				"description": "Glob pattern to filter files (e.g., '*.go', '**/*.ts').",
			},
		},
	}
}

func (t *ListFilesTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *ListFilesTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	path := "."
	if p, ok := input["path"].(string); ok && p != "" {
		path = p
	}

	recursive := false
	if r, ok := input["recursive"].(bool); ok {
		recursive = r
	}

	pattern := ""
	if p, ok := input["pattern"].(string); ok {
		pattern = p
	}

	// Resolve path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("path not found: %s", path)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", path)
	}

	var files []string

	if recursive {
		_ = filepath.Walk(absPath, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			// Skip hidden directories
			if info.IsDir() && strings.HasPrefix(info.Name(), ".") && p != absPath {
				return filepath.SkipDir
			}

			rel, _ := filepath.Rel(absPath, p)
			if rel == "." {
				return nil
			}

			if pattern != "" {
				matched, _ := filepath.Match(pattern, info.Name())
				if !matched {
					return nil
				}
			}

			prefix := "  "
			if info.IsDir() {
				prefix = "📁"
			}
			files = append(files, fmt.Sprintf("%s %s", prefix, rel))
			return nil
		})
	} else {
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return "", fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range entries {
			name := entry.Name()
			if pattern != "" {
				matched, _ := filepath.Match(pattern, name)
				if !matched {
					continue
				}
			}

			prefix := "  "
			if entry.IsDir() {
				prefix = "📁"
			}
			files = append(files, fmt.Sprintf("%s %s", prefix, name))
		}
	}

	if len(files) == 0 {
		return "No files found.", nil
	}

	return strings.Join(files, "\n"), nil
}
