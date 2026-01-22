package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log severity
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

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

// Logger provides structured logging
type Logger struct {
	mu       sync.Mutex
	output   io.Writer
	minLevel Level
	prefix   string
}

// global default logger
var defaultLogger = New(os.Stderr, LevelInfo, "")

// New creates a new logger
func New(output io.Writer, minLevel Level, prefix string) *Logger {
	return &Logger{
		output:   output,
		minLevel: minLevel,
		prefix:   prefix,
	}
}

// SetOutput sets the output destination
func SetOutput(w io.Writer) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.output = w
}

// SetLevel sets the minimum log level
func SetLevel(level Level) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.minLevel = level
}

// SetLevelFromString sets level from string (debug, info, warn, error)
func SetLevelFromString(level string) {
	switch level {
	case "debug":
		SetLevel(LevelDebug)
	case "info":
		SetLevel(LevelInfo)
	case "warn":
		SetLevel(LevelWarn)
	case "error":
		SetLevel(LevelError)
	}
}

// WithPrefix returns a new logger with a prefix
func WithPrefix(prefix string) *Logger {
	return New(defaultLogger.output, defaultLogger.minLevel, prefix)
}

func (l *Logger) log(level Level, format string, args ...any) {
	if level < l.minLevel {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	prefix := ""
	if l.prefix != "" {
		prefix = "[" + l.prefix + "] "
	}

	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(l.output, "%s %s %s%s\n", timestamp, level.String(), prefix, msg)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...any) {
	l.log(LevelDebug, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...any) {
	l.log(LevelInfo, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...any) {
	l.log(LevelWarn, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...any) {
	l.log(LevelError, format, args...)
}

// Package-level functions using default logger

// Debug logs a debug message
func Debug(format string, args ...any) {
	defaultLogger.Debug(format, args...)
}

// Info logs an info message
func Info(format string, args ...any) {
	defaultLogger.Info(format, args...)
}

// Warn logs a warning message
func Warn(format string, args ...any) {
	defaultLogger.Warn(format, args...)
}

// Error logs an error message
func Error(format string, args ...any) {
	defaultLogger.Error(format, args...)
}

// Enabled returns true if the given level would be logged
func Enabled(level Level) bool {
	return level >= defaultLogger.minLevel
}

// DebugEnabled returns true if debug logging is enabled
func DebugEnabled() bool {
	return Enabled(LevelDebug)
}
