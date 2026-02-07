package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"
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

// --- Tool Visualization Rendering Tests ---

func TestRenderToolCallBlock_CategoryIcons(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	tests := []struct {
		name     string
		toolName string
		toolType ToolCategory
		wantIcon string
		wantLabel string
	}{
		{"read tool", "read_file", ToolCategoryRead, iconToolRead, "[READ]"},
		{"write tool", "write_file", ToolCategoryWrite, iconToolWrite, "[WRITE]"},
		{"exec tool", "bash", ToolCategoryExecute, iconToolExec, "[EXEC]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := ContentBlock{
				Type:     BlockToolCall,
				ToolName: tt.toolName,
				Content:  "description",
				ToolMeta: &ToolBlockMeta{
					ToolType:  tt.toolType,
					IsRunning: false,
					GroupID:   "test-1",
				},
			}
			rendered := model.renderToolCallBlock(block)
			plain := stripANSI(rendered)

			if !strings.Contains(plain, tt.wantIcon) {
				t.Errorf("Expected icon %q in rendered output %q", tt.wantIcon, plain)
			}
			if !strings.Contains(plain, tt.wantLabel) {
				t.Errorf("Expected label %q in rendered output %q", tt.wantLabel, plain)
			}
			if !strings.Contains(plain, tt.toolName) {
				t.Errorf("Expected tool name %q in rendered output %q", tt.toolName, plain)
			}
		})
	}
}

func TestRenderToolCallBlock_SpinnerWhenRunning(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolCall,
		ToolName: "bash",
		Content:  "running command",
		ToolMeta: &ToolBlockMeta{
			ToolType:  ToolCategoryExecute,
			IsRunning: true,
			GroupID:   "test-1",
		},
	}
	rendered := model.renderToolCallBlock(block)
	plain := stripANSI(rendered)

	// Should show spinner frame instead of category icon
	spinnerFrame := GetSpinnerFrame(model.spinnerFrame)
	if !strings.Contains(plain, spinnerFrame) {
		t.Errorf("Expected spinner frame %q in rendered output %q", spinnerFrame, plain)
	}
}

func TestRenderToolCallBlock_NoMeta(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolCall,
		ToolName: "some_tool",
		Content:  "desc",
		ToolMeta: nil,
	}
	rendered := model.renderToolCallBlock(block)
	plain := stripANSI(rendered)

	// Should fall back to default icon
	if !strings.Contains(plain, iconToolCall) {
		t.Errorf("Expected default icon %q in rendered output %q", iconToolCall, plain)
	}
	if !strings.Contains(plain, "some_tool") {
		t.Errorf("Expected tool name in rendered output %q", plain)
	}
}

func TestRenderToolResultBlock_WithElapsedTime(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolResult,
		ToolName: "read_file",
		Content:  "file contents here",
		IsError:  false,
		ToolMeta: &ToolBlockMeta{
			ToolType: ToolCategoryRead,
			Elapsed:  1500 * time.Millisecond,
			GroupID:  "test-1",
		},
	}
	rendered := model.renderToolResultBlock(block)
	plain := stripANSI(rendered)

	if !strings.Contains(plain, iconSuccess) {
		t.Errorf("Expected success icon in rendered output %q", plain)
	}
	if !strings.Contains(plain, "read_file") {
		t.Errorf("Expected tool name in rendered output %q", plain)
	}
	if !strings.Contains(plain, "1.5s") {
		t.Errorf("Expected elapsed time '1.5s' in rendered output %q", plain)
	}
}

func TestRenderToolResultBlock_Error(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolResult,
		ToolName: "bash",
		Content:  "command not found",
		IsError:  true,
	}
	rendered := model.renderToolResultBlock(block)
	plain := stripANSI(rendered)

	if !strings.Contains(plain, iconError) {
		t.Errorf("Expected error icon in rendered output %q", plain)
	}
	if !strings.Contains(plain, "command not found") {
		t.Errorf("Expected error content in rendered output %q", plain)
	}
}

func TestRenderToolResultBlock_TruncationIndicator(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolResult,
		ToolName: "read_file",
		Content:  "truncated content...",
		IsError:  false,
		ToolMeta: &ToolBlockMeta{
			ToolType:  ToolCategoryRead,
			Elapsed:   200 * time.Millisecond,
			ResultLen: 2450, // Original was > 500
		},
	}
	rendered := model.renderToolResultBlock(block)
	plain := stripANSI(rendered)

	// Should show truncation indicator with byte count
	if !strings.Contains(plain, "2.4 KB") {
		t.Errorf("Expected truncation indicator with byte count in rendered output %q", plain)
	}
	if !strings.Contains(plain, "showing first 500") {
		t.Errorf("Expected 'showing first 500' in rendered output %q", plain)
	}
}

func TestRenderToolResultBlock_NoOutput(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolResult,
		ToolName: "write_file",
		Content:  "(no output)",
		IsError:  false,
		ToolMeta: &ToolBlockMeta{
			ToolType: ToolCategoryWrite,
			Elapsed:  50 * time.Millisecond,
		},
	}
	rendered := model.renderToolResultBlock(block)
	plain := stripANSI(rendered)

	// Should show tool name and time but NOT the "(no output)" content
	if !strings.Contains(plain, "write_file") {
		t.Errorf("Expected tool name in rendered output %q", plain)
	}
	// Content "(no output)" should be suppressed
	if strings.Contains(plain, "(no output)") {
		t.Errorf("'(no output)' should be suppressed in rendered output %q", plain)
	}
}

// --- Helper Function Tests ---

func TestTruncateUTF8Safe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxBytes int
		want     string
	}{
		{"short string", "short", 100, "short"},
		{"truncate mid-word", "hello world", 8, "hello..."},
		{"empty string", "", 10, ""},
		{"fits exactly", "ab", 3, "ab"},
		{"exact length", "abcdef", 6, "abcdef"},
		{"one over", "abcdefgh", 6, "abc..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateUTF8Safe(tt.input, tt.maxBytes)
			if got != tt.want {
				t.Errorf("truncateUTF8Safe(%q, %d) = %q, want %q", tt.input, tt.maxBytes, got, tt.want)
			}
			if len(got) > tt.maxBytes {
				t.Errorf("truncateUTF8Safe result exceeds maxBytes: len(%q) = %d > %d", got, len(got), tt.maxBytes)
			}
		})
	}
}
