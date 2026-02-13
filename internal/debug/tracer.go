package debug

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

// Event types
const (
	EventSessionStart     = "session.start"
	EventSessionEnd       = "session.end"
	EventIntentClassified = "intent.classified"
	EventLLMRequest       = "llm.request"
	EventLLMResponse      = "llm.response"
	EventToolCall         = "tool.call"
	EventToolResult       = "tool.result"
	EventError            = "error"
	EventPlanCreated      = "plan.created"
	EventStepStart        = "step.start"
	EventStepComplete     = "step.complete"
)

// defaultDebugDir returns the default debug directory using os.UserCacheDir.
func defaultDebugDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "vecai", "debug")
	}
	return "/tmp/vecai-debug"
}

// global tracer instance
var (
	globalTracer *Tracer
	globalMu     sync.RWMutex
)

// Event represents a debug event
type Event struct {
	Timestamp string         `json:"ts"`
	Event     string         `json:"event"`
	Session   string         `json:"session"`
	Data      map[string]any `json:"data,omitempty"`
}

// LLMPayload represents a full LLM request/response for detailed logging
type LLMPayload struct {
	Timestamp string         `json:"ts"`
	Type      string         `json:"type"` // "request" or "response"
	Session   string         `json:"session"`
	RequestID string         `json:"request_id"`
	Data      map[string]any `json:"data"`
}

// Tracer handles debug event logging
type Tracer struct {
	sessionID   string
	sessionFile *os.File
	llmFile     *os.File // Separate file for full LLM payloads
	enabled     bool
	llmEnabled  bool // Whether to log full LLM payloads
	debugDir    string
	mu          sync.Mutex
}

// Init initializes the global debug tracer
// Called from main.go when VECAI_DEBUG=1
func Init() error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalTracer != nil {
		return nil // Already initialized
	}

	// Get debug directory from env or use default
	debugDir := os.Getenv("VECAI_DEBUG_DIR")
	if debugDir == "" {
		debugDir = defaultDebugDir()
	}

	// Create debug directory if it doesn't exist
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug directory: %w", err)
	}

	// Generate session ID
	sessionID := generateSessionID()

	// Create timestamp for file names
	timestamp := time.Now().Format("2006-01-02_15-04-05")

	// Open session file
	sessionPath := filepath.Join(debugDir, fmt.Sprintf("session_%s.jsonl", timestamp))
	sessionFile, err := os.OpenFile(sessionPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}

	// Check if LLM logging is enabled
	llmEnabled := os.Getenv("VECAI_DEBUG_LLM") == "1"

	var llmFile *os.File
	if llmEnabled {
		llmPath := filepath.Join(debugDir, fmt.Sprintf("llm_%s.jsonl", timestamp))
		llmFile, err = os.OpenFile(llmPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			_ = sessionFile.Close()
			return fmt.Errorf("failed to create LLM file: %w", err)
		}
	}

	// Create symlink to latest session file
	latestPath := filepath.Join(debugDir, "latest.jsonl")
	_ = os.Remove(latestPath) // Remove existing symlink
	_ = os.Symlink(sessionPath, latestPath)

	tracer := &Tracer{
		sessionID:   sessionID,
		sessionFile: sessionFile,
		llmFile:     llmFile,
		enabled:     true,
		llmEnabled:  llmEnabled,
		debugDir:    debugDir,
	}

	// Log session start (using internal method to avoid deadlock)
	tracer.logEvent(EventSessionStart, map[string]any{
		"session_id": sessionID,
		"debug_dir":  debugDir,
		"llm_trace":  llmEnabled,
	})

	globalTracer = tracer
	return nil
}

// IsEnabled returns whether tracing is active
func IsEnabled() bool {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalTracer != nil && globalTracer.enabled
}

// Event_ logs a structured event (using Event_ to avoid name collision with Event type)
func Event_(eventType string, data map[string]any) {
	globalMu.RLock()
	tracer := globalTracer
	globalMu.RUnlock()

	if tracer == nil || !tracer.enabled {
		return
	}

	tracer.logEvent(eventType, data)
}

// LLMRequest logs an LLM request event
func LLMRequest(requestID, model string, msgCount, toolCount int) {
	Event_(EventLLMRequest, map[string]any{
		"request_id": requestID,
		"model":      model,
		"messages":   msgCount,
		"tools":      toolCount,
	})
}

