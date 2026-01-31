package logging

import (
	"sync"
	"time"
)

// ToolMetrics tracks metrics for a single tool.
type ToolMetrics struct {
	Calls      int           `json:"calls"`
	Errors     int           `json:"errors"`
	TotalTime  time.Duration `json:"total_time_ms"`
	Denied     int           `json:"denied"`
}

// LLMMetrics tracks metrics for LLM operations.
type LLMMetrics struct {
	Requests     int `json:"requests"`
	Errors       int `json:"errors"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Metrics collects runtime metrics for a session.
type Metrics struct {
	mu sync.Mutex

	// Session timing
	SessionStart time.Time `json:"session_start"`
	SessionEnd   time.Time `json:"session_end,omitempty"`

	// Query metrics
	QueriesTotal int `json:"queries_total"`

	// Tool metrics by name
	Tools map[string]*ToolMetrics `json:"tools"`

	// LLM metrics
	LLM LLMMetrics `json:"llm"`

	// Context metrics
	ContextCompactions int `json:"context_compactions"`
	ContextWarnings    int `json:"context_warnings"`
}

// NewMetrics creates a new metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{
		SessionStart: time.Now(),
		Tools:        make(map[string]*ToolMetrics),
	}
}

// RecordQuery increments the query counter.
func (m *Metrics) RecordQuery() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.QueriesTotal++
}

// RecordToolCall records a tool call.
func (m *Metrics) RecordToolCall(name string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tool := m.getOrCreateTool(name)
	tool.Calls++
	tool.TotalTime += duration
	if err != nil {
		tool.Errors++
	}
}

// RecordToolDenied records a denied tool call.
func (m *Metrics) RecordToolDenied(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tool := m.getOrCreateTool(name)
	tool.Denied++
}

// RecordLLMRequest records an LLM request.
func (m *Metrics) RecordLLMRequest(inputTokens, outputTokens int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.LLM.Requests++
	m.LLM.InputTokens += inputTokens
	m.LLM.OutputTokens += outputTokens
	if err != nil {
		m.LLM.Errors++
	}
}

// RecordContextCompaction records a context compaction.
func (m *Metrics) RecordContextCompaction() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ContextCompactions++
}

// RecordContextWarning records a context warning.
func (m *Metrics) RecordContextWarning() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ContextWarnings++
}

// getOrCreateTool gets or creates a tool metrics entry.
func (m *Metrics) getOrCreateTool(name string) *ToolMetrics {
	if m.Tools[name] == nil {
		m.Tools[name] = &ToolMetrics{}
	}
	return m.Tools[name]
}

// Summary returns a summary of the session metrics.
func (m *Metrics) Summary() MetricsSummary {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SessionEnd = time.Now()

	// Calculate totals
	totalToolCalls := 0
	totalToolErrors := 0
	totalToolDenied := 0
	var totalToolTime time.Duration
	for _, t := range m.Tools {
		totalToolCalls += t.Calls
		totalToolErrors += t.Errors
		totalToolDenied += t.Denied
		totalToolTime += t.TotalTime
	}

	return MetricsSummary{
		SessionDuration:    m.SessionEnd.Sub(m.SessionStart),
		QueriesTotal:       m.QueriesTotal,
		ToolCallsTotal:     totalToolCalls,
		ToolErrorsTotal:    totalToolErrors,
		ToolDeniedTotal:    totalToolDenied,
		ToolTimeTotal:      totalToolTime,
		LLMRequestsTotal:   m.LLM.Requests,
		LLMErrorsTotal:     m.LLM.Errors,
		LLMInputTokens:     m.LLM.InputTokens,
		LLMOutputTokens:    m.LLM.OutputTokens,
		ContextCompactions: m.ContextCompactions,
		ContextWarnings:    m.ContextWarnings,
		Tools:              m.Tools,
	}
}

// GetSnapshot returns a copy of the current metrics for serialization.
func (m *Metrics) GetSnapshot() map[string]any {
	summary := m.Summary()
	return map[string]any{
		"session_duration_ms":  summary.SessionDuration.Milliseconds(),
		"queries_total":        summary.QueriesTotal,
		"tool_calls_total":     summary.ToolCallsTotal,
		"tool_errors_total":    summary.ToolErrorsTotal,
		"tool_denied_total":    summary.ToolDeniedTotal,
		"tool_time_total_ms":   summary.ToolTimeTotal.Milliseconds(),
		"llm_requests_total":   summary.LLMRequestsTotal,
		"llm_errors_total":     summary.LLMErrorsTotal,
		"llm_input_tokens":     summary.LLMInputTokens,
		"llm_output_tokens":    summary.LLMOutputTokens,
		"context_compactions":  summary.ContextCompactions,
		"context_warnings":     summary.ContextWarnings,
	}
}

// MetricsSummary provides a summary view of session metrics.
type MetricsSummary struct {
	SessionDuration    time.Duration
	QueriesTotal       int
	ToolCallsTotal     int
	ToolErrorsTotal    int
	ToolDeniedTotal    int
	ToolTimeTotal      time.Duration
	LLMRequestsTotal   int
	LLMErrorsTotal     int
	LLMInputTokens     int
	LLMOutputTokens    int
	ContextCompactions int
	ContextWarnings    int
	Tools              map[string]*ToolMetrics
}
