# ADR-002: Comprehensive Logging Solution for Agent State Tracking

## Status

**Implemented and Migration Complete (2026-01-30)**

All code has been migrated to use `internal/logging`. The deprecated `internal/logger` package has been removed.

## Context

We need a robust logging solution to track agent state properly, similar to how Claude Code handles debugging. The system should:

1. Enable debug logging via `--debug` flag or `VECAI_DEBUG=1` environment variable
2. Track agent state transitions, tool executions, and LLM interactions
3. Provide structured logs for debugging complex agent workflows
4. Support multiple verbosity levels for different use cases

### Current State

The codebase already has two logging mechanisms:

1. **`internal/logger/`** - Basic console/file logger with levels (Debug, Info, Warn, Error)
   - Writes to `.vecai/logs/session_*.log`
   - Always logs everything to file, respects level for console
   - ~55 log calls across 9 files

2. **`internal/debug/`** - Structured event tracer (JSONL format)
   - Writes to `/tmp/vecai-debug/session_*.jsonl`
   - Activated via `VECAI_DEBUG=1` or `--debug`
   - Events: session, intent, LLM, tool calls, errors, plans
   - Optional full LLM payloads via `VECAI_DEBUG_LLM=1`

### Problems with Current Approach

1. **Two disconnected systems** - Logger and Debug tracer operate independently
2. **Missing state transitions** - No tracking of agent mode changes (analysis, architect, quick)
3. **No structured fields** - Logger uses format strings, not structured logging
4. **Missing metrics** - No token counts, latencies, or error rates over time
5. **No log correlation** - Hard to trace a request through the system
6. **Inconsistent verbosity** - Debug flag only affects tracer, not logger level

## Decision Drivers

* Need to debug complex agent state issues
* Claude Code's `--debug` pattern is familiar to users
* Must not impact performance when disabled
* Should be easy to extend with new events
* Logs should be human-readable AND machine-parseable
* Minimal code changes to existing codebase

## Considered Options

### Option 1: Unified Logging with `slog` (Go 1.21+ Structured Logging)

Replace both systems with Go's standard `log/slog` package:

```go
// internal/logging/logger.go
package logging

import (
    "log/slog"
    "os"
)

var Logger *slog.Logger

func Init(debugMode bool) {
    level := slog.LevelInfo
    if debugMode {
        level = slog.LevelDebug
    }

    opts := &slog.HandlerOptions{Level: level}
    handler := slog.NewJSONHandler(os.Stderr, opts)
    Logger = slog.New(handler)
    slog.SetDefault(Logger)
}

// Usage:
slog.Debug("tool executed",
    "tool", toolName,
    "duration_ms", duration,
    "success", true,
)
```

**Pros:**
- Standard library, no dependencies
- Structured by default
- Excellent performance
- Context propagation support

**Cons:**
- Requires refactoring all 55+ log calls
- Loses dual output (console + file)
- JSON output less readable for humans

### Option 2: Enhanced Layered Logging (Recommended)

Keep the existing dual-system architecture but unify them under a single interface with proper state tracking:

```
┌─────────────────────────────────────────────────────────────┐
│                     logging.Log()                           │
│  Single entry point with structured fields + level          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐         ┌──────────────────────────────┐ │
│  │ Console Out  │         │ File Logger                  │ │
│  │ (level-gated)│         │ (.vecai/logs/session_*.log)  │ │
│  └──────────────┘         │ Always DEBUG level           │ │
│                           └──────────────────────────────┘ │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Debug Tracer (when VECAI_DEBUG=1)                    │  │
│  │ - Structured JSONL events                            │  │
│  │ - Full LLM payloads (VECAI_DEBUG_LLM=1)             │  │
│  │ - State transitions, metrics                         │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

**Pros:**
- Minimal refactoring
- Backwards compatible
- Human-readable console, machine-readable trace
- Clear separation of concerns

**Cons:**
- Two file locations for logs
- Slightly more complex codebase

### Option 3: External Observability (OpenTelemetry)

Integrate OpenTelemetry for traces, metrics, and logs:

**Pros:**
- Industry standard
- Exporters for Jaeger, Prometheus, etc.
- Distributed tracing ready

**Cons:**
- Heavy dependency
- Overkill for CLI tool
- Complex setup

## Decision Outcome

**Chosen option: Option 2 - Enhanced Layered Logging**

This approach builds on existing infrastructure while adding:
1. Unified logging interface
2. State tracking events
3. Request correlation IDs
4. Metrics collection
5. Consistent verbosity control

### Implementation Plan

#### Phase 1: Unified Interface (`internal/logging/`)

Create a new `logging` package that unifies logger and debug tracer:

```go
// internal/logging/logging.go
package logging

