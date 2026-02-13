# AGENTS.md - vecai Agent Architecture

This document describes the AI agent architecture used in vecai, including the agent loop, tool execution, permission system, and skills integration.

## Overview

vecai implements a **ReAct (Reasoning + Acting)** agent pattern where the AI:
1. Receives a user query
2. Reasons about what information it needs
3. Executes tools to gather information or take actions
4. Continues until it can provide a complete answer

```
┌─────────────────────────────────────────────────────────────┐
│                        User Query                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Skills Matching                         │
│         (Check if query triggers a skill prompt)            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       Agent Loop                            │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │  LLM Call   │───▶│ Tool Calls? │───▶│  Execute    │     │
│  │  (Ollama)   │    │             │    │   Tools     │     │
│  └─────────────┘    └─────────────┘    └─────────────┘     │
│         ▲                  │                   │            │
│         │                  │ No                │            │
│         │                  ▼                   │            │
│         │           ┌─────────────┐            │            │
│         └───────────│   Return    │◀───────────┘            │
│                     │  Response   │                         │
│                     └─────────────┘                         │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### Agent (`internal/agent/agent.go`)

The `Agent` struct is the central coordinator:

```go
type Agent struct {
    llm         llm.LLMClient        // Ollama LLM client
    tools       *tools.Registry       // Available tools
    permissions *permissions.Policy   // Permission checking
    skills      *skills.Loader        // Skill prompts
    output      *ui.OutputHandler     // Console output
    input       *ui.InputHandler      // User input
    config      *config.Config        // Configuration
    contextMgr  *context.ContextManager // Context window management
    planner     *Planner              // Plan mode handler
}
```

### Agent Loop

The agent loop (`runAgentLoop`) executes up to configurable max iterations (default 20):

```go
for i := 0; i < maxIterations; i++ {
    // 1. Get tool definitions for LLM
    toolDefs := a.getToolDefinitions()

    // 2. Stream response from Ollama
    stream := a.llm.ChatStream(ctx, messages, toolDefs, systemPrompt)

    // 3. Process stream chunks (text, thinking, tool_calls)
    for chunk := range stream {
        switch chunk.Type {
        case "text":     // Display text to user
        case "thinking": // Display thinking (if extended thinking enabled)
        case "tool_call": // Collect tool calls
        case "done":     // Response complete
        }
    }

    // 4. If no tool calls, we're done
    if len(response.ToolCalls) == 0 {
        return nil
    }

    // 5. Execute tool calls with permission checking
    results := a.executeToolCalls(ctx, response.ToolCalls)

    // 6. Add results to conversation and continue loop
    a.messages = append(a.messages, toolResultMessage)
}
```

## Tool System

### Tool Interface (`internal/tools/registry.go`)

All tools implement this interface:

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Execute(ctx context.Context, input map[string]any) (string, error)
    Permission() PermissionLevel
}
```

### Permission Levels

```go
const (
    PermissionRead    = 0  // Read-only (vecgrep_search, read_file, list_files, grep)
    PermissionWrite   = 1  // File modifications (write_file, edit_file)
    PermissionExecute = 2  // Shell execution (bash)
)
```

### Available Tools

#### Core File Tools
| Tool | Permission | Description |
|------|------------|-------------|
| `read_file` | Read | Read file contents with line ranges |
| `write_file` | Write | Create or overwrite files |
| `edit_file` | Write | Make targeted find/replace edits |
| `list_files` | Read | List directory contents recursively |
| `grep` | Read | Regex pattern search (ripgrep-based) |
| `bash` | Execute | Run shell commands (sandboxed) |

#### Go Development Tools
| Tool | Permission | Description |
|------|------------|-------------|
| `ast_parse` | Read | Parse and analyze Go AST structures |
| `lsp_query` | Read | Go language server queries (definitions, references) |
| `linter` | Read | Run golangci-lint on files |
| `test_runner` | Execute | Run Go tests with filtering |

#### Semantic Search (vecgrep)
| Tool | Permission | Description |
|------|------------|-------------|
| `vecgrep_search` | Read | Semantic, keyword, or hybrid code search |
| `vecgrep_similar` | Read | Find similar code by snippet, location, or chunk ID |
| `vecgrep_status` | Read | Check search index status |
| `vecgrep_index` | Write | Index or re-index files |
| `vecgrep_clean` | Write | Remove orphaned data and optimize DB |
| `vecgrep_delete` | Write | Delete a file from the search index |
| `vecgrep_init` | Write | Initialize vecgrep in current project |

