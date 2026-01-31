package logging

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	// Save and restore environment
	origDebug := os.Getenv("VECAI_DEBUG")
	origDebugLLM := os.Getenv("VECAI_DEBUG_LLM")
	origDebugDir := os.Getenv("VECAI_DEBUG_DIR")
	origLogLevel := os.Getenv("VECAI_LOG_LEVEL")
	defer func() {
		os.Setenv("VECAI_DEBUG", origDebug)
		os.Setenv("VECAI_DEBUG_LLM", origDebugLLM)
		os.Setenv("VECAI_DEBUG_DIR", origDebugDir)
		os.Setenv("VECAI_LOG_LEVEL", origLogLevel)
	}()

	// Clear environment
	os.Unsetenv("VECAI_DEBUG")
	os.Unsetenv("VECAI_DEBUG_LLM")
	os.Unsetenv("VECAI_DEBUG_DIR")
	os.Unsetenv("VECAI_LOG_LEVEL")

	// Test defaults
	cfg := ConfigFromEnv()
	if cfg.Level != LevelInfo {
		t.Errorf("expected default level Info, got %s", cfg.Level)
	}
	if cfg.DebugMode {
		t.Error("expected DebugMode false by default")
	}
	if cfg.DebugDir != DefaultDebugDir {
		t.Errorf("expected default debug dir %s, got %s", DefaultDebugDir, cfg.DebugDir)
	}

	// Test VECAI_DEBUG=1
	os.Setenv("VECAI_DEBUG", "1")
	cfg = ConfigFromEnv()
	if !cfg.DebugMode {
		t.Error("expected DebugMode true when VECAI_DEBUG=1")
	}
	if cfg.Level != LevelDebug {
		t.Errorf("expected level Debug when VECAI_DEBUG=1, got %s", cfg.Level)
	}

	// Test VECAI_DEBUG_LLM=1
	os.Setenv("VECAI_DEBUG_LLM", "1")
	cfg = ConfigFromEnv()
	if !cfg.DebugLLM {
		t.Error("expected DebugLLM true when VECAI_DEBUG_LLM=1")
	}

	// Test custom debug dir
	os.Setenv("VECAI_DEBUG_DIR", "/custom/debug")
	cfg = ConfigFromEnv()
	if cfg.DebugDir != "/custom/debug" {
		t.Errorf("expected custom debug dir, got %s", cfg.DebugDir)
	}

	// Test VECAI_LOG_LEVEL
	os.Setenv("VECAI_LOG_LEVEL", "warn")
	os.Unsetenv("VECAI_DEBUG") // Clear debug mode to test level alone
	cfg = ConfigFromEnv()
	if cfg.Level != LevelWarn {
		t.Errorf("expected level Warn, got %s", cfg.Level)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"ERROR", LevelError},
		{"unknown", LevelInfo}, // Default
		{"", LevelInfo},        // Default
	}

	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.expected {
			t.Errorf("ParseLevel(%q) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func TestConsoleWriter(t *testing.T) {
	var buf bytes.Buffer
	cw := NewConsoleWriter(LevelInfo)
	cw.SetOutput(&buf)

	// Debug should be filtered at Info level
	cw.Write(LevelDebug, "debug message")
	if buf.Len() > 0 {
		t.Error("debug message should be filtered at Info level")
	}

	// Info should pass
	cw.Write(LevelInfo, "info message")
	if !strings.Contains(buf.String(), "info message") {
		t.Error("info message should be logged")
	}
	if !strings.Contains(buf.String(), "INFO") {
		t.Error("level should appear in output")
	}

	// Test with fields
	buf.Reset()
	cw.Write(LevelWarn, "warning", F("key", "value"), F("count", 42))
	output := buf.String()
	if !strings.Contains(output, "key=value") {
		t.Error("field key=value should appear")
	}
	if !strings.Contains(output, "count=42") {
		t.Error("field count=42 should appear")
	}

	// Test level change
	cw.SetLevel(LevelDebug)
	buf.Reset()
	cw.Write(LevelDebug, "debug after level change")
	if !strings.Contains(buf.String(), "debug after level change") {
		t.Error("debug should be logged after level change")
	}
}

func TestConsoleWriterPrefix(t *testing.T) {
	var buf bytes.Buffer
	cw := NewConsoleWriter(LevelInfo)
	cw.SetOutput(&buf)
	cw.SetPrefix("agent")

	cw.Write(LevelInfo, "test message")
	output := buf.String()
	if !strings.Contains(output, "[agent]") {
		t.Errorf("prefix should appear in output: %s", output)
	}
}

func TestFileWriter(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	fw := NewFileWriter(logDir)

	// Write a message
	err := fw.Write(LevelInfo, "test", "hello world")
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Check log directory was created
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Error("log directory should be created")
	}

	// Check log file exists
	logPath := fw.GetPath()
	if logPath == "" {
		t.Fatal("log path should not be empty")
	}
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file should exist")
	}

	// Check latest.log symlink
	latestPath := filepath.Join(logDir, "latest.log")
	if _, err := os.Lstat(latestPath); os.IsNotExist(err) {
		t.Error("latest.log symlink should exist")
	}

	// Read log content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "hello world") {
		t.Error("log file should contain message")
	}
	if !strings.Contains(string(content), "[test]") {
		t.Error("log file should contain prefix")
	}

	// Close and verify session end message
	if err := fw.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
	content, _ = os.ReadFile(logPath)
	if !strings.Contains(string(content), "Session ended") {
		t.Error("log should contain session end message")
	}
}

