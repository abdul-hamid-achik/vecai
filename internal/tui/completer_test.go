package tui

import (
	"strings"
	"testing"
)

func TestCompleterNewCompleter(t *testing.T) {
	c := NewCompleter()
	if c == nil {
		t.Fatal("Expected non-nil completer")
	}
	if len(c.allCommands) == 0 {
		t.Error("Expected built-in commands to be populated")
	}
	if c.maxVisible != 8 {
		t.Errorf("Expected maxVisible 8, got %d", c.maxVisible)
	}
	if c.IsActive() {
		t.Error("Expected completer to be inactive initially")
	}
}

func TestCompleterAddCommands(t *testing.T) {
	c := NewCompleter()
	initialCount := len(c.allCommands)

	c.AddCommands([]CommandDef{
		{Name: "/custom", Description: "Custom command"},
	})

	if len(c.allCommands) != initialCount+1 {
		t.Errorf("Expected %d commands after add, got %d", initialCount+1, len(c.allCommands))
	}
}

func TestCompleterActivatesOnSlash(t *testing.T) {
	c := NewCompleter()

	// Should activate on "/"
	c.Update("/")
	if !c.IsActive() {
		t.Error("Expected completer to be active after '/'")
	}
	if len(c.filtered) == 0 {
		t.Error("Expected filtered results for '/'")
	}
}

func TestCompleterFiltersPrefix(t *testing.T) {
	c := NewCompleter()

	// Filter to /he -> /help
	c.Update("/he")
	if !c.IsActive() {
		t.Error("Expected completer to be active for '/he'")
	}
	for _, cmd := range c.filtered {
		if !strings.HasPrefix(strings.ToLower(cmd.Name), "/he") {
			t.Errorf("Filtered command %q doesn't match prefix '/he'", cmd.Name)
		}
	}
}

func TestCompleterCaseInsensitive(t *testing.T) {
	c := NewCompleter()

	c.Update("/HE")
	if !c.IsActive() {
		t.Error("Expected case-insensitive match for '/HE'")
	}
	found := false
	for _, cmd := range c.filtered {
		if cmd.Name == "/help" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected /help in case-insensitive results for '/HE'")
	}
}

func TestCompleterDeactivatesWithSpace(t *testing.T) {
	c := NewCompleter()

	// Activate first
	c.Update("/plan")
	if !c.IsActive() {
		t.Error("Expected active for '/plan'")
	}

	// Space after command deactivates (user is typing args)
	c.Update("/plan ")
	if c.IsActive() {
		t.Error("Expected completer to deactivate when input contains space")
	}
}

func TestCompleterDeactivatesWithoutSlash(t *testing.T) {
	c := NewCompleter()

	c.Update("hello")
	if c.IsActive() {
		t.Error("Expected completer to be inactive for non-slash input")
	}

	c.Update("")
	if c.IsActive() {
		t.Error("Expected completer to be inactive for empty input")
	}
}

func TestCompleterNavigation(t *testing.T) {
	c := NewCompleter()
	c.Update("/")

	if c.selected != 0 {
		t.Errorf("Expected initial selection 0, got %d", c.selected)
	}

	// Move down
	c.MoveDown()
	if c.selected != 1 {
		t.Errorf("Expected selection 1 after MoveDown, got %d", c.selected)
	}

	// Move up back to 0
	c.MoveUp()
	if c.selected != 0 {
		t.Errorf("Expected selection 0 after MoveUp, got %d", c.selected)
	}

	// Move up wraps to last item
	c.MoveUp()
	if c.selected != len(c.filtered)-1 {
		t.Errorf("Expected selection %d (last) after wrapping up, got %d", len(c.filtered)-1, c.selected)
	}

	// Move down wraps to first item
	c.MoveDown()
	if c.selected != 0 {
		t.Errorf("Expected selection 0 after wrapping down, got %d", c.selected)
	}
}

func TestCompleterAccept(t *testing.T) {
	c := NewCompleter()
	c.Update("/he")

	// Find /help in filtered
	accepted := c.Accept()
	if accepted == "" {
		t.Error("Expected non-empty accepted value")
	}
	if !strings.HasPrefix(accepted, "/he") {
		t.Errorf("Expected accepted to start with '/he', got %q", accepted)
	}

	// After accept, completer should be dismissed
	if c.IsActive() {
		t.Error("Expected completer to be dismissed after accept")
	}
}