#### Git Visualization (gpeek)
| Tool | Permission | Description |
|------|------------|-------------|
| `gpeek_status` | Read | Repository status overview |
| `gpeek_diff` | Read | Structured diffs with hunks |
| `gpeek_log` | Read | Commit history with filters |
| `gpeek_summary` | Read | Complete repo snapshot |
| `gpeek_blame` | Read | Line-by-line attribution |
| `gpeek_branches` | Read | List branches |
| `gpeek_stashes` | Read | List stashes |
| `gpeek_tags` | Read | List tags |
| `gpeek_changes_between` | Read | Changes between refs |
| `gpeek_conflict_check` | Read | Predict merge conflicts |

#### Web Search
| Tool | Permission | Description |
|------|------------|-------------|
| `web_search` | Read | Search the web (requires TAVILY_API_KEY) |

#### Memory (noted)
| Tool | Permission | Description |
|------|------------|-------------|
| `noted_remember` | Write | Store memories with tags and importance |
| `noted_recall` | Read | Search memories semantically |
| `noted_forget` | Write | Delete memories by ID, tags, or age |

### Tool Execution Flow

```
┌─────────────────┐
│   Tool Call     │
│   from LLM      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Find Tool in   │───▶ Unknown? Return error
│    Registry     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Check        │
│   Permission    │───▶ Denied? Return "Permission denied"
└────────┬────────┘
         │ Allowed
         ▼
┌─────────────────┐
│  Execute Tool   │
│                 │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Return Result   │
│   to Agent      │
└─────────────────┘
```

## Permission System

### Modes (`internal/permissions/policy.go`)

```go
const (
    ModeAsk      = 0  // Default: prompt for write/execute, auto-approve reads
    ModeAuto     = 1  // Approve everything (--auto flag)
    ModeStrict   = 2  // Prompt for everything including reads
    ModeAnalysis = 3  // Read-only, block all writes/executes
)
```

### Decision Flow

```
                    ┌─────────────┐
                    │ Tool Called │
                    └──────┬──────┘
                           │
                           ▼
                ┌─────────────────────┐
                │  Mode == Auto?      │───Yes──▶ Allow
                └──────────┬──────────┘
                           │ No
                           ▼
                ┌─────────────────────┐
                │  Check Cache        │
                │  (always/never)     │───Hit──▶ Use cached decision
                └──────────┬──────────┘
                           │ Miss
                           ▼
                ┌─────────────────────┐
                │ Mode == Ask AND     │
                │ Level == Read?      │───Yes──▶ Allow
                └──────────┬──────────┘
                           │ No
                           ▼
                ┌─────────────────────┐
                │  Prompt User        │
                │  [y/n/a/v]          │
                └──────────┬──────────┘
                           │
           ┌───────┬───────┴───────┬───────┐
           ▼       ▼               ▼       ▼
         Allow   Deny          Always   Never
                               (cache)  (cache)
```

### User Responses

- `y` / `yes` - Allow this time
- `n` / `no` - Deny this time
- `a` / `always` - Always allow this tool (cached for session)
- `v` / `never` - Never allow this tool (cached for session)

## Skills System

### Skill Structure (`internal/skills/loader.go`)

Skills are markdown files with YAML frontmatter:

```markdown
---
name: code-review
description: Thorough code review
triggers:
  - "review"
  - "/review (this|the|my) code/"
tags:
  - quality
---

# Code Review Instructions

Your prompt content here...
```

### Skill Matching

1. Plain text triggers: case-insensitive substring match
2. Regex triggers: wrapped in `/`, matched against query

```go
func (s *Skill) Matches(query string) bool {
    query = strings.ToLower(query)
    for _, trigger := range s.Triggers {
        if strings.HasPrefix(trigger, "/") && strings.HasSuffix(trigger, "/") {
            // Regex match
            pattern := trigger[1 : len(trigger)-1]
            if matched, _ := regexp.MatchString(pattern, query); matched {
                return true
            }
        } else {
            // Substring match
            if strings.Contains(query, strings.ToLower(trigger)) {
                return true
            }
        }
    }
    return false
}
```

### Skill Integration

When a skill matches, its prompt is prepended to the user's query:

```go
if skill := a.skills.Match(query); skill != nil {
    query = skill.GetPrompt() + "\n\nUser request: " + query
}
```

## Plan Mode

### Planner (`internal/agent/planner.go`)