func TestTracer(t *testing.T) {
	tmpDir := t.TempDir()

	// Test inactive tracer (debug mode false)
	tracer, err := NewTracer(tmpDir, false, false)
	if err != nil {
		t.Fatalf("NewTracer failed: %v", err)
	}
	if tracer.IsEnabled() {
		t.Error("tracer should be disabled when debugMode=false")
	}
	tracer.Event("test.event") // Should be no-op

	// Test active tracer
	tracer, err = NewTracer(tmpDir, true, false)
	if err != nil {
		t.Fatalf("NewTracer failed: %v", err)
	}
	if !tracer.IsEnabled() {
		t.Error("tracer should be enabled when debugMode=true")
	}

	sessionID := tracer.GetSessionID()
	if !strings.HasPrefix(sessionID, "sess_") {
		t.Errorf("session ID should start with 'sess_', got %s", sessionID)
	}

	// Log an event
	tracer.Event(EventToolStart, ToolName("read_file"), Path("/test/file.go"))

	// Log event with data
	tracer.EventWithData(EventLLMRequest, map[string]any{
		"model":    "qwen3:8b",
		"messages": 5,
	}, Duration(100*time.Millisecond))

	// Close tracer
	if err := tracer.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify JSONL file
	tracePath := tracer.GetPath()
	content, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("failed to read trace file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 3 {
		t.Errorf("expected at least 3 lines (start, events, end), got %d", len(lines))
	}

	// Parse first event (session.start)
	var startEvent Event
	if err := json.Unmarshal([]byte(lines[0]), &startEvent); err != nil {
		t.Fatalf("failed to parse start event: %v", err)
	}
	if startEvent.Event != EventSessionStart {
		t.Errorf("first event should be session.start, got %s", startEvent.Event)
	}

	// Check latest.jsonl symlink
	latestPath := filepath.Join(tmpDir, "latest.jsonl")
	if _, err := os.Lstat(latestPath); os.IsNotExist(err) {
		t.Error("latest.jsonl symlink should exist")
	}
}

func TestTracerRequestID(t *testing.T) {
	tmpDir := t.TempDir()

	tracer, err := NewTracer(tmpDir, true, false)
	if err != nil {
		t.Fatalf("NewTracer failed: %v", err)
	}
	defer tracer.Close()

	// Generate new request ID
	reqID := tracer.NewRequestID()
	if !strings.HasPrefix(reqID, "req_") {
		t.Errorf("request ID should start with 'req_', got %s", reqID)
	}

	// Event should include request ID
	tracer.Event(EventToolStart, ToolName("bash"))

	// Clear request ID
	tracer.ClearRequestID()
}

