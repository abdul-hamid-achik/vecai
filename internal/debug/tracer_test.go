package debug

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Init / Close lifecycle
// ---------------------------------------------------------------------------

func TestInitAndClose(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VECAI_DEBUG_DIR", dir)

	if err := Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	defer Close()

	if !IsEnabled() {
		t.Error("expected IsEnabled() to be true after Init")
	}

	// Verify a session file was created
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "session_") && strings.HasSuffix(e.Name(), ".jsonl") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a session_*.jsonl file in debug dir")
	}

	Close()

	if IsEnabled() {
		t.Error("expected IsEnabled() to be false after Close")
	}
}

func TestInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VECAI_DEBUG_DIR", dir)

	if err := Init(); err != nil {
		t.Fatalf("first Init() error: %v", err)
	}
	defer Close()

	// Second Init should be a no-op (not an error)
	if err := Init(); err != nil {
		t.Fatalf("second Init() error: %v", err)
	}
}

func TestClose_WithoutInit(t *testing.T) {
	// Ensure the global tracer is nil
	globalMu.Lock()
	globalTracer = nil
	globalMu.Unlock()

	// Close on nil tracer should not panic
	Close()
}

// ---------------------------------------------------------------------------
// Nil-safety: calling debug functions before Init
// ---------------------------------------------------------------------------

func TestNilSafety_EventBeforeInit(t *testing.T) {
	// Ensure no global tracer
	globalMu.Lock()
	globalTracer = nil
	globalMu.Unlock()

	// None of these should panic
	Event_("test.event", map[string]any{"key": "value"})
	LLMRequest("req_1", "model", 3, 2)
	LLMResponse("req_1", 100, 500, nil)
	LLMRequestFull("req_1", map[string]any{"model": "test"})
	LLMResponseFull("req_1", map[string]any{"content": "hello"})
	ToolCall("bash", map[string]any{"command": "ls"})
	ToolResult("bash", true, 42)
	Error("test_error", errForTest("oops"), nil)
	IntentClassified("query", "code_search", "embedding")
	PlanCreated("do something", 3)
	StepStart("step-1", "first step")
	StepComplete("step-1", true, nil)

	if IsEnabled() {
		t.Error("expected IsEnabled() to be false without Init")
	}
}

// errForTest is a simple error type for testing.
type errForTest string

func (e errForTest) Error() string { return string(e) }

// ---------------------------------------------------------------------------
// Event formatting: verify logged events are valid JSONL
// ---------------------------------------------------------------------------

func TestEventFormatting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VECAI_DEBUG_DIR", dir)

	if err := Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Log a few events
	Event_("test.custom", map[string]any{"foo": "bar", "count": 42})
	LLMRequest("req_test", "llama3", 5, 2)
	ToolCall("bash", map[string]any{"command": "echo hello"})
	ToolResult("bash", true, 11)

	Close()

	// Read the session file and verify each line is valid JSON
	entries, _ := os.ReadDir(dir)
	var sessionFile string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "session_") && strings.HasSuffix(e.Name(), ".jsonl") {
			sessionFile = filepath.Join(dir, e.Name())
			break
		}
	}
	if sessionFile == "" {
		t.Fatal("no session file found")
	}

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (start + events + end), got %d", len(lines))
	}

	for i, line := range lines {
		var evt Event
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nline: %s", i, err, line)
			continue
		}
		if evt.Timestamp == "" {
			t.Errorf("line %d: missing timestamp", i)
		}
		if evt.Event == "" {
			t.Errorf("line %d: missing event type", i)
		}
		if evt.Session == "" {
			t.Errorf("line %d: missing session ID", i)
		}
	}

	// The first event should be session.start, last should be session.end
	var first, last Event
	_ = json.Unmarshal([]byte(lines[0]), &first)
	_ = json.Unmarshal([]byte(lines[len(lines)-1]), &last)
	if first.Event != EventSessionStart {
		t.Errorf("first event should be %q, got %q", EventSessionStart, first.Event)
	}
	if last.Event != EventSessionEnd {
		t.Errorf("last event should be %q, got %q", EventSessionEnd, last.Event)
	}
}

