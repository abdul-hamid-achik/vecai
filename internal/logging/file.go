package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileWriter writes log messages to a session log file.
// It always logs all levels regardless of console settings.
type FileWriter struct {
	mu       sync.Mutex
	file     *os.File
	logDir   string
	logPath  string
	initOnce sync.Once
	initErr  error
}

// NewFileWriter creates a new file writer.
// The log directory will be created if it doesn't exist.
// Initialization is lazy - the file is only created on first write.
func NewFileWriter(logDir string) *FileWriter {
	return &FileWriter{
		logDir: logDir,
	}
}

// init initializes the file writer lazily on first use.
func (f *FileWriter) init() error {
	f.initOnce.Do(func() {
		f.initErr = f.doInit()
	})
	return f.initErr
}

// doInit performs the actual initialization.
func (f *FileWriter) doInit() error {
	// Get absolute path for logs directory
	logDir := f.logDir
	if !filepath.IsAbs(logDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		logDir = filepath.Join(cwd, f.logDir)
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	// Create session log file with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join(logDir, fmt.Sprintf("session_%s.log", timestamp))

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}

	f.file = file
	f.logPath = logPath

	// Write initial log entry
	cwd, _ := os.Getwd()
	_, _ = fmt.Fprintf(file, "=== Session started at %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	_, _ = fmt.Fprintf(file, "Working directory: %s\n", cwd)
	_, _ = fmt.Fprintf(file, "Log file: %s\n", logPath)
	_, _ = fmt.Fprintf(file, "---\n")

	// Create/update symlink to latest log
	latestPath := filepath.Join(logDir, "latest.log")
	_ = os.Remove(latestPath) // Remove old symlink
	_ = os.Symlink(filepath.Base(logPath), latestPath)

	return nil
}

// Write writes a log message to the file.
// All levels are written regardless of any level settings.
// Format: "15:04:05 LEVEL [prefix] message key=value"
func (f *FileWriter) Write(level Level, prefix, msg string, fields ...Field) error {
	if err := f.init(); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.file == nil {
		return nil
	}

	timestamp := time.Now().Format("15:04:05")

	// Build the log line
	var sb strings.Builder
	sb.WriteString(timestamp)
	sb.WriteString(" ")

	// Level with fixed width
	sb.WriteString(fmt.Sprintf("%-5s", level.String()))
	sb.WriteString(" ")

	// Prefix if set
	if prefix != "" {
		sb.WriteString("[")
		sb.WriteString(prefix)
		sb.WriteString("] ")
	}

	// Message
	sb.WriteString(msg)

	// Fields as key=value pairs
	if len(fields) > 0 {
		for _, fld := range fields {
			sb.WriteString(" ")
			sb.WriteString(fld.Key)
			sb.WriteString("=")
			sb.WriteString(formatValue(fld.Value))
		}
	}

	sb.WriteString("\n")

	_, err := f.file.Write([]byte(sb.String()))
	return err
}

// GetPath returns the path to the current log file.
// Returns empty string if not initialized.
func (f *FileWriter) GetPath() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.logPath
}

// Close closes the file writer.
func (f *FileWriter) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.file != nil {
		_, _ = fmt.Fprintf(f.file, "---\n=== Session ended at %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
		err := f.file.Close()
		f.file = nil
		return err
	}
	return nil
}
