package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("Level(%d).String() = %s, want %s", tt.level, got, tt.expected)
		}
	}
}

func TestLoggerOutput(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelDebug, "")

	l.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("expected output to contain 'INFO', got %q", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got %q", output)
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelWarn, "")

	l.Debug("debug message")
	l.Info("info message")
	l.Warn("warn message")
	l.Error("error message")

	output := buf.String()

	// Debug and Info should be filtered out
	if strings.Contains(output, "debug message") {
		t.Error("debug message should be filtered")
	}
	if strings.Contains(output, "info message") {
		t.Error("info message should be filtered")
	}

	// Warn and Error should be present
	if !strings.Contains(output, "warn message") {
		t.Error("warn message should be present")
	}
	if !strings.Contains(output, "error message") {
		t.Error("error message should be present")
	}
}

func TestLoggerWithPrefix(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	SetLevel(LevelDebug)

	l := WithPrefix("TEST")
	l.Info("prefixed message")

	output := buf.String()
	if !strings.Contains(output, "[TEST]") {
		t.Errorf("expected output to contain '[TEST]', got %q", output)
	}
}

func TestSetLevelFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"debug", LevelDebug},
		{"info", LevelInfo},
		{"warn", LevelWarn},
		{"error", LevelError},
	}

	for _, tt := range tests {
		SetLevelFromString(tt.input)
		if defaultLogger.minLevel != tt.expected {
			t.Errorf("SetLevelFromString(%q) set level to %v, want %v",
				tt.input, defaultLogger.minLevel, tt.expected)
		}
	}
}

func TestDebugEnabled(t *testing.T) {
	SetLevel(LevelDebug)
	if !DebugEnabled() {
		t.Error("DebugEnabled() should return true when level is Debug")
	}

	SetLevel(LevelInfo)
	if DebugEnabled() {
		t.Error("DebugEnabled() should return false when level is Info")
	}
}

func TestFormatArgs(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelDebug, "")

	l.Info("value: %d, name: %s", 42, "test")

	output := buf.String()
	if !strings.Contains(output, "value: 42") {
		t.Errorf("expected formatted value, got %q", output)
	}
	if !strings.Contains(output, "name: test") {
		t.Errorf("expected formatted name, got %q", output)
	}
}
