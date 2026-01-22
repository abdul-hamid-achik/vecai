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
│  │  (Claude)   │    │             │    │   Tools     │     │
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
    llm         *llm.Client           // Anthropic SDK wrapper
    tools       *tools.Registry       // Available tools
    permissions *permissions.Policy   // Permission checking
    skills      *skills.Loader        // Skill prompts
    output      *ui.OutputHandler     // Console output
    input       *ui.InputHandler      // User input
    config      *config.Config        // Configuration
    messages    []llm.Message         // Conversation history
    planner     *Planner              // Plan mode handler
}
```

### Agent Loop

The agent loop (`runLoop`) executes up to 20 iterations:

```go
for i := 0; i < maxIterations; i++ {
    // 1. Get tool definitions for LLM
    toolDefs := a.getToolDefinitions()

    // 2. Stream response from Claude
    stream := a.llm.ChatStream(ctx, a.messages, toolDefs, systemPrompt)

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

| Tool | Permission | Description |
|------|------------|-------------|
| `vecgrep_search` | Read | Semantic code search via embeddings |
| `vecgrep_status` | Read | Check search index status |
| `read_file` | Read | Read file contents |
| `list_files` | Read | List directory contents |
| `grep` | Read | Pattern search (ripgrep) |
| `write_file` | Write | Create/overwrite files |
| `edit_file` | Write | Make targeted edits (find/replace) |
| `bash` | Execute | Run shell commands |

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
    ModeAsk    = 0  // Default: prompt for write/execute, auto-approve reads
    ModeAuto   = 1  // Approve everything (--auto flag)
    ModeStrict = 2  // Prompt for everything including reads
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

### Client (`internal/llm/client.go`)

Wraps the Anthropic SDK:

```go
type Client struct {
    client *anthropic.Client
    config *config.Config
    model  string
}
```

### Model Tiers

```go
const (
    TierFast   = "fast"   // Claude Haiku - quick responses
    TierSmart  = "smart"  // Claude Sonnet - balanced
    TierGenius = "genius" // Claude Opus - most capable
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

```
User: "What does the main function do?"
     │
     ▼
[User Message: "What does the main function do?"]
     │
     ▼
[LLM Response: Tool Call - read_file("cmd/vecai/main.go")]
     │
     ▼
[Execute Tool - read main.go]
     │
     ▼
[User Message: "Tool read_file result: <file contents>"]
     │
     ▼
[LLM Response: "The main function initializes the configuration..."]
     │
     ▼
Display to user
```

## System Prompt

The agent operates with a system prompt that defines:

- Available tools and their purposes
- Guidelines for tool selection
- Response formatting expectations

```go
const systemPrompt = `You are vecai, an AI-powered codebase assistant...

Guidelines:
1. Use vecgrep_search for understanding concepts
2. Use grep for exact pattern matching
3. Always read files before modifying them
...`
```

## Extension Points

### Adding New Tools

1. Create a struct implementing `Tool` interface
2. Register in `NewRegistry()`:

```go
func NewRegistry() *Registry {
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
- Max iterations (20) prevent infinite loops
- Stream errors are surfaced to the user

## Concurrency

- Tool registry uses `sync.RWMutex` for thread-safe access
- Permission cache uses `sync.RWMutex` for concurrent reads
- Streaming uses channels for async communication

## Configuration

See `internal/config/config.go` for:

- Model selection
- Max tokens
- Temperature
- Skills directory
- vecgrep path
