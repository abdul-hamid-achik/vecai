package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TaggedFile represents a file tagged by the user with @
type TaggedFile struct {
	RelPath  string // Relative path from project root
	AbsPath  string // Absolute path for content loading
	Language string // Detected language
}

// TagParseResult is the result of parsing @ mentions from user input
type TagParseResult struct {
	CleanQuery string       // Input with @mentions removed
	NewTags    []TaggedFile // Newly tagged files
}

// atMentionRegex matches @path/to/file.ext patterns.
// Must be preceded by start-of-string or whitespace.
// The path must contain at least one dot (to distinguish from @username-style text).
// No trailing \s required — the \w+ extension stops at non-word chars naturally.
var atMentionRegex = regexp.MustCompile(`(?:^|\s)@([\w./\-]+\.\w+)`)

// ParseFileTags extracts @file mentions from user input.
// Returns clean query (mentions removed) and list of tagged files.
func ParseFileTags(input string, projectRoot string) TagParseResult {
	matches := atMentionRegex.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return TagParseResult{CleanQuery: strings.TrimSpace(input)}
	}

	var tags []TaggedFile
	clean := input

	// Process matches in reverse order so indices stay valid when removing
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		// m[2], m[3] is the capture group (file path)
		relPath := input[m[2]:m[3]]
		lang := langFromExt("." + extFromPath(relPath))

		tags = append(tags, TaggedFile{
			RelPath:  relPath,
			Language: lang,
		})

		// Remove the @mention from clean query (keep any leading space)
		start := m[0]
		end := m[1]
		// If match starts with whitespace, keep the space
		if input[start] == ' ' || input[start] == '\t' {
			start++
		}
		clean = clean[:start] + clean[end:]
	}

	// Reverse tags to maintain input order
	for i, j := 0, len(tags)-1; i < j; i, j = i+1, j-1 {
		tags[i], tags[j] = tags[j], tags[i]
	}

	// Collapse multiple spaces left by mention removal
	cleanQuery := strings.TrimSpace(clean)
	for strings.Contains(cleanQuery, "  ") {
		cleanQuery = strings.ReplaceAll(cleanQuery, "  ", " ")
	}

	return TagParseResult{
		CleanQuery: cleanQuery,
		NewTags:    tags,
	}
}

// extFromPath extracts the file extension without the dot
func extFromPath(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i+1:]
		}
		if path[i] == '/' {
			return ""
		}
	}
	return ""
}

// renderFileTagChips renders the tagged file chips for the status bar.
// Returns empty string if no files are tagged.
func renderFileTagChips(files []TaggedFile, maxWidth int) string {
	if len(files) == 0 {
		return ""
	}

	label := fileTagLabelStyle.Render("Files:")
	var chips []string
	totalWidth := lipgloss.Width(label) + 1 // +1 for space after label

	for i, f := range files {
		name := shortName(f.RelPath)
		chip := fileTagChipStyle.Render(name + " " + fileTagRemoveStyle.Render("\u2717"))
		chipWidth := lipgloss.Width(chip) + 1 // +1 for separator space

		// Check if we have room
		if totalWidth+chipWidth > maxWidth && i > 0 {
			remaining := len(files) - i
			more := fileTagLabelStyle.Render("+" + strings.Repeat("", 0) + itoa(remaining) + " more")
			chips = append(chips, more)
			break
		}

		totalWidth += chipWidth
		chips = append(chips, chip)
	}

	return label + " " + strings.Join(chips, " ")
}

// shortName returns a short display name for a file path
func shortName(relPath string) string {
	base := relPath
	if idx := strings.LastIndex(relPath, "/"); idx >= 0 {
		base = relPath[idx+1:]
	}
	return base
}

// itoa converts an int to string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	digits := make([]byte, 0, 5)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// Reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

// renderSuggestHint renders a compact hint showing proactively suggested files.
// Example: "Related: router.go, pipeline.go — type @ to add"
func renderSuggestHint(suggestions []SuggestedFile, maxWidth int) string {
	if len(suggestions) == 0 || maxWidth < 20 {
		return ""
	}

	prefix := suggestLabelStyle.Render("Related:")
	suffix := suggestHintStyle.Render(" — type @ to add")
	prefixW := lipgloss.Width(prefix) + 1 // +1 for space
	suffixW := lipgloss.Width(suffix)
	available := maxWidth - prefixW - suffixW

	if available < 8 {
		return ""
	}

	var names []string
	used := 0
	for _, s := range suggestions {
		name := shortName(s.RelPath)
		nameW := len(name) + 2 // +2 for ", " separator
		if used+nameW > available && len(names) > 0 {
			break
		}
		names = append(names, suggestFileStyle.Render(name))
		used += nameW
	}

	if len(names) == 0 {
		return ""
	}

	return prefix + " " + strings.Join(names, suggestHintStyle.Render(", ")) + suffix
}

// File tag styles (Nord-themed)
var (
	fileTagChipStyle = lipgloss.NewStyle().
				Background(colorBgElevate).
				Foreground(colorAccent).
				Padding(0, 1)

	fileTagRemoveStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	fileTagLabelStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	// Suggest hint styles
	suggestLabelStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	suggestFileStyle = lipgloss.NewStyle().
				Foreground(colorAccent2)

	suggestHintStyle = lipgloss.NewStyle().
				Foreground(colorDim)
)
