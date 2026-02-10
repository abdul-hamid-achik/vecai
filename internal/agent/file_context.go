package agent

import (
	"fmt"
	"os"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/tui"
)

// maxFileContextChars is the maximum total characters of tagged file content to inject.
// This prevents blowing the context window with very large files.
const maxFileContextChars = 20000

// maxSingleFileChars is the maximum characters for a single tagged file.
const maxSingleFileChars = 8000

// formatTaggedFileContext reads the content of tagged files and formats them
// as a context block suitable for injection into the LLM conversation.
// Returns empty string if no files have content or all fail to read.
func formatTaggedFileContext(files []tui.TaggedFile) string {
	if len(files) == 0 {
		return ""
	}

	var b strings.Builder
	totalChars := 0

	for _, f := range files {
		path := f.AbsPath
		if path == "" {
			path = f.RelPath
		}

		content, err := os.ReadFile(path)
		if err != nil {
			logDebug("file_context: failed to read %s: %v", path, err)
			continue
		}

		text := string(content)
		if len(text) == 0 {
			continue
		}

		// Truncate single large files
		if len(text) > maxSingleFileChars {
			text = text[:maxSingleFileChars] + "\n... (truncated)"
		}

		// Check total budget
		snippet := formatFileSnippet(f, text)
		if totalChars+len(snippet) > maxFileContextChars {
			remaining := len(files) - b.Len() // approximate
			if remaining > 0 {
				b.WriteString(fmt.Sprintf("\n(+%d more files omitted due to context budget)\n", remaining))
			}
			break
		}

		b.WriteString(snippet)
		totalChars += len(snippet)
	}

	if b.Len() == 0 {
		return ""
	}

	return b.String()
}

// formatFileSnippet formats a single file's content as a code block with metadata
func formatFileSnippet(f tui.TaggedFile, content string) string {
	var b strings.Builder

	// Header with path and language
	lang := strings.ToLower(f.Language)
	b.WriteString(fmt.Sprintf("### %s", f.RelPath))
	if f.Language != "" {
		b.WriteString(fmt.Sprintf(" (%s)", f.Language))
	}
	b.WriteString("\n")

	// Code fence with language for syntax context
	if lang == "" {
		lang = ""
	}
	b.WriteString(fmt.Sprintf("```%s\n", lang))
	b.WriteString(strings.TrimRight(content, "\n"))
	b.WriteString("\n```\n\n")

	return b.String()
}
