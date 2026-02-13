package tui

import (
	"strings"
	"testing"
	"time"
)

// TestNewModelTextAreaStyling verifies that the textarea has Nord theme styling applied
func TestNewModelTextAreaStyling(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test-model", streamChan)

	// Verify textarea is initialized
	if model.textArea.Placeholder != "Type message..." {
		t.Errorf("Expected placeholder 'Type message...', got '%s'", model.textArea.Placeholder)
	}

	// Verify char limit is 0 (unlimited)
	if model.textArea.CharLimit != 0 {
		t.Errorf("Expected CharLimit 0, got %d", model.textArea.CharLimit)
	}

	// Verify max height
	if model.textArea.MaxHeight != 5 {
		t.Errorf("Expected MaxHeight 5, got %d", model.textArea.MaxHeight)
	}

	// Verify line numbers are hidden
	if model.textArea.ShowLineNumbers {
		t.Error("Expected ShowLineNumbers to be false")
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

// TestModelAgentMode verifies agent mode cycling
func TestModelAgentMode(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	// Should start in Build mode (default)
	if model.GetAgentMode() != ModeBuild {
		t.Errorf("Expected initial mode ModeBuild, got %v", model.GetAgentMode())
	}

	// Set to Ask
	model.SetAgentMode(ModeAsk)
	if model.GetAgentMode() != ModeAsk {
		t.Errorf("Expected ModeAsk, got %v", model.GetAgentMode())
	}

	// Cycle: Ask → Plan
	next := model.CycleAgentMode()
	if next != ModePlan {
		t.Errorf("Expected ModePlan after cycling from Ask, got %v", next)
	}

	// Cycle: Plan → Build
	next = model.CycleAgentMode()
	if next != ModeBuild {
		t.Errorf("Expected ModeBuild after cycling from Plan, got %v", next)
	}

	// Cycle: Build → Ask
	next = model.CycleAgentMode()
	if next != ModeAsk {
		t.Errorf("Expected ModeAsk after cycling from Build, got %v", next)
	}
}

// TestAgentModeString verifies mode string representations
func TestAgentModeString(t *testing.T) {
	tests := []struct {
		mode AgentMode
		want string
	}{
		{ModeAsk, "Ask"},
		{ModePlan, "Plan"},
		{ModeBuild, "Build"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("AgentMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

// TestAgentModeNext verifies mode cycling
func TestAgentModeNext(t *testing.T) {
	tests := []struct {
		mode AgentMode
		want AgentMode
	}{
		{ModeAsk, ModePlan},
		{ModePlan, ModeBuild},
		{ModeBuild, ModeAsk},
	}
	for _, tt := range tests {
		if got := tt.mode.Next(); got != tt.want {
			t.Errorf("AgentMode(%d).Next() = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

// TestModelModeChangeCallback verifies the mode change callback fires
func TestModelModeChangeCallback(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	var calledWith AgentMode
	called := false
	model.SetModeChangeCallback(func(mode AgentMode) {
		called = true
		calledWith = mode
	})

	model.CycleAgentMode() // Build → Ask
	if !called {
		t.Error("Expected mode change callback to be called")
	}
	if calledWith != ModeAsk {
		t.Errorf("Expected callback with ModeAsk, got %v", calledWith)
	}
}

// === Tool Visualization Tests ===

func TestClassifyTool(t *testing.T) {
	tests := []struct {
		name     string
		expected ToolCategory
	}{
		{"read_file", ToolCategoryRead},
		{"list_files", ToolCategoryRead},
		{"grep", ToolCategoryRead},
		{"vecgrep_search", ToolCategoryRead},
		{"vecgrep_similar", ToolCategoryRead},
		{"vecgrep_status", ToolCategoryRead},
		{"write_file", ToolCategoryWrite},
		{"edit_file", ToolCategoryWrite},
		{"bash", ToolCategoryExecute},
	}
	for _, tt := range tests {
		got := classifyTool(tt.name)
		if got != tt.expected {
			t.Errorf("classifyTool(%q) = %v, want %v", tt.name, got, tt.expected)
		}
	}
}

func TestClassifyToolHeuristic(t *testing.T) {
	tests := []struct {
		name     string
		expected ToolCategory
	}{
		{"search_code", ToolCategoryRead},
		{"get_info", ToolCategoryRead},
		{"list_items", ToolCategoryRead},
		{"read_config", ToolCategoryRead},
		{"check_status", ToolCategoryRead},
		{"deploy_code", ToolCategoryWrite},
		{"custom_tool", ToolCategoryWrite},
	}
	for _, tt := range tests {
		got := classifyTool(tt.name)
		if got != tt.expected {
			t.Errorf("classifyTool(%q) = %v, want %v", tt.name, got, tt.expected)
		}
	}
}

func TestToolCategoryString(t *testing.T) {
	tests := []struct {
		cat  ToolCategory
		want string
	}{
		{ToolCategoryRead, "READ"},
		{ToolCategoryWrite, "WRITE"},
		{ToolCategoryExecute, "EXEC"},
	}
	for _, tt := range tests {
		if got := tt.cat.String(); got != tt.want {
			t.Errorf("ToolCategory(%d).String() = %q, want %q", tt.cat, got, tt.want)
		}
	}
}

func TestToolBlockMetaTiming(t *testing.T) {
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	end := time.Now()

	meta := &ToolBlockMeta{
		ToolType:  ToolCategoryRead,
		StartTime: start,
		EndTime:   end,
		Elapsed:   end.Sub(start),
		GroupID:   "tool-1",
		ResultLen: 2500,
	}

	if meta.Elapsed <= 0 {
		t.Error("Expected positive elapsed time")
	}
	if meta.GroupID != "tool-1" {
		t.Errorf("Expected GroupID 'tool-1', got %q", meta.GroupID)
	}
}

func TestRenderToolCallBlockWithMeta(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolCall,
		ToolName: "read_file",
		Content:  "src/main.go",
		ToolMeta: &ToolBlockMeta{
			ToolType:  ToolCategoryRead,
			StartTime: time.Now(),
			IsRunning: false,
			GroupID:   "tool-1",
		},
	}

	rendered := model.renderToolCallBlock(block)
	if !strings.Contains(rendered, "READ") {
		t.Error("Expected rendered tool call to contain 'READ' category label")
	}
	if !strings.Contains(rendered, "read_file") {
		t.Error("Expected rendered tool call to contain tool name")
	}
	if !strings.Contains(rendered, "src/main.go") {
		t.Error("Expected rendered tool call to contain description")
	}
}

func TestRenderToolCallBlockCategories(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	tests := []struct {
		toolType ToolCategory
		label    string
		icon     string
	}{
		{ToolCategoryRead, "READ", iconToolRead},
		{ToolCategoryWrite, "WRITE", iconToolWrite},
		{ToolCategoryExecute, "EXEC", iconToolExec},
	}

	for _, tt := range tests {
		block := ContentBlock{
			Type:     BlockToolCall,
			ToolName: "test_tool",
			ToolMeta: &ToolBlockMeta{
				ToolType: tt.toolType,
			},
		}
		rendered := model.renderToolCallBlock(block)
		if !strings.Contains(rendered, tt.label) {
			t.Errorf("Expected %q category label in rendered output for %v", tt.label, tt.toolType)
		}
	}
}

func TestRenderToolCallBlockRunningSpinner(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)
	model.spinnerFrame = 3

	block := ContentBlock{
		Type:     BlockToolCall,
		ToolName: "bash",
		Content:  "npm test",
		ToolMeta: &ToolBlockMeta{
			ToolType:  ToolCategoryExecute,
			IsRunning: true,
		},
	}

	rendered := model.renderToolCallBlock(block)
	spinnerFrame := GetSpinnerFrame(3)
	if !strings.Contains(rendered, spinnerFrame) {
		t.Errorf("Expected spinner frame %q in rendered output for running tool", spinnerFrame)
	}
}

func TestRenderToolCallBlockWithoutMeta(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolCall,
		ToolName: "read_file",
		Content:  "src/main.go",
	}

	rendered := model.renderToolCallBlock(block)
	if !strings.Contains(rendered, "read_file") {
		t.Error("Expected tool name in rendered output without meta")
	}
}

func TestRenderToolResultBlockWithElapsed(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolResult,
		ToolName: "read_file",
		Content:  "file contents here",
		ToolMeta: &ToolBlockMeta{
			ToolType: ToolCategoryRead,
			Elapsed:  2 * time.Second,
			GroupID:  "tool-1",
		},
	}

	rendered := model.renderToolResultBlock(block)
	if !strings.Contains(rendered, "2.0s") {
		t.Error("Expected elapsed time '2.0s' in rendered result")
	}
	if !strings.Contains(rendered, "read_file") {
		t.Error("Expected tool name in rendered result")
	}
}

func TestRenderToolResultBlockTruncated(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolResult,
		ToolName: "read_file",
		Content:  "truncated content",
		ToolMeta: &ToolBlockMeta{
			ToolType:  ToolCategoryRead,
			Elapsed:   1 * time.Second,
			ResultLen: 5000,
		},
	}

	rendered := model.renderToolResultBlock(block)
	if !strings.Contains(rendered, "total") {
		t.Error("Expected truncation indicator with 'total' in rendered result")
	}
}

func TestRenderToolResultBlockError(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolResult,
		ToolName: "bash",
		Content:  "command not found",
		IsError:  true,
	}

	rendered := model.renderToolResultBlock(block)
	if !strings.Contains(rendered, "bash") {
		t.Error("Expected tool name in error result")
	}
	if !strings.Contains(rendered, "command not found") {
		t.Error("Expected error message in result")
	}
}

func TestRenderToolResultBlockNoOutput(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	block := ContentBlock{
		Type:     BlockToolResult,
		ToolName: "write_file",
		Content:  "(no output)",
		ToolMeta: &ToolBlockMeta{
			ToolType: ToolCategoryWrite,
			Elapsed:  500 * time.Millisecond,
		},
	}

	rendered := model.renderToolResultBlock(block)
	if !strings.Contains(rendered, "write_file") {
		t.Error("Expected tool name in no-output result")
	}
}

// === Helper Function Tests ===

func TestFormatByteCount(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{100, "100 B"},
		{1024, "1.0 KB"},
		{2500, "2.4 KB"},
		{1048576, "1.0 MB"},
		{5242880, "5.0 MB"},
	}
	for _, tt := range tests {
		got := formatByteCount(tt.input)
		if got != tt.want {
			t.Errorf("formatByteCount(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{500 * time.Millisecond, "500ms"},
		{2 * time.Second, "2.0s"},
		{90 * time.Second, "1m30s"},
		{3661 * time.Second, "1h1m1s"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.input)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestModelActiveToolsTracking(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	if model.activeTools == nil {
		t.Fatal("Expected activeTools to be initialized")
	}
	if len(model.activeTools) != 0 {
		t.Errorf("Expected empty activeTools, got %d", len(model.activeTools))
	}
}

func TestModelCompleterInitialized(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)

	if model.engine == nil {
		t.Fatal("Expected completion engine to be initialized")
	}
	if model.engine.IsActive() {
		t.Error("Expected completion engine to be inactive initially")
	}
}

func TestGetConversationTextWithToolBlocks(t *testing.T) {
	streamChan := make(chan StreamMsg, 10)
	model := NewModel("test", streamChan)
	model.ready = true

	model.AddBlock(ContentBlock{Type: BlockUser, Content: "Hello"})
	model.AddBlock(ContentBlock{Type: BlockToolCall, ToolName: "bash", Content: "ls"})
	model.AddBlock(ContentBlock{Type: BlockToolResult, ToolName: "bash", Content: "file.txt"})
	model.AddBlock(ContentBlock{Type: BlockToolResult, ToolName: "bash", Content: "error!", IsError: true})
	model.AddBlock(ContentBlock{Type: BlockAssistant, Content: "Done"})

	text := model.GetConversationText()
	if !strings.Contains(text, "User: Hello") {
		t.Error("Expected user message in conversation text")
	}
	if !strings.Contains(text, "[Tool: bash]") {
		t.Error("Expected tool call in conversation text")
	}
	if !strings.Contains(text, "[Result: bash]") {
		t.Error("Expected tool result in conversation text")
	}
	if !strings.Contains(text, "[Error: bash]") {
		t.Error("Expected tool error in conversation text")
	}
	if !strings.Contains(text, "Assistant: Done") {
		t.Error("Expected assistant message in conversation text")
	}
}