func TestMetrics(t *testing.T) {
	m := NewMetrics()

	// Record queries
	m.RecordQuery()
	m.RecordQuery()

	// Record tool calls
	m.RecordToolCall("read_file", 50*time.Millisecond, nil)
	m.RecordToolCall("read_file", 30*time.Millisecond, nil)
	m.RecordToolCall("bash", 100*time.Millisecond, nil)

	// Record tool denied
	m.RecordToolDenied("write_file")

	// Record LLM requests
	m.RecordLLMRequest(500, 200, nil)
	m.RecordLLMRequest(600, 300, nil)

	// Record context events
	m.RecordContextCompaction()
	m.RecordContextWarning()

	// Get summary
	summary := m.Summary()

	if summary.QueriesTotal != 2 {
		t.Errorf("expected 2 queries, got %d", summary.QueriesTotal)
	}
	if summary.ToolCallsTotal != 3 {
		t.Errorf("expected 3 tool calls, got %d", summary.ToolCallsTotal)
	}
	if summary.ToolDeniedTotal != 1 {
		t.Errorf("expected 1 denied, got %d", summary.ToolDeniedTotal)
	}
	if summary.LLMRequestsTotal != 2 {
		t.Errorf("expected 2 LLM requests, got %d", summary.LLMRequestsTotal)
	}
	if summary.LLMInputTokens != 1100 {
		t.Errorf("expected 1100 input tokens, got %d", summary.LLMInputTokens)
	}
	if summary.LLMOutputTokens != 500 {
		t.Errorf("expected 500 output tokens, got %d", summary.LLMOutputTokens)
	}
	if summary.ContextCompactions != 1 {
		t.Errorf("expected 1 compaction, got %d", summary.ContextCompactions)
	}

	// Check per-tool metrics
	if m.Tools["read_file"] == nil {
		t.Fatal("read_file metrics should exist")
	}
	if m.Tools["read_file"].Calls != 2 {
		t.Errorf("expected 2 read_file calls, got %d", m.Tools["read_file"].Calls)
	}
}

func TestFields(t *testing.T) {
	// Test basic field
	f := F("key", "value")
	if f.Key != "key" || f.Value != "value" {
		t.Error("F() should create field correctly")
	}

	// Test common fields
	if SessionID("abc123").Key != "session_id" {
		t.Error("SessionID should have correct key")
	}
	if ToolName("read_file").Key != "tool" {
		t.Error("ToolName should have correct key")
	}
	if Duration(100 * time.Millisecond).Value != int64(100) {
		t.Error("Duration should convert to milliseconds")
	}
	if Model("qwen3:8b").Value != "qwen3:8b" {
		t.Error("Model should have correct value")
	}

	// Test Query truncation
	longQuery := strings.Repeat("x", 300)
	q := Query(longQuery)
	if len(q.Value.(string)) > 203 { // 200 + "..."
		t.Error("Query should be truncated")
	}
	if !strings.HasSuffix(q.Value.(string), "...") {
		t.Error("truncated Query should end with ...")
	}

	// Test Error field
	errField := Error(nil)
	if errField.Value != nil {
		t.Error("Error(nil) should have nil value")
	}
	errField = Error(os.ErrNotExist)
	if errField.Value != "file does not exist" {
		t.Errorf("Error should extract error string, got %v", errField.Value)
	}
}

func TestLoggerIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		Level:     LevelDebug,
		DebugMode: true,
		DebugDir:  tmpDir,
		LogDir:    filepath.Join(tmpDir, "logs"),
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer logger.Close()

	// Test session ID
	if logger.GetSessionID() == "" {
		t.Error("session ID should be set")
	}

	// Test debug enabled
	if !logger.IsDebugEnabled() {
		t.Error("debug should be enabled")
	}
	if !logger.IsTracingEnabled() {
		t.Error("tracing should be enabled")
	}

	// Test logging methods
	logger.Debug("debug message", F("key", "value"))
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	// Test events
	logger.Event(EventToolStart, ToolName("test"))

	// Test WithPrefix
	prefixedLogger := logger.WithPrefix("test-prefix")
	prefixedLogger.Info("prefixed message")

	// Test metrics
	m := logger.Metrics()
	if m == nil {
		t.Error("metrics should not be nil")
	}
}

func TestGlobalLogger(t *testing.T) {
	// Global should be nil before init
	if Global() != nil {
		t.Error("Global should be nil before Init")
	}

	tmpDir := t.TempDir()
	cfg := Config{
		Level:    LevelInfo,
		LogDir:   filepath.Join(tmpDir, "logs"),
		DebugDir: tmpDir,
	}

	logger, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer func() { _ = Close() }()

	// Global should be set
	if Global() != logger {
		t.Error("Global should return initialized logger")
	}

	// Package-level functions should work
	Debug("debug via package")
	Info("info via package")
	Warn("warn via package")
	LogError("error via package")

	// Close should clear global
	_ = Close()
	if Global() != nil {
		t.Error("Global should be nil after Close")
	}
}
