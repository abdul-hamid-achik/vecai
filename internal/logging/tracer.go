package logging

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event represents a structured debug event for JSONL output.
type Event struct {
	Timestamp string         `json:"ts"`
	Event     string         `json:"event"`
	Session   string         `json:"session"`
	RequestID string         `json:"request_id,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

// LLMPayload represents a full LLM request/response for detailed logging.
type LLMPayload struct {
	Timestamp string         `json:"ts"`
	Type      string         `json:"type"` // "request" or "response"
	Session   string         `json:"session"`
	RequestID string         `json:"request_id"`
	Data      map[string]any `json:"data"`
}

// Tracer writes structured events to JSONL files.
// It is only active when debug mode is enabled.
type Tracer struct {
	mu          sync.Mutex
	sessionID   string
	requestID   string // Current request correlation ID
	sessionFile *os.File
	llmFile     *os.File // Separate file for full LLM payloads
	enabled     bool
	llmEnabled  bool // Whether to log full LLM payloads
	debugDir    string
	sessionPath string
}

// NewTracer creates a new tracer.
// If debugMode is false, the tracer will be inactive (no-op).
func NewTracer(debugDir string, debugMode, llmEnabled bool) (*Tracer, error) {
	t := &Tracer{
		enabled:    debugMode,
		llmEnabled: llmEnabled,
		debugDir:   debugDir,
	}

	if !debugMode {
		return t, nil
	}

	// Create debug directory
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return nil, fmt.Errorf("create debug directory: %w", err)
	}

	// Generate session ID
	t.sessionID = generateID("sess_")

	// Create timestamp for file names
	timestamp := time.Now().Format("2006-01-02_15-04-05")

	// Open session file
	sessionPath := filepath.Join(debugDir, fmt.Sprintf("session_%s.jsonl", timestamp))
	sessionFile, err := os.OpenFile(sessionPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("create session file: %w", err)
	}
	t.sessionFile = sessionFile
	t.sessionPath = sessionPath

	// Open LLM file if enabled
	if llmEnabled {
		llmPath := filepath.Join(debugDir, fmt.Sprintf("llm_%s.jsonl", timestamp))
		llmFile, err := os.OpenFile(llmPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			_ = sessionFile.Close()
			return nil, fmt.Errorf("create LLM file: %w", err)
		}
		t.llmFile = llmFile
	}

	// Create symlink to latest session file
	latestPath := filepath.Join(debugDir, "latest.jsonl")
	_ = os.Remove(latestPath)
	_ = os.Symlink(sessionPath, latestPath)

	// Log session start
	t.logEvent(EventSessionStart, nil, map[string]any{
		"session_id": t.sessionID,
		"debug_dir":  debugDir,
		"llm_trace":  llmEnabled,
	})

	return t, nil
}

// IsEnabled returns whether tracing is active.
func (t *Tracer) IsEnabled() bool {
	return t != nil && t.enabled
}

// GetSessionID returns the current session ID.
func (t *Tracer) GetSessionID() string {
	if t == nil {
		return ""
	}
	return t.sessionID
}

// SetRequestID sets the current request correlation ID.
func (t *Tracer) SetRequestID(id string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requestID = id
}

// NewRequestID generates and sets a new request correlation ID.
func (t *Tracer) NewRequestID() string {
	id := generateID("req_")
	t.SetRequestID(id)
	return id
}

// ClearRequestID clears the current request correlation ID.
func (t *Tracer) ClearRequestID() {
	t.SetRequestID("")
}

// Event logs a structured event.
func (t *Tracer) Event(eventType string, fields ...Field) {
	if t == nil || !t.enabled {
		return
	}
	t.logEvent(eventType, nil, fieldsToMap(fields))
}

// EventWithData logs a structured event with additional data.
func (t *Tracer) EventWithData(eventType string, data map[string]any, fields ...Field) {
	if t == nil || !t.enabled {
		return
	}

	// Merge fields into data
	merged := make(map[string]any)
	for k, v := range data {
		merged[k] = v
	}
	for _, f := range fields {
		merged[f.Key] = f.Value
	}

	t.logEvent(eventType, nil, merged)
}

// logEvent writes an event to the session file.
func (t *Tracer) logEvent(eventType string, requestID *string, data map[string]any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.sessionFile == nil {
		return
	}

	reqID := ""
	if requestID != nil {
		reqID = *requestID
	} else if t.requestID != "" {
		reqID = t.requestID
	}

	event := Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Event:     eventType,
		Session:   t.sessionID,
		RequestID: reqID,
		Data:      data,
	}

	line, err := json.Marshal(event)
	if err != nil {
		return
	}

	_, _ = t.sessionFile.Write(line)
	_, _ = t.sessionFile.Write([]byte("\n"))
}

// LLMRequest logs a full LLM request payload (when VECAI_DEBUG_LLM=1).
func (t *Tracer) LLMRequest(requestID string, payload map[string]any) {
	if t == nil || !t.llmEnabled {
		return
	}
	t.logLLMPayload("request", requestID, payload)
}

// LLMResponse logs a full LLM response payload (when VECAI_DEBUG_LLM=1).
func (t *Tracer) LLMResponse(requestID string, payload map[string]any) {
	if t == nil || !t.llmEnabled {
		return
	}
	t.logLLMPayload("response", requestID, payload)
}

// logLLMPayload writes a full LLM payload to the LLM file.
func (t *Tracer) logLLMPayload(payloadType, requestID string, data map[string]any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.llmFile == nil {
		return
	}

	payload := LLMPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Type:      payloadType,
		Session:   t.sessionID,
		RequestID: requestID,
		Data:      data,
	}

	line, err := json.Marshal(payload)
	if err != nil {
		return
	}

	_, _ = t.llmFile.Write(line)
	_, _ = t.llmFile.Write([]byte("\n"))
}

// GetPath returns the path to the session trace file.
func (t *Tracer) GetPath() string {
	if t == nil {
		return ""
	}
	return t.sessionPath
}

// Close closes the tracer and logs session end.
func (t *Tracer) Close() error {
	if t == nil || !t.enabled {
		return nil
	}

	// Log session end before closing
	t.logEvent(EventSessionEnd, nil, map[string]any{
		"session_id": t.sessionID,
	})

	t.mu.Lock()
	defer t.mu.Unlock()

	var errs []error
	if t.sessionFile != nil {
		if err := t.sessionFile.Close(); err != nil {
			errs = append(errs, err)
		}
		t.sessionFile = nil
	}
	if t.llmFile != nil {
		if err := t.llmFile.Close(); err != nil {
			errs = append(errs, err)
		}
		t.llmFile = nil
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// generateID generates a unique identifier with the given prefix.
func generateID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// GenerateRequestID creates a unique request identifier for LLM calls.
// This is a package-level function for convenience.
func GenerateRequestID() string {
	return generateID("req_")
}
