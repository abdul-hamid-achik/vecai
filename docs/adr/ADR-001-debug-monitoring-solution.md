# ADR-001: Debug Monitoring Solution for vecai

## Status

Proposed

## Context

vecai is a local AI agent using Ollama with a multi-agent architecture (Planner, Executor, Verifier). During development and debugging, we need visibility into:

1. LLM requests and responses (prompts, completions, tool calls)
2. Agent state transitions (intent classification, plan creation, execution)
3. Errors and failures (JSON parsing, model errors, tool failures)
4. Performance metrics (latency, token usage)

Current logging exists (`internal/logger/logger.go`) but uses text format and lacks structured data for the complex agent workflows. The user specifically needs logs in `/tmp` for real-time monitoring.

## Decision Drivers

* Must work for local development debugging
* Needs real-time monitoring capability (tail -f)
* Should capture enough detail to diagnose JSON parsing and model errors
* Should be easy to parse programmatically (grep, jq)
* Must not impact production performance significantly
* Should integrate with existing logger infrastructure

## Considered Options

1. **Structured JSON Debug Logger** - New JSON-based debug tracer writing to /tmp
2. **OpenTelemetry Integration** - Full observability with traces, metrics, spans
3. **Enhanced Text Logging** - Improve existing logger with more context

## Decision Outcome

Chosen option: **Structured JSON Debug Logger**, because:
- Minimal overhead for local development
- JSON is easy to parse with `jq` and grep
- Can be enabled/disabled via environment variable
- No external dependencies required
- Integrates cleanly with existing architecture

### Consequences

**Good:**
* Easy real-time monitoring with `tail -f /tmp/vecai-debug/*.jsonl | jq`
* Structured data enables programmatic analysis
* Minimal code changes required
* Can be extended to OpenTelemetry later

**Bad:**
* JSON logs are larger than text logs
* Requires jq for nice formatting
* Not a full tracing solution (no distributed traces)

## Detailed Design

### C4 Component Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                         vecai Process                             │
│                                                                    │
│  ┌─────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐       │
│  │ Router  │──▶│ Planner  │──▶│ Executor │──▶│ Verifier │       │
│  └────┬────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘       │
│       │             │              │              │               │
│       ▼             ▼              ▼              ▼               │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                     Debug Tracer                             │ │
│  │  - Event logging (state transitions, errors)                 │ │
│  │  - LLM request/response capture                             │ │
│  │  - Tool call tracing                                        │ │
│  └──────────────────────────┬──────────────────────────────────┘ │
└─────────────────────────────┼────────────────────────────────────┘
                              │
                              ▼
            ┌────────────────────────────────────┐
            │     /tmp/vecai-debug/              │
            │  ├── session_<id>.jsonl            │
            │  ├── llm_<id>.jsonl                │
            │  └── latest -> session_<id>.jsonl  │
            └────────────────────────────────────┘
```

### Log File Structure

```
/tmp/vecai-debug/
├── session_2024-01-25_14-30-00.jsonl    # Main debug log
├── llm_2024-01-25_14-30-00.jsonl        # Full LLM request/response payloads
└── latest.jsonl -> session_2024-01-25_14-30-00.jsonl
```

### Event Types

| Event Type | Description | Fields |
|------------|-------------|--------|
| `session.start` | Session initialized | session_id, model, config |
| `session.end` | Session completed | duration_ms, token_total |
| `intent.classified` | Router classified intent | query, intent, confidence |
| `plan.created` | Planner generated plan | plan_id, goal, steps |
| `step.start` | Executor starting step | step_id, description |
| `step.complete` | Executor completed step | step_id, success, duration_ms |
| `llm.request` | LLM request sent | request_id, model, token_count |
| `llm.response` | LLM response received | request_id, duration_ms, token_count |
| `llm.stream_chunk` | Streaming chunk | request_id, type, size |
| `tool.call` | Tool invocation | tool_name, input_preview |
| `tool.result` | Tool result | tool_name, success, output_preview |
| `error` | Any error | error_type, message, context |
| `verification.start` | Verifier starting | files |
| `verification.complete` | Verifier done | passed, warnings, errors |

### JSON Event Schema

```json
{
  "ts": "2024-01-25T14:30:00.123Z",
  "event": "llm.request",
  "session_id": "sess_abc123",
  "trace_id": "tr_xyz789",
  "data": {
    "request_id": "req_001",
    "model": "qwen2.5-coder:7b",
    "messages_count": 5,
    "tools_count": 12,
    "estimated_tokens": 2500
  }
}
```

### LLM Payload Log (Separate File)

Full request/response bodies are large; store separately for detailed debugging:

```json
{
  "ts": "2024-01-25T14:30:00.123Z",
  "request_id": "req_001",
  "type": "request",
  "payload": {
    "model": "qwen2.5-coder:7b",
    "messages": [...],
    "tools": [...]
  }
}
```

### Implementation

#### New File: `internal/debug/tracer.go`

```go
package debug

