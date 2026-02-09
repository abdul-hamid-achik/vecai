package tools

import (
	"fmt"
	"strings"
	"time"

	"github.com/pmezard/go-difflib/difflib"
)

// GenerateUnifiedDiff produces a unified diff between old and new content.
// contextLines controls how many surrounding lines are shown (default 3).
func GenerateUnifiedDiff(filename, oldContent, newContent string, contextLines int) string {
	if contextLines <= 0 {
		contextLines = 3
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldContent),
		B:        difflib.SplitLines(newContent),
		FromFile: "a/" + filename,
		ToFile:   "b/" + filename,
		FromDate: time.Now().Format(time.RFC3339),
		ToDate:   time.Now().Format(time.RFC3339),
		Context:  contextLines,
	}

	result, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return fmt.Sprintf("(diff generation failed: %v)", err)
	}

	if result == "" {
		return "" // No changes
	}

	// Count additions and deletions for a summary line
	adds, dels := 0, 0
	for _, line := range strings.Split(result, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			adds++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			dels++
		}
	}

	return fmt.Sprintf("%s\n%d insertion(s), %d deletion(s)", result, adds, dels)
}