Plan mode breaks complex goals into steps with user interaction:

```
┌─────────────────────────────────────────────────────────────┐
│                    Phase 1: Gather Context                  │
│  - Use tools to understand codebase                         │
│  - Generate questions and plan steps                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Phase 2: Questionnaire                    │
│  - Present questions to user                                │
│  - Collect answers                                          │
│  - Refine plan based on answers                             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Phase 3: Confirmation                    │
│  - Display final plan                                       │
│  - Ask user to confirm execution                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Phase 4: Execution                       │
│  - Execute plan steps via main agent                        │
│  - Report progress                                          │
└─────────────────────────────────────────────────────────────┘
```

### Plan Format

The planner generates structured output:

```markdown
## Questions (if any)
1. What authentication method should we use?
2. Should we support multiple providers?

## Plan
1. **Create auth module**: Set up the authentication structure
2. **Add user model**: Define user schema with credentials
3. **Implement login**: Create login endpoint

## Risks
- May need to update existing user references
- Session management needs consideration
```

## LLM Integration

### Client (`internal/llm/`)

Wraps the Ollama API with streaming support and resilience:

```go
type LLMClient interface {
    Chat(ctx context.Context, messages []Message, tools []ToolDefinition, system string) (*Response, error)
    ChatStream(ctx context.Context, messages []Message, tools []ToolDefinition, system string) <-chan StreamChunk
    SetModel(model string)
    SetTier(tier config.ModelTier)
    GetModel() string
    Close() error
}
```

The `ResilientClient` wraps any `LLMClient` with retry logic and a circuit breaker for fault tolerance.

### Model Tiers

```go
const (
    TierFast   = "fast"   // qwen2.5-coder:3b - quick responses
    TierSmart  = "smart"  // qwen2.5-coder:7b - balanced
    TierGenius = "genius" // qwen2.5-coder:14b - most capable
)
```

### Streaming

Responses are streamed for real-time display:

```go
type StreamChunk struct {
    Type      string     // "text", "thinking", "tool_call", "done", "error"
    Text      string
    ToolCall  *ToolCall
    Error     error
}
```

## Agent Modes

vecai supports three agent modes that control tool access and behavior:

| Mode | Tools | Tier | Use Case |
|------|-------|------|----------|
| **Ask** | Read-only | Fast | Explore and understand code |
| **Plan** | Read + confirm writes | Smart | Analyze and plan changes |
| **Build** | All tools | Smart+ | Implement changes |

Modes can be switched via Shift+Tab in the TUI or auto-selected based on query intent.

## Message Flow

### Conversation Structure

```go
type Message struct {
    Role    string  // "user" or "assistant"
    Content string
}
```

### Typical Flow

1. User sends query
2. Agent adds as user message
3. LLM responds with text and/or tool calls
4. If tool calls: execute and add results as user message
5. Loop until no more tool calls
6. Final text response displayed to user

## System Prompt

The agent operates with a system prompt that defines:

- Available tools and their purposes
- Guidelines for tool selection (vecgrep_search first)
- Response formatting expectations
- Mode-specific instructions (Ask/Plan/Build)

## Extension Points

### Adding New Tools

1. Create a struct implementing `Tool` interface
2. Register in `NewRegistry()`:

```go
func NewRegistry(cfg *config.ToolsConfig) *Registry {
    r := &Registry{tools: make(map[string]Tool)}
    r.Register(&MyNewTool{})
    return r
}
```

### Adding Skills

1. Create a markdown file in `skills/` directory
2. Add YAML frontmatter with triggers
3. Skills auto-load on startup

### Custom Permission Modes

Implement `InputHandler` and `OutputHandler` interfaces for custom permission prompts.

## Error Handling

- Tool execution errors are returned to the LLM for self-correction
- Permission denials are communicated back to allow alternative approaches
- Max iterations prevent infinite loops (configurable, default: 20)
- Stream errors are surfaced to the user
- Structured errors (`internal/errors/`) with Category, Code, and Retryable fields

## Concurrency

- Tool registry uses `sync.RWMutex` for thread-safe access
- Permission cache uses `sync.RWMutex` for concurrent reads
- Streaming uses channels for async communication
- Parallel tool execution (configurable, default: 4 concurrent)

## Configuration

See `internal/config/config.go` for:

- Model selection and tiers
- Max tokens and temperature
- Context window management
- Skills directory
- Tool group enable/disable
- Memory layer settings
- Rate limiting and resilience
