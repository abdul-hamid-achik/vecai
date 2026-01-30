package tui

import (
	"regexp"
	"strings"
	"testing"
)

// stripANSI removes ANSI escape codes from a string for easier testing
func stripANSI(s string) string {
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansiRegex.ReplaceAllString(s, "")
}

// TestEnumerationRendering verifies that numbered lists render correctly
// without the %d format bug (numbers should appear as "1. ", "2. ", not "%d.")
func TestEnumerationRendering(t *testing.T) {
	// Test markdown with numbered list
	markdown := `Here is a list:

1. First item
2. Second item
3. Third item
`

	rendered := renderMarkdown(markdown)
	plain := stripANSI(rendered)

	// Should NOT contain literal "%d"
	if strings.Contains(plain, "%d") {
		t.Errorf("Rendered markdown contains literal '%%d', indicating format bug:\n%s", plain)
	}

	// Should contain actual numbers followed by period
	if !strings.Contains(plain, "1.") {
		t.Errorf("Rendered markdown missing '1.':\n%s", plain)
	}
	if !strings.Contains(plain, "2.") {
		t.Errorf("Rendered markdown missing '2.':\n%s", plain)
	}
	if !strings.Contains(plain, "3.") {
		t.Errorf("Rendered markdown missing '3.':\n%s", plain)
	}

	// Should contain the list items
	if !strings.Contains(plain, "First item") {
		t.Errorf("Rendered markdown missing 'First item':\n%s", plain)
	}
	if !strings.Contains(plain, "Second item") {
		t.Errorf("Rendered markdown missing 'Second item':\n%s", plain)
	}
	if !strings.Contains(plain, "Third item") {
		t.Errorf("Rendered markdown missing 'Third item':\n%s", plain)
	}
}

// TestUnorderedListRendering verifies that bullet lists render correctly
func TestUnorderedListRendering(t *testing.T) {
	markdown := `Here is a bullet list:

- Item A
- Item B
- Item C
`

	rendered := renderMarkdown(markdown)
	plain := stripANSI(rendered)

	// Should contain the list items
	if !strings.Contains(plain, "Item A") {
		t.Errorf("Rendered markdown missing 'Item A':\n%s", plain)
	}
	if !strings.Contains(plain, "Item B") {
		t.Errorf("Rendered markdown missing 'Item B':\n%s", plain)
	}
	if !strings.Contains(plain, "Item C") {
		t.Errorf("Rendered markdown missing 'Item C':\n%s", plain)
	}
}

// TestNestedEnumerationRendering verifies nested numbered lists work
func TestNestedEnumerationRendering(t *testing.T) {
	markdown := `Nested list:

1. First
   1. Nested first
   2. Nested second
2. Second
`

	rendered := renderMarkdown(markdown)
	plain := stripANSI(rendered)

	// Should NOT contain literal "%d"
	if strings.Contains(plain, "%d") {
		t.Errorf("Nested list contains literal '%%d':\n%s", plain)
	}

	// Should contain both outer and inner items
	if !strings.Contains(plain, "First") {
		t.Errorf("Missing 'First' in nested list:\n%s", plain)
	}
	if !strings.Contains(plain, "Nested first") {
		t.Errorf("Missing 'Nested first' in nested list:\n%s", plain)
	}
}

// TestMixedListRendering verifies mixed list types render correctly
func TestMixedListRendering(t *testing.T) {
	markdown := `Mixed content:

1. Numbered one
2. Numbered two

- Bullet one
- Bullet two

3. Continue numbered
`

	rendered := renderMarkdown(markdown)
	plain := stripANSI(rendered)

	// Should NOT contain literal "%d"
	if strings.Contains(plain, "%d") {
		t.Errorf("Mixed list contains literal '%%d':\n%s", plain)
	}
}
