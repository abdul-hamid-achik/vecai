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
	return "Read the contents of a file. Use this to examine code, configuration files, or any text file."
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
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Permission() PermissionLevel {
	return PermissionRead
}

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

	// Check if file exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("cannot access file: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
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
			sb.WriteString(fmt.Sprintf("%4d | %s\n", i+1, lines[i]))
		}
		return sb.String(), nil
	}

	return string(content), nil
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

	// Create parent directories
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}

	// Write file
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
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

	// Read current content
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
	newContent := strings.Replace(string(content), oldText, newText, 1)

	// Write back
	if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
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