type Tracer struct {
    sessionID    string
    sessionFile  *os.File
    llmFile      *os.File
    enabled      bool
    mu           sync.Mutex
}

// Global tracer instance
var globalTracer *Tracer

func Init() error { ... }
func Event(eventType string, data map[string]any) { ... }
func LLMRequest(requestID string, payload any) { ... }
func LLMResponse(requestID string, payload any, duration time.Duration) { ... }
func ToolCall(name string, input map[string]any) { ... }
func Error(errType string, err error, context map[string]any) { ... }
func Close() { ... }
```

#### Integration Points

| File | Integration |
|------|-------------|
| `cmd/vecai/main.go` | Call `debug.Init()` on startup |
| `internal/llm/ollama.go` | Wrap Chat/ChatStream with tracing |
| `internal/agent/router.go` | Log intent classification |
| `internal/agent/pipeline.go` | Log plan/step state changes |
| `internal/agent/executor_agent.go` | Log tool calls |
| `internal/agent/verifier_agent.go` | Log verification results |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VECAI_DEBUG` | Enable debug tracing | `false` |
| `VECAI_DEBUG_DIR` | Debug log directory | `/tmp/vecai-debug` |
| `VECAI_DEBUG_LLM` | Log full LLM payloads | `false` |
| `VECAI_DEBUG_LEVEL` | Min level: debug/info/warn/error | `debug` |

### Usage

```bash
# Enable debug mode
export VECAI_DEBUG=1

# Run vecai
./vecai "explain what this project does"

# In another terminal, monitor in real-time
tail -f /tmp/vecai-debug/latest.jsonl | jq

# Filter for errors only
tail -f /tmp/vecai-debug/latest.jsonl | jq 'select(.event == "error")'

# Filter for LLM events
tail -f /tmp/vecai-debug/latest.jsonl | jq 'select(.event | startswith("llm."))'

# View full LLM payloads
cat /tmp/vecai-debug/llm_*.jsonl | jq
```

### Cleanup

Debug logs in `/tmp` are automatically cleaned by the OS on reboot. Additionally, the tracer will:
- Rotate files if they exceed 50MB
- Keep only last 10 session files

## Alternatives Considered

### Option 2: OpenTelemetry Integration

**Pros:**
- Industry standard observability
- Supports distributed tracing
- Rich ecosystem (Jaeger, Grafana)

**Cons:**
- Heavier dependency
- Requires running a collector for local use
- Overkill for local debugging

**Verdict:** Good for production monitoring, but too complex for local development debugging.

### Option 3: Enhanced Text Logging

**Pros:**
- Minimal changes
- Human-readable

**Cons:**
- Hard to parse programmatically
- Can't easily filter by event type
- Loses structure in complex data

**Verdict:** Insufficient for debugging complex agent interactions.

## Implementation Order

1. Create `internal/debug/tracer.go` with core functionality
2. Add initialization in `cmd/vecai/main.go`
3. Instrument `internal/llm/ollama.go` for LLM tracing
4. Instrument pipeline components
5. Add helper scripts for common queries

## Open Questions

1. Should we add a web UI for viewing traces? (Deferred to future)
2. Should traces be opt-in per-session or global? (Global via env var for now)