import (
    "context"
    "sync"
)

// LogLevel represents logging verbosity
type LogLevel int

const (
    LevelDebug LogLevel = iota
    LevelInfo
    LevelWarn
    LevelError
)

// Config holds logging configuration
type Config struct {
    Level        LogLevel
    DebugMode    bool     // VECAI_DEBUG=1 or --debug
    DebugLLM     bool     // VECAI_DEBUG_LLM=1
    DebugDir     string   // VECAI_DEBUG_DIR override
    LogDir       string   // .vecai/logs
    ConsoleOut   bool     // Write to stderr
}

// Logger is the unified logging interface
type Logger struct {
    config    Config
    sessionID string
    mu        sync.RWMutex

    // Internal writers
    console   *consoleWriter
    file      *fileWriter
    tracer    *eventTracer
}

// Field represents a structured log field
type Field struct {
    Key   string
    Value any
}

// F creates a field
func F(key string, value any) Field {
    return Field{Key: key, Value: value}
}

// Log methods with structured fields
func (l *Logger) Debug(msg string, fields ...Field)
func (l *Logger) Info(msg string, fields ...Field)
func (l *Logger) Warn(msg string, fields ...Field)
func (l *Logger) Error(msg string, fields ...Field)

// State tracking
func (l *Logger) StateChange(from, to string, fields ...Field)
func (l *Logger) ToolCall(name string, input map[string]any)
func (l *Logger) ToolResult(name string, success bool, duration time.Duration)
func (l *Logger) LLMRequest(model string, tokens int)
func (l *Logger) LLMResponse(tokens int, duration time.Duration, err error)

// Context-aware logging with request ID
func (l *Logger) WithContext(ctx context.Context) *Logger
func (l *Logger) WithRequestID(id string) *Logger
```

#### Phase 2: State Events

Add comprehensive state tracking events:

```go
// State transition events
const (
    // Agent modes
    EventModeChange       = "agent.mode.change"       // quick, analysis, architect
    EventPermissionChange = "agent.permission.change" // auto, ask, strict

    // Execution flow
    EventQueryStart       = "query.start"
    EventQueryComplete    = "query.complete"
    EventTierSelected     = "tier.selected"

    // Context management
    EventContextAdd       = "context.add"
    EventContextCompact   = "context.compact"
    EventContextWarning   = "context.warning"

    // Session
    EventSessionLoad      = "session.load"
    EventSessionSave      = "session.save"
    EventSessionResume    = "session.resume"

    // Memory layer
    EventMemoryStore      = "memory.store"
    EventMemoryRecall     = "memory.recall"

    // Tool execution
    EventToolStart        = "tool.start"
    EventToolComplete     = "tool.complete"
    EventToolDenied       = "tool.denied"

    // Errors
    EventErrorRecoverable = "error.recoverable"
    EventErrorFatal       = "error.fatal"
)
```

#### Phase 3: Configuration & CLI

Update configuration to support logging options:

```yaml
# vecai.yaml
logging:
  level: info          # debug, info, warn, error
  console: true        # Write to stderr
  file: true           # Write to .vecai/logs/

debug:
  enabled: false       # Override with VECAI_DEBUG=1 or --debug
  llm_payloads: false  # Override with VECAI_DEBUG_LLM=1
  dir: /tmp/vecai-debug
```

CLI flag handling in `cmd/vecai/main.go`:

```go
// Early flag detection (before config load)
debugMode := os.Getenv("VECAI_DEBUG") == "1"
verboseMode := false

for i, arg := range os.Args[1:] {
    switch arg {
    case "--debug", "-d":
        debugMode = true
        os.Args = append(os.Args[:i+1], os.Args[i+2:]...)
    case "--verbose", "-v":
        verboseMode = true  // Sets logger to DEBUG without full tracing
        os.Args = append(os.Args[:i+1], os.Args[i+2:]...)
    }
}

// Initialize logging
logConfig := logging.Config{
    Level:      logging.LevelInfo,
    DebugMode:  debugMode,
    ConsoleOut: true,
}

if verboseMode || debugMode {
    logConfig.Level = logging.LevelDebug
}

