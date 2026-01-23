package highlight

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Highlighter provides syntax highlighting for code blocks
type Highlighter struct {
	enabled   bool
	formatter chroma.Formatter
	style     *chroma.Style
}

// New creates a new Highlighter
func New(enabled bool) *Highlighter {
	return &Highlighter{
		enabled:   enabled,
		formatter: formatters.Get("terminal256"),
		style:     styles.Get("monokai"),
	}
}

// Highlight applies syntax highlighting to a code string
func (h *Highlighter) Highlight(code, language string) string {
	if !h.enabled {
		return code
	}

	// Get lexer for the language
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// Tokenize the code
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	// Format with colors
	var buf bytes.Buffer
	err = h.formatter.Format(&buf, h.style, iterator)
	if err != nil {
		return code
	}

	return buf.String()
}

// codeBlockRegex matches markdown code blocks with optional language
var codeBlockRegex = regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```")

// HighlightMarkdownCodeBlocks finds and highlights markdown code blocks in text
func (h *Highlighter) HighlightMarkdownCodeBlocks(text string) string {
	if !h.enabled {
		return text
	}

	return codeBlockRegex.ReplaceAllStringFunc(text, func(match string) string {
		// Extract language and code from the match
		parts := codeBlockRegex.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		language := parts[1]
		code := parts[2]

		// Remove trailing newline from code if present
		code = strings.TrimSuffix(code, "\n")

		// Highlight the code
		highlighted := h.Highlight(code, language)

		// Return with the code block markers removed (just highlighted code)
		return highlighted
	})
}
