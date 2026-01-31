package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ConsoleWriter writes human-readable log messages to stderr.
// It respects log level filtering.
type ConsoleWriter struct {
	mu       sync.Mutex
	output   io.Writer
	minLevel Level
	prefix   string
}

// NewConsoleWriter creates a new console writer with the given minimum level.
func NewConsoleWriter(minLevel Level) *ConsoleWriter {
	return &ConsoleWriter{
		output:   os.Stderr,
		minLevel: minLevel,
	}
}

// SetOutput sets the output destination (mainly for testing).
func (c *ConsoleWriter) SetOutput(w io.Writer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.output = w
}

// SetLevel sets the minimum log level.
func (c *ConsoleWriter) SetLevel(level Level) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.minLevel = level
}

// GetLevel returns the current minimum log level.
func (c *ConsoleWriter) GetLevel() Level {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.minLevel
}

// SetPrefix sets a prefix for all log messages.
func (c *ConsoleWriter) SetPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prefix = prefix
}

// Write writes a log message if the level meets the minimum.
// Format: "15:04:05 LEVEL [prefix] message key=value key=value"
func (c *ConsoleWriter) Write(level Level, msg string, fields ...Field) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if level < c.minLevel {
		return
	}

	timestamp := time.Now().Format("15:04:05")

	// Build the log line
	var sb strings.Builder
	sb.WriteString(timestamp)
	sb.WriteString(" ")

	// Level with fixed width for alignment
	levelStr := level.String()
	sb.WriteString(fmt.Sprintf("%-5s", levelStr))
	sb.WriteString(" ")

	// Prefix if set
	if c.prefix != "" {
		sb.WriteString("[")
		sb.WriteString(c.prefix)
		sb.WriteString("] ")
	}

	// Message
	sb.WriteString(msg)

	// Fields as key=value pairs
	if len(fields) > 0 {
		for _, f := range fields {
			sb.WriteString(" ")
			sb.WriteString(f.Key)
			sb.WriteString("=")
			sb.WriteString(formatValue(f.Value))
		}
	}

	sb.WriteString("\n")

	_, _ = c.output.Write([]byte(sb.String()))
}

// formatValue formats a value for log output.
func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		// Quote strings that contain spaces
		if strings.ContainsAny(val, " \t\n") {
			return fmt.Sprintf("%q", val)
		}
		return val
	case error:
		if val == nil {
			return "<nil>"
		}
		return fmt.Sprintf("%q", val.Error())
	case nil:
		return "<nil>"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// Enabled returns true if the given level would be logged.
func (c *ConsoleWriter) Enabled(level Level) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return level >= c.minLevel
}