func TestCompleterAcceptWithArgs(t *testing.T) {
	c := NewCompleter()

	// /plan has HasArgs=true
	c.Update("/plan")
	var planIdx int
	for i, cmd := range c.filtered {
		if cmd.Name == "/plan" {
			planIdx = i
			break
		}
	}
	c.selected = planIdx
	accepted := c.Accept()
	if accepted != "/plan " {
		t.Errorf("Expected '/plan ' (with trailing space) for HasArgs command, got %q", accepted)
	}
}

func TestCompleterAcceptNoArgs(t *testing.T) {
	c := NewCompleter()

	// /help has HasArgs=false
	c.Update("/help")
	accepted := c.Accept()
	if accepted != "/help" {
		t.Errorf("Expected '/help' (no trailing space) for no-args command, got %q", accepted)
	}
}

func TestCompleterDismiss(t *testing.T) {
	c := NewCompleter()
	c.Update("/")
	if !c.IsActive() {
		t.Fatal("Expected active before dismiss")
	}

	c.Dismiss()
	if c.IsActive() {
		t.Error("Expected inactive after dismiss")
	}
	if c.selected != 0 {
		t.Error("Expected selected reset to 0 after dismiss")
	}
	if c.scrollOffset != 0 {
		t.Error("Expected scrollOffset reset to 0 after dismiss")
	}
}

func TestCompleterAcceptEmpty(t *testing.T) {
	c := NewCompleter()
	// Not active
	result := c.Accept()
	if result != "" {
		t.Errorf("Expected empty string from inactive completer, got %q", result)
	}
}

func TestCompleterRender(t *testing.T) {
	c := NewCompleter()

	// Inactive completer renders empty
	if c.Render(80) != "" {
		t.Error("Expected empty render for inactive completer")
	}

	// Active completer renders content
	c.Update("/")
	rendered := c.Render(80)
	if rendered == "" {
		t.Error("Expected non-empty render for active completer")
	}

	// Should contain command names
	if !strings.Contains(rendered, "/help") {
		t.Error("Expected rendered dropdown to contain '/help'")
	}
}

func TestCompleterRenderSelectedHighlight(t *testing.T) {
	c := NewCompleter()
	c.Update("/")

	// Move to second item
	c.MoveDown()
	rendered := c.Render(80)

	// Rendered content should be non-empty (we can't easily check style application in tests)
	if rendered == "" {
		t.Error("Expected non-empty render with selection")
	}
}

func TestCompleterScrolling(t *testing.T) {
	// Create a completer with many commands
	c := &Completer{
		maxVisible: 3,
	}
	for i := 0; i < 10; i++ {
		c.allCommands = append(c.allCommands, CommandDef{
			Name:        strings.Repeat("x", i+1),
			Description: "test",
		})
	}

	// Make them all match by not filtering on /
	c.filtered = c.allCommands
	c.active = true

	if c.VisibleCount() != 3 {
		t.Errorf("Expected VisibleCount 3, got %d", c.VisibleCount())
	}

	// Move down past visible window
	c.MoveDown() // 1
	c.MoveDown() // 2
	c.MoveDown() // 3 -> should trigger scroll
	if c.scrollOffset == 0 {
		t.Error("Expected scrollOffset > 0 after moving past visible window")
	}
}

func TestCompleterNoMatchDeactivates(t *testing.T) {
	c := NewCompleter()
	c.Update("/zzzznonexistent")
	if c.IsActive() {
		t.Error("Expected completer to be inactive when no matches")
	}
}

func TestCompleterNavigationOnInactive(t *testing.T) {
	c := NewCompleter()
	// Should not panic on inactive completer
	c.MoveUp()
	c.MoveDown()
	if c.selected != 0 {
		t.Error("Expected selected to remain 0 on inactive completer")
	}
}

func TestCompleterSelectionResetOnFilter(t *testing.T) {
	c := NewCompleter()
	c.Update("/")
	c.MoveDown()
	c.MoveDown()
	c.MoveDown()

	// Now narrow the filter — selected should reset if out of bounds
	c.Update("/help")
	if c.selected >= len(c.filtered) {
		t.Error("Expected selection to reset within bounds after re-filter")
	}
}
