package logging

import (
	"time"
)

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value any
}

// F creates a new Field with the given key and value.
// This is a shorthand for creating Field{Key: k, Value: v}.
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// Common field constructors for frequently used fields.

// SessionID creates a session ID field.
func SessionID(id string) Field {
	return F("session_id", id)
}

// RequestID creates a request ID field.
func RequestID(id string) Field {
	return F("request_id", id)
}

// ToolName creates a tool name field.
func ToolName(name string) Field {
	return F("tool", name)
}

// Duration creates a duration field in milliseconds.
func Duration(d time.Duration) Field {
	return F("duration_ms", d.Milliseconds())
}

// DurationSince creates a duration field from a start time.
func DurationSince(start time.Time) Field {
	return Duration(time.Since(start))
}

// Tokens creates a token count field.
func Tokens(count int) Field {
	return F("tokens", count)
}

// InputTokens creates an input token count field.
func InputTokens(count int) Field {
	return F("input_tokens", count)
}

// OutputTokens creates an output token count field.
func OutputTokens(count int) Field {
	return F("output_tokens", count)
}

// Model creates a model name field.
func Model(name string) Field {
	return F("model", name)
}

// Tier creates a tier field.
func Tier(tier string) Field {
	return F("tier", tier)
}

// Mode creates a mode field.
func Mode(mode string) Field {
	return F("mode", mode)
}

// Query creates a query field, truncating if too long.
func Query(q string) Field {
	if len(q) > 200 {
		q = q[:197] + "..."
	}
	return F("query", q)
}

// Path creates a file path field.
func Path(p string) Field {
	return F("path", p)
}

// Error creates an error field.
func Error(err error) Field {
	if err == nil {
		return F("error", nil)
	}
	return F("error", err.Error())
}

// Success creates a success boolean field.
func Success(ok bool) Field {
	return F("success", ok)
}

// Count creates a count field.
func Count(n int) Field {
	return F("count", n)
}

// From creates a "from" field for state transitions.
func From(value string) Field {
	return F("from", value)
}

// To creates a "to" field for state transitions.
func To(value string) Field {
	return F("to", value)
}

// Reason creates a reason field.
func Reason(r string) Field {
	return F("reason", r)
}

// MessageCount creates a message count field.
func MessageCount(n int) Field {
	return F("msg_count", n)
}

// UsagePercent creates a usage percentage field.
func UsagePercent(pct float64) Field {
	return F("usage_pct", pct)
}

// Iteration creates an iteration count field.
func Iteration(n int) Field {
	return F("iteration", n)
}

// fieldsToMap converts a slice of Fields to a map.
func fieldsToMap(fields []Field) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Value
	}
	return m
}