log := logging.Init(logConfig)
defer log.Close()
```

#### Phase 4: Output Formats

**Console Output** (human-readable, respects level):
```
15:04:05 DEBUG [agent] mode changed from=interactive to=analysis
15:04:05 INFO  [tool] executing name=read_file path=/foo/bar.go
15:04:05 DEBUG [llm] request model=qwen3:14b tokens=1234
15:04:06 DEBUG [llm] response tokens=567 duration=1.2s
15:04:06 INFO  [tool] complete name=read_file success=true duration=45ms
```

**File Output** (all levels, `.vecai/logs/session_*.log`):
```
2024-01-15T15:04:05.123Z DEBUG [agent] mode changed from=interactive to=analysis session=sess_abc123
2024-01-15T15:04:05.234Z INFO  [tool] executing name=read_file path=/foo/bar.go session=sess_abc123
```

**Debug Trace** (JSONL, `/tmp/vecai-debug/session_*.jsonl`):
```json
{"ts":"2024-01-15T15:04:05.123Z","event":"agent.mode.change","session":"sess_abc123","data":{"from":"interactive","to":"analysis"}}
{"ts":"2024-01-15T15:04:05.234Z","event":"tool.start","session":"sess_abc123","data":{"name":"read_file","path":"/foo/bar.go"}}
```

#### Phase 5: Metrics Collection

Track key metrics for performance analysis:

```go
type Metrics struct {
    mu sync.RWMutex

    // Counters
    ToolCalls      map[string]int64  // By tool name
    ToolErrors     map[string]int64
    LLMRequests    int64
    LLMErrors      int64

    // Gauges
    TokensIn       int64
    TokensOut      int64
    ContextUsage   float64  // Percentage

    // Histograms (simplified)
    ToolLatencies  map[string][]time.Duration
    LLMLatencies   []time.Duration
}

// Flush metrics to log on session end
func (m *Metrics) Summary() map[string]any
```

### Consequences

**Good:**
- Single interface for all logging
- Consistent debug experience (`--debug` works like Claude Code)
- State transitions are tracked and debuggable
- Human-readable AND machine-parseable output
- Minimal changes to existing code
- Request correlation for tracing

**Bad:**
- Two log file locations (can be confusing)
- Slightly larger binary (minimal)
- Need to update ~55 log call sites (can be gradual)

### File Structure After Implementation

```
internal/
├── logging/
│   ├── logging.go       # Main Logger type and interface
│   ├── config.go        # Configuration handling
│   ├── console.go       # Console writer (stderr)
│   ├── file.go          # File writer (.vecai/logs/)
│   ├── tracer.go        # Debug event tracer (JSONL)
│   ├── events.go        # Event type constants
│   ├── metrics.go       # Metrics collection
│   └── fields.go        # Field helpers
└── debug/               # DEPRECATED - to be removed
```

**Note**: The `internal/logger/` package was removed on 2026-01-30 after all call sites were migrated.

### Migration Strategy

1. **Phase 1**: Create `internal/logging/` with new interface ✅
2. **Phase 2**: Add shim functions to old packages that delegate to new ✅
3. **Phase 3**: Gradually update call sites to use new package ✅
4. **Phase 4**: Remove deprecated packages after full migration ✅

**Migration Completed**: All 31+ logger calls across the codebase have been migrated to use the unified `internal/logging` package. The deprecated `internal/logger` package has been removed.

### Environment Variables Summary

| Variable | Purpose | Default |
|----------|---------|---------|
| `VECAI_DEBUG` | Enable full debug tracing | `0` |
| `VECAI_DEBUG_LLM` | Log full LLM payloads | `0` |
| `VECAI_DEBUG_DIR` | Debug trace output directory | `/tmp/vecai-debug` |
| `VECAI_LOG_LEVEL` | Console log level (debug/info/warn/error) | `info` |

### CLI Flags Summary

| Flag | Short | Purpose |
|------|-------|---------|
| `--debug` | `-d` | Enable debug tracing (equiv to `VECAI_DEBUG=1`) |
| `--verbose` | (new) | Set log level to debug without full tracing |
| `--quiet` | `-q` | Suppress non-error console output |

## Alternatives Considered

See Options 1 and 3 above.

## Dependencies

- No new external dependencies
- Uses Go standard library only

## Open Questions

1. Should we add log rotation for file logs?
2. Should debug traces be written to `.vecai/debug/` instead of `/tmp/`?
3. Do we need a `--log-file` flag to specify custom log location?
4. Should we add a `/debug` TUI command to toggle debug mode at runtime?

## References

- Claude Code debug mode behavior
- Go `log/slog` package documentation
- Current `internal/logger/` implementation
- Current `internal/debug/` implementation
