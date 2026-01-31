// Package logging provides a unified logging system for vecai.
//
// The logging system has three output channels:
//   - Console (stderr): Human-readable messages, respects log level
//   - File (.vecai/logs/): Session logs, always captures all levels
//   - Tracer (JSONL): Structured events for debugging, only active in debug mode
//
// Usage:
//
//	log, err := logging.Init(logging.ConfigFromEnv())
//	if err != nil {
//	    // handle error
//	}
//	defer log.Close()
//
//	log.Info("Starting session")
//	log.Debug("Verbose info", logging.F("key", "value"))
//	log.Event(logging.EventToolStart, logging.ToolName("read_file"))
package logging

import (
	"sync"
)

// Logger is the main logging interface.
// It provides methods for console/file logging and structured event tracing.
type Logger struct {
	mu sync.RWMutex

	config  Config
	console *ConsoleWriter
	file    *FileWriter
	tracer  *Tracer
	metrics *Metrics

	// Session ID for correlation
	sessionID string

	// Current component prefix (e.g., "agent", "llm", "tool")
	prefix string
}

// global logger instance
var (
	globalLogger *Logger
	globalMu     sync.RWMutex
)

// Init initializes the global logger with the given configuration.
// This should be called early in main() before any logging occurs.
func Init(cfg Config) (*Logger, error) {
	globalMu.Lock()
	defer globalMu.Unlock()

	logger, err := New(cfg)
	if err != nil {
		return nil, err
	}

	globalLogger = logger
	return logger, nil
}

// New creates a new Logger instance.
func New(cfg Config) (*Logger, error) {
	// Create console writer
	consoleLevel := cfg.Level
	if cfg.Verbose || cfg.DebugMode {
		consoleLevel = LevelDebug
	}
	console := NewConsoleWriter(consoleLevel)

	// Create file writer
	file := NewFileWriter(cfg.LogDir)

	// Create tracer (only active if debug mode enabled)
	tracer, err := NewTracer(cfg.DebugDir, cfg.DebugMode, cfg.DebugLLM)
	if err != nil {
		return nil, err
	}

	logger := &Logger{
		config:    cfg,
		console:   console,
		file:      file,
		tracer:    tracer,
		metrics:   NewMetrics(),
		sessionID: tracer.GetSessionID(),
	}

	return logger, nil
}

// Global returns the global logger instance.
// Returns nil if Init has not been called.
func Global() *Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalLogger
}

// WithPrefix returns a new logger with the given prefix.
// The prefix appears in log output as [prefix].
func (l *Logger) WithPrefix(prefix string) *Logger {
	if l == nil {
		return nil
	}
	return &Logger{
		config:    l.config,
		console:   l.console,
		file:      l.file,
		tracer:    l.tracer,
		metrics:   l.metrics,
		sessionID: l.sessionID,
		prefix:    prefix,
	}
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, fields ...Field) {
	if l == nil {
		return
	}
	l.log(LevelDebug, msg, fields...)
}