// ---------------------------------------------------------------------------
// GenerateRequestID
// ---------------------------------------------------------------------------

func TestGenerateRequestID_Format(t *testing.T) {
	id := GenerateRequestID()
	if !strings.HasPrefix(id, "req_") {
		t.Errorf("expected req_ prefix, got %q", id)
	}
	// "req_" + 12 hex chars (6 bytes)
	if len(id) != 4+12 {
		t.Errorf("expected length 16, got %d (%q)", len(id), id)
	}
}

func TestGenerateRequestID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateRequestID()
		if seen[id] {
			t.Fatalf("duplicate request ID: %q", id)
		}
		seen[id] = true
	}
}

// ---------------------------------------------------------------------------
// generateSessionID (internal)
// ---------------------------------------------------------------------------

func TestGenerateSessionID_Format(t *testing.T) {
	id := generateSessionID()
	if !strings.HasPrefix(id, "sess_") {
		t.Errorf("expected sess_ prefix, got %q", id)
	}
	// "sess_" + 16 hex chars (8 bytes)
	if len(id) != 5+16 {
		t.Errorf("expected length 21, got %d (%q)", len(id), id)
	}
}

// ---------------------------------------------------------------------------
// truncateInput (internal)
// ---------------------------------------------------------------------------

func TestTruncateInput_ShortValues(t *testing.T) {
	input := map[string]any{
		"key":  "short",
		"num":  42,
		"bool": true,
	}
	result := truncateInput(input)
	if result["key"] != "short" {
		t.Errorf("expected 'short', got %v", result["key"])
	}
	if result["num"] != 42 {
		t.Errorf("expected 42, got %v", result["num"])
	}
}

func TestTruncateInput_LongString(t *testing.T) {
	long := strings.Repeat("x", 300)
	input := map[string]any{"data": long}
	result := truncateInput(input)
	val, ok := result["data"].(string)
	if !ok {
		t.Fatalf("expected string, got %T", result["data"])
	}
	if len(val) != 203 { // 200 + "..."
		t.Errorf("expected truncated length 203, got %d", len(val))
	}
	if !strings.HasSuffix(val, "...") {
		t.Error("expected truncated string to end with ...")
	}
}

// ---------------------------------------------------------------------------
// LLM payload logging
// ---------------------------------------------------------------------------

func TestLLMPayloadLogging(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VECAI_DEBUG_DIR", dir)
	t.Setenv("VECAI_DEBUG_LLM", "1")

	if err := Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	LLMRequestFull("req_test", map[string]any{"model": "llama3", "messages": 5})
	LLMResponseFull("req_test", map[string]any{"content": "response text"})

	Close()

	// Find the LLM file
	entries, _ := os.ReadDir(dir)
	var llmFile string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "llm_") && strings.HasSuffix(e.Name(), ".jsonl") {
			llmFile = filepath.Join(dir, e.Name())
			break
		}
	}
	if llmFile == "" {
		t.Fatal("expected an llm_*.jsonl file when VECAI_DEBUG_LLM=1")
	}

	data, err := os.ReadFile(llmFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (request + response), got %d", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var payload LLMPayload
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
			continue
		}
		if payload.RequestID != "req_test" {
			t.Errorf("line %d: expected request_id req_test, got %q", i, payload.RequestID)
		}
	}

	// First should be request, second should be response
	var p0, p1 LLMPayload
	_ = json.Unmarshal([]byte(lines[0]), &p0)
	_ = json.Unmarshal([]byte(lines[1]), &p1)
	if p0.Type != "request" {
		t.Errorf("first payload type should be request, got %q", p0.Type)
	}
	if p1.Type != "response" {
		t.Errorf("second payload type should be response, got %q", p1.Type)
	}
}
