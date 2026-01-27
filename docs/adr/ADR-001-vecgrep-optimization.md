# ADR-001: Optimize vecgrep Usage for Codebase Exploration

## Status

Proposed

## Context

vecai is an AI-powered codebase assistant that has access to multiple search tools:
- `vecgrep_search` - Semantic search using vector embeddings
- `grep` - Pattern-based search using ripgrep
- `list_files` / `read_file` - File system navigation
- `bash` - Shell commands

Currently, the LLM decides which tool to use based on the system prompt guidelines. However, the system prompt only provides brief guidance:

```
1. Use vecgrep_search for understanding concepts, finding related code, or exploring the codebase
2. Use grep for exact string/pattern matching
```

This leads to suboptimal tool selection:
- The LLM often defaults to `grep` or `list_files` when vecgrep would be more efficient
- No automatic indexing check before exploration tasks
- No "similar code" capability exposed for finding related patterns
- System prompt doesn't emphasize vecgrep's semantic understanding advantages

## Decision Drivers

* **Performance**: Semantic search is faster for concept-based queries than iterative grep/file reads
* **Accuracy**: Vector embeddings capture semantic meaning better than keyword matching
* **Context Efficiency**: Fewer tool calls = less context used = more room for conversation
* **User Experience**: Faster, more relevant results improve interaction quality

## Considered Options

### Option 1: Enhanced System Prompt Only
Update the system prompt with stronger vecgrep-first guidance.

**Pros:**
- Zero code changes
- Immediate effect
- Easy to iterate

**Cons:**
- LLM may still ignore guidance
- No enforcement mechanism
- Doesn't add new capabilities

### Option 2: Add vecgrep_similar Tool + Enhanced Prompting
Add a new tool for finding similar code patterns, plus improved system prompt.

**Pros:**
- New capability (find code similar to a snippet)
- Better guidance
- Moderate implementation effort

**Cons:**
- Requires code changes
- More tools = more complexity for LLM

### Option 3: Smart Tool Routing Layer
Add a pre-processing layer that analyzes queries and routes to vecgrep automatically.

**Pros:**
- Enforced vecgrep usage
- Can optimize based on query type

**Cons:**
- Complex implementation
- May override legitimate LLM decisions
- Harder to debug

### Option 4: Combined Approach (Recommended)
1. Enhanced system prompt with vecgrep-first strategy
2. Add `vecgrep_similar` tool for pattern-based exploration
3. Add startup index freshness check with auto-reindex suggestion
4. Improve tool descriptions to guide LLM better

## Decision Outcome

Chosen option: **Option 4 (Combined Approach)** because it provides immediate improvements via prompting while adding valuable new capabilities without over-engineering.

### Implementation Plan

#### Phase 1: Enhanced System Prompt (Immediate)
Update `systemPrompt` in `internal/agent/agent.go`:

```go
const systemPrompt = `You are vecai, an AI-powered codebase assistant...

## Tool Selection Strategy (IMPORTANT)

**For codebase exploration, ALWAYS prefer vecgrep_search first:**
- "Where is X implemented?" → vecgrep_search("X implementation")
- "How does Y work?" → vecgrep_search("Y logic flow")
- "Find code related to Z" → vecgrep_search("Z functionality")
- "What handles authentication?" → vecgrep_search("authentication handler")

**Only use grep when:**
- Searching for exact strings (error messages, constants)
- Finding all usages of a specific identifier
- Pattern matching with regex

**Only use list_files/read_file when:**
- You already know the exact file path
- vecgrep returned a relevant file and you need full context

## Tool Descriptions
- vecgrep_search: PREFERRED for concept/semantic queries. Understands code meaning.
- vecgrep_similar: Find code structurally similar to a pattern or location.
- grep: Exact pattern matching only. Use for literals, not concepts.
- read_file: Read specific files. Use after vecgrep identifies relevant files.
`
```

#### Phase 2: Add vecgrep_similar Tool
New tool in `internal/tools/vecgrep.go`:

```go
type VecgrepSimilarTool struct{}

func (t *VecgrepSimilarTool) Name() string { return "vecgrep_similar" }

func (t *VecgrepSimilarTool) Description() string {
    return "Find code similar to a given snippet, file location, or existing chunk. " +
           "Use this to discover related patterns, implementations, or potential duplicates."
}

func (t *VecgrepSimilarTool) InputSchema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "text": map[string]any{
                "type": "string",
                "description": "Code snippet to find similar code for",
            },
            "file_location": map[string]any{
                "type": "string",
                "description": "File:line to find similar code (e.g., 'main.go:50')",
            },
            "limit": map[string]any{
                "type": "integer",
                "description": "Max results (default: 5)",
                "default": 5,
            },
            "exclude_same_file": map[string]any{
                "type": "boolean",
                "description": "Exclude results from the same file (default: true)",
                "default": true,
            },
        },
    }
}
```

#### Phase 3: Index Freshness Check
Add startup check in `internal/agent/agent.go`:

```go
func (a *Agent) checkVecgrepFreshness() {
    // Call vecgrep status --format json
    // If stale_files > 0, suggest reindexing
    // "Index has 14 modified files. Run /reindex for best results."
}
```

Add `/reindex` command:
```go
case "/reindex":
    runner.SendInfo("Reindexing codebase...")
    // Call vecgrep index
    runner.SendSuccess("Index updated: X files processed")
```

### Consequences

**Good:**
* Faster codebase exploration via semantic search
* Reduced tool call iterations (fewer grep + read_file chains)
* Better context efficiency (more room for actual conversation)
* New capability: find similar code patterns
* Users get more relevant results faster

**Bad:**
* Requires vecgrep to be initialized (graceful fallback needed)
* Slightly more complex system prompt
* One more tool for LLM to understand

## Metrics for Success

1. **Reduced tool calls per query**: Measure average tools used for exploration tasks
2. **vecgrep usage rate**: Track % of exploration queries using vecgrep vs grep
3. **User satisfaction**: Qualitative feedback on result relevance

## Implementation Priority

| Task | Priority | Effort |
|------|----------|--------|
| Enhanced system prompt | P0 | Low |
| Add vecgrep_similar | P1 | Medium |
| /reindex command | P1 | Low |
| Index freshness check | P2 | Low |
| Usage metrics | P3 | Medium |
