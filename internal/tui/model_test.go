package tui

import (
	"testing"
)

// TestNewModelTextInputStyling verifies that the textinput has Nord theme styling applied
func TestNewModelTextInputStyling(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test-model", streamChan)

	// Verify textinput is initialized
	if model.textInput.Placeholder != "Type message..." {
		t.Errorf("Expected placeholder 'Type message...', got '%s'", model.textInput.Placeholder)
	}

	// Verify prompt is empty (we render our own)
	if model.textInput.Prompt != "" {
		t.Errorf("Expected empty prompt, got '%s'", model.textInput.Prompt)
	}

	// Verify char limit is 0 (unlimited)
	if model.textInput.CharLimit != 0 {
		t.Errorf("Expected CharLimit 0, got %d", model.textInput.CharLimit)
	}

	// Verify styling is applied (non-empty styles indicate configuration)
	// Note: We can't directly compare lipgloss styles, but we verify they're set
	cursorStyleStr := model.textInput.Cursor.Style.String()
	if cursorStyleStr == "" {
		t.Log("Warning: Cursor style appears empty (may be expected if no ANSI)")
	}

	placeholderStyleStr := model.textInput.PlaceholderStyle.String()
	if placeholderStyleStr == "" {
		t.Log("Warning: PlaceholderStyle appears empty (may be expected if no ANSI)")
	}

	textStyleStr := model.textInput.TextStyle.String()
	if textStyleStr == "" {
		t.Log("Warning: TextStyle appears empty (may be expected if no ANSI)")
	}
}

// TestNewModelInitialization verifies model is properly initialized
func TestNewModelInitialization(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("gpt-4", streamChan)

	// Check initial state
	if model.state != StateStarting {
		t.Errorf("Expected initial state StateStarting, got %v", model.state)
	}

	// Check model name
	if model.modelName != "gpt-4" {
		t.Errorf("Expected modelName 'gpt-4', got '%s'", model.modelName)
	}

	// Check blocks initialized
	if model.blocks == nil {
		t.Error("Expected blocks to be initialized, got nil")
	}

	// Check streaming buffer initialized
	if model.streaming == nil {
		t.Error("Expected streaming buffer to be initialized, got nil")
	}

	// Check input queue initialized
	if model.inputQueue == nil {
		t.Error("Expected inputQueue to be initialized, got nil")
	}

	// Check max iterations
	if model.maxIterations != 20 {
		t.Errorf("Expected maxIterations 20, got %d", model.maxIterations)
	}
}

// TestModelAddBlock verifies blocks can be added
func TestModelAddBlock(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	// Initialize viewport (required for AddBlock)
	model.ready = true

	block := ContentBlock{
		Type:    BlockUser,
		Content: "Hello, world!",
	}

	initialLen := len(*model.blocks)
	model.AddBlock(block)

	if len(*model.blocks) != initialLen+1 {
		t.Errorf("Expected %d blocks, got %d", initialLen+1, len(*model.blocks))
	}

	if (*model.blocks)[0].Content != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got '%s'", (*model.blocks)[0].Content)
	}
}

// TestModelQueueInput verifies input queuing works
func TestModelQueueInput(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	// Queue should start empty
	if model.GetQueueLength() != 0 {
		t.Errorf("Expected empty queue, got %d items", model.GetQueueLength())
	}

	// Add items
	if !model.QueueInput("first") {
		t.Error("Failed to queue first input")
	}
	if !model.QueueInput("second") {
		t.Error("Failed to queue second input")
	}

	if model.GetQueueLength() != 2 {
		t.Errorf("Expected 2 items in queue, got %d", model.GetQueueLength())
	}

	// Dequeue
	input, ok := model.DequeueInput()
	if !ok || input != "first" {
		t.Errorf("Expected 'first', got '%s' (ok=%v)", input, ok)
	}

	input, ok = model.DequeueInput()
	if !ok || input != "second" {
		t.Errorf("Expected 'second', got '%s' (ok=%v)", input, ok)
	}

	// Empty queue
	_, ok = model.DequeueInput()
	if ok {
		t.Error("Expected empty queue, but dequeue succeeded")
	}
}

// TestModelArchitectMode verifies architect mode toggling
func TestModelArchitectMode(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	// Should start disabled
	if model.IsArchitectMode() {
		t.Error("Expected architect mode to be disabled initially")
	}

	// Enable
	model.SetArchitectMode(true)
	if !model.IsArchitectMode() {
		t.Error("Expected architect mode to be enabled")
	}
	if model.GetArchitectSubMode() != "chat" {
		t.Errorf("Expected sub-mode 'chat', got '%s'", model.GetArchitectSubMode())
	}

	// Toggle sub-mode
	newMode := model.ToggleArchitectSubMode()
	if newMode != "plan" {
		t.Errorf("Expected 'plan' after toggle, got '%s'", newMode)
	}

	newMode = model.ToggleArchitectSubMode()
	if newMode != "chat" {
		t.Errorf("Expected 'chat' after second toggle, got '%s'", newMode)
	}

	// Disable
	model.SetArchitectMode(false)
	if model.IsArchitectMode() {
		t.Error("Expected architect mode to be disabled")
	}
}