// LLMResponse logs an LLM response event
func LLMResponse(requestID string, durationMs int64, tokens int, err error) {
	data := map[string]any{
		"request_id":  requestID,
		"duration_ms": durationMs,
		"tokens":      tokens,
	}
	if err != nil {
		data["error"] = err.Error()
	}
	Event_(EventLLMResponse, data)
}

// LLMRequestFull logs full LLM request payload (when VECAI_DEBUG_LLM=1)
func LLMRequestFull(requestID string, payload map[string]any) {
	globalMu.RLock()
	tracer := globalTracer
	globalMu.RUnlock()

	if tracer == nil || !tracer.llmEnabled {
		return
	}

	tracer.logLLMPayload("request", requestID, payload)
}

// LLMResponseFull logs full LLM response payload (when VECAI_DEBUG_LLM=1)
func LLMResponseFull(requestID string, payload map[string]any) {
	globalMu.RLock()
	tracer := globalTracer
	globalMu.RUnlock()

	if tracer == nil || !tracer.llmEnabled {
		return
	}

	tracer.logLLMPayload("response", requestID, payload)
}

// ToolCall logs a tool call event
func ToolCall(name string, input map[string]any) {
	// Truncate large inputs for the event log
	truncatedInput := truncateInput(input)
	Event_(EventToolCall, map[string]any{
		"tool":  name,
		"input": truncatedInput,
	})
}

// ToolResult logs a tool result event
func ToolResult(name string, success bool, resultLen int) {
	Event_(EventToolResult, map[string]any{
		"tool":       name,
		"success":    success,
		"result_len": resultLen,
	})
}

// Error logs an error event
func Error(errType string, err error, ctx map[string]any) {
	data := map[string]any{
		"type":  errType,
		"error": err.Error(),
	}
	for k, v := range ctx {
		data[k] = v
	}
	Event_(EventError, data)
}

// IntentClassified logs an intent classification event
func IntentClassified(query string, intent string, method string) {
	// Truncate query for logging
	truncatedQuery := query
	if len(truncatedQuery) > 100 {
		truncatedQuery = truncatedQuery[:100] + "..."
	}
	Event_(EventIntentClassified, map[string]any{
		"query":  truncatedQuery,
		"intent": intent,
		"method": method,
	})
}

// PlanCreated logs a plan creation event
func PlanCreated(goal string, stepCount int) {
	truncatedGoal := goal
	if len(truncatedGoal) > 100 {
		truncatedGoal = truncatedGoal[:100] + "..."
	}
	Event_(EventPlanCreated, map[string]any{
		"goal":  truncatedGoal,
		"steps": stepCount,
	})
}

// StepStart logs a step start event
func StepStart(stepID string, description string) {
	Event_(EventStepStart, map[string]any{
		"step":        stepID,
		"description": description,
	})
}

// StepComplete logs a step completion event
func StepComplete(stepID string, success bool, err error) {
	data := map[string]any{
		"step":    stepID,
		"success": success,
	}
	if err != nil {
		data["error"] = err.Error()
	}
	Event_(EventStepComplete, data)
}

// Close closes the debug tracer and logs session end
func Close() {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalTracer == nil {
		return
	}

	// Log session end before closing
	globalTracer.logEvent(EventSessionEnd, map[string]any{
		"session_id": globalTracer.sessionID,
	})

	if globalTracer.sessionFile != nil {
		_ = globalTracer.sessionFile.Close()
	}
	if globalTracer.llmFile != nil {
		_ = globalTracer.llmFile.Close()
	}

	globalTracer = nil
}

// logEvent writes an event to the session file
func (t *Tracer) logEvent(eventType string, data map[string]any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.sessionFile == nil {
		return
	}

	event := Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Event:     eventType,
		Session:   t.sessionID,
		Data:      data,
	}

	line, err := json.Marshal(event)
	if err != nil {
		return
	}

	_, _ = t.sessionFile.Write(line)
	_, _ = t.sessionFile.Write([]byte("\n"))
}

// logLLMPayload writes full LLM payload to the LLM file
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

// generateSessionID creates a unique session identifier
func generateSessionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "sess_" + hex.EncodeToString(b)
}

// GenerateRequestID creates a unique request identifier for LLM calls
func GenerateRequestID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}

// truncateInput truncates input values for event logging
func truncateInput(input map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range input {
		switch val := v.(type) {
		case string:
			if len(val) > 200 {
				result[k] = val[:200] + "..."
			} else {
				result[k] = val
			}
		default:
			result[k] = v
		}
	}
	return result
}