// Info logs an informational message.
func (l *Logger) Info(msg string, fields ...Field) {
	if l == nil {
		return
	}
	l.log(LevelInfo, msg, fields...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, fields ...Field) {
	if l == nil {
		return
	}
	l.log(LevelWarn, msg, fields...)
}

// Error logs an error message.
func (l *Logger) Error(msg string, fields ...Field) {
	if l == nil {
		return
	}
	l.log(LevelError, msg, fields...)
}

// log writes to console and file.
func (l *Logger) log(level Level, msg string, fields ...Field) {
	// Add prefix to console output
	if l.prefix != "" {
		l.console.SetPrefix(l.prefix)
	}
	l.console.Write(level, msg, fields...)
	l.console.SetPrefix("") // Reset prefix

	// Write to file (always all levels)
	_ = l.file.Write(level, l.prefix, msg, fields...)
}

// Event logs a structured event to the tracer.
// Events are only written when debug mode is enabled.
func (l *Logger) Event(eventType string, fields ...Field) {
	if l == nil || !l.tracer.IsEnabled() {
		return
	}
	l.tracer.Event(eventType, fields...)
}

// EventWithData logs a structured event with additional data.
func (l *Logger) EventWithData(eventType string, data map[string]any, fields ...Field) {
	if l == nil || !l.tracer.IsEnabled() {
		return
	}
	l.tracer.EventWithData(eventType, data, fields...)
}

// NewRequestID generates a new request correlation ID and sets it for subsequent events.
func (l *Logger) NewRequestID() string {
	if l == nil || l.tracer == nil {
		return GenerateRequestID()
	}
	return l.tracer.NewRequestID()
}

// SetRequestID sets the current request correlation ID.
func (l *Logger) SetRequestID(id string) {
	if l == nil || l.tracer == nil {
		return
	}
	l.tracer.SetRequestID(id)
}

// ClearRequestID clears the current request correlation ID.
func (l *Logger) ClearRequestID() {
	if l == nil || l.tracer == nil {
		return
	}
	l.tracer.ClearRequestID()
}

// LLMRequest logs a full LLM request payload (when VECAI_DEBUG_LLM=1).
func (l *Logger) LLMRequest(requestID string, payload map[string]any) {
	if l == nil || l.tracer == nil {
		return
	}
	l.tracer.LLMRequest(requestID, payload)
}

// LLMResponse logs a full LLM response payload (when VECAI_DEBUG_LLM=1).
func (l *Logger) LLMResponse(requestID string, payload map[string]any) {
	if l == nil || l.tracer == nil {
		return
	}
	l.tracer.LLMResponse(requestID, payload)
}

// Metrics returns the metrics collector.
func (l *Logger) Metrics() *Metrics {
	if l == nil {
		return nil
	}
	return l.metrics
}

// GetSessionID returns the current session ID.
func (l *Logger) GetSessionID() string {
	if l == nil {
		return ""
	}
	return l.sessionID
}

// IsDebugEnabled returns true if debug logging is enabled.
func (l *Logger) IsDebugEnabled() bool {
	if l == nil {
		return false
	}
	return l.console.Enabled(LevelDebug)
}

// IsTracingEnabled returns true if event tracing is enabled.
func (l *Logger) IsTracingEnabled() bool {
	if l == nil {
		return false
	}
	return l.tracer.IsEnabled()
}

// SetLevel sets the console log level.
func (l *Logger) SetLevel(level Level) {
	if l == nil {
		return
	}
	l.console.SetLevel(level)
}

// Close closes the logger and all its writers.
// This should be called on application exit.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}

	// Log session summary if tracing is enabled
	if l.tracer.IsEnabled() && l.metrics != nil {
		summary := l.metrics.GetSnapshot()
		l.tracer.EventWithData(EventSessionEnd, summary)
	}

	// Close all writers
	var errs []error
	if err := l.file.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := l.tracer.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Package-level convenience functions using the global logger

// Debug logs a debug message to the global logger.
func Debug(msg string, fields ...Field) {
	if l := Global(); l != nil {
		l.Debug(msg, fields...)
	}
}

// Info logs an informational message to the global logger.
func Info(msg string, fields ...Field) {
	if l := Global(); l != nil {
		l.Info(msg, fields...)
	}
}

// Warn logs a warning message to the global logger.
func Warn(msg string, fields ...Field) {
	if l := Global(); l != nil {
		l.Warn(msg, fields...)
	}
}

// Error logs an error message to the global logger.
func LogError(msg string, fields ...Field) {
	if l := Global(); l != nil {
		l.Error(msg, fields...)
	}
}

// Event logs a structured event to the global logger.
func LogEvent(eventType string, fields ...Field) {
	if l := Global(); l != nil {
		l.Event(eventType, fields...)
	}
}

// Close closes the global logger.
func Close() error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalLogger != nil {
		err := globalLogger.Close()
		globalLogger = nil
		return err
	}
	return nil
}

// DebugEnabled returns true if debug logging is enabled in the global logger.
func DebugEnabled() bool {
	if l := Global(); l != nil {
		return l.IsDebugEnabled()
	}
	return false
}

// TracingEnabled returns true if event tracing is enabled in the global logger.
func TracingEnabled() bool {
	if l := Global(); l != nil {
		return l.IsTracingEnabled()
	}
	return false
}
