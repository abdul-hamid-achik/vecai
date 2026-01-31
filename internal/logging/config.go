// Package logging provides a unified logging system for vecai.
// It supports console output, file logging, and structured event tracing.
package logging

import (
	"os"
	"strings"
)

// Level represents log severity levels.
type Level int

const (
	// LevelDebug logs everything, including verbose debugging information.
	LevelDebug Level = iota
	// LevelInfo logs informational messages and above.
	LevelInfo
	// LevelWarn logs warnings and errors only.
	LevelWarn
	// LevelError logs only error messages.
	LevelError
)

// String returns the string representation of a log level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel converts a string to a Level.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Config holds logging configuration.
type Config struct {
	// Level is the minimum level for console output.
	Level Level

	// DebugMode enables full debug tracing to JSONL files.
	DebugMode bool

	// DebugLLM enables logging of full LLM request/response payloads.
	DebugLLM bool

	// DebugDir is the directory for debug trace files.
	// Defaults to /tmp/vecai-debug.
	DebugDir string

	// LogDir is the directory for session log files.
	// Defaults to .vecai/logs in the current working directory.
	LogDir string

	// Verbose enables debug-level console output without full tracing.
	Verbose bool
}

// DefaultDebugDir is the default directory for debug traces.
const DefaultDebugDir = "/tmp/vecai-debug"

// DefaultLogDir is the default directory for session logs (relative to cwd).
const DefaultLogDir = ".vecai/logs"

// ConfigFromEnv creates a Config from environment variables.
//
// Environment variables:
//   - VECAI_DEBUG: Set to "1" to enable debug tracing
//   - VECAI_DEBUG_LLM: Set to "1" to log full LLM payloads
//   - VECAI_DEBUG_DIR: Override debug trace directory
//   - VECAI_LOG_LEVEL: Console log level (debug, info, warn, error)
func ConfigFromEnv() Config {
	cfg := Config{
		Level:    LevelInfo,
		DebugDir: DefaultDebugDir,
		LogDir:   DefaultLogDir,
	}

	// Check VECAI_DEBUG
	if os.Getenv("VECAI_DEBUG") == "1" {
		cfg.DebugMode = true
		cfg.Level = LevelDebug
	}

	// Check VECAI_DEBUG_LLM
	if os.Getenv("VECAI_DEBUG_LLM") == "1" {
		cfg.DebugLLM = true
	}

	// Check VECAI_DEBUG_DIR
	if dir := os.Getenv("VECAI_DEBUG_DIR"); dir != "" {
		cfg.DebugDir = dir
	}

	// Check VECAI_LOG_LEVEL
	if level := os.Getenv("VECAI_LOG_LEVEL"); level != "" {
		cfg.Level = ParseLevel(level)
	}

	return cfg
}

// WithDebugMode returns a copy of the config with debug mode enabled.
func (c Config) WithDebugMode(enabled bool) Config {
	c.DebugMode = enabled
	if enabled {
		c.Level = LevelDebug
	}
	return c
}

// WithVerbose returns a copy of the config with verbose mode enabled.
func (c Config) WithVerbose(enabled bool) Config {
	c.Verbose = enabled
	if enabled {
		c.Level = LevelDebug
	}
	return c
}

// WithLevel returns a copy of the config with the specified level.
func (c Config) WithLevel(level Level) Config {
	c.Level = level
	return c
}
