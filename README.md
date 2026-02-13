# vecai

AI-powered codebase assistant that combines semantic search with local LLM intelligence to help you understand, navigate, and modify code.

## What is vecai?

vecai is a terminal-based AI coding assistant that runs entirely on your machine using [Ollama](https://ollama.ai). It connects to your local LLM to help you explore, understand, and modify codebases through natural language. Unlike cloud-based tools, your code never leaves your machine.

Key capabilities:
- **Semantic code search** via [vecgrep](https://github.com/abdul-hamid-achik/vecgrep) embeddings
- **Autonomous tool use** -- the agent reads files, searches code, and runs commands on your behalf
- **Three agent modes** -- Ask (read-only), Plan (analysis), Build (full access)
- **Smart tier selection** -- automatically picks the right model size for each query

## Features

- **Semantic Code Search** - Find code by meaning, not just keywords (via vecgrep)
- **Local LLM** - Runs entirely on Ollama, no cloud API required
- **Multiple Model Tiers** - Choose between fast, smart, or genius modes
- **Smart Tier Selection** - Automatically selects optimal model based on query complexity
- **Quick Mode** - Instant responses without tools for simple questions
- **Rich TUI** - Full-featured terminal interface with input queue and visual feedback
- **Plan Mode** - Break down complex tasks into steps with interactive planning
- **Session Management** - Save, resume, and manage conversation sessions
- **Memory Layer** - Unified memory system with session tracking, corrections, and learning
- **Memory Integration** - Persistent memory via noted for preferences and context
- **Auto-Learning** - Automatically extracts and remembers patterns from conversations
- **Permission System** - Control what the AI can read, write, or execute
- **Skills** - Customizable prompts for common tasks like code review
- **Analysis Mode** - Token-efficient read-only mode for code reviews
- **Context Management** - Auto-compaction to handle long conversations
- **Capture Mode** - Save responses to notes for future reference

## Prerequisites

- [Go](https://go.dev) 1.24+ (for building from source)
- [Ollama](https://ollama.ai) - Local LLM server (required)
- [vecgrep](https://github.com/abdul-hamid-achik/vecgrep) - Semantic code search (optional)
- [gpeek](https://github.com/abdul-hamid-achik/gpeek) - Git visualization (optional)
- [noted](https://github.com/abdul-hamid-achik/noted) - Persistent memory (optional)

## Quick Start

```bash
# Start Ollama
ollama serve

# Pull all required models
vecai models pull

# Run a query
vecai "explain how authentication works in this codebase"

# Quick mode for instant answers
vecai -q "what is 2+2"

# Start interactive mode
vecai
```

## Installation

### Homebrew (macOS and Linux)

```bash
brew tap abdul-hamid-achik/tap
brew install vecai
```

### Go Install

```bash
go install github.com/abdul-hamid-achik/vecai/cmd/vecai@latest
```

### Download Binary

Pre-built binaries for macOS, Linux, and Windows are available on the [Releases](https://github.com/abdul-hamid-achik/vecai/releases) page.

### From Source

```bash
git clone https://github.com/abdul-hamid-achik/vecai.git
cd vecai
go build -o vecai ./cmd/vecai

# Or use task runner
task build
```

### Verify Installation

```bash
vecai --version
```

## Usage

### One-Shot Mode

Ask a single question and get an answer:

```bash
vecai "what does the main function do?"
vecai "find all API endpoints"
vecai "explain the error handling in this project"
```

### Quick Mode

Get instant responses without tools for simple questions:

```bash
vecai -q "what is a goroutine?"
vecai --quick "explain REST vs GraphQL"
```

Quick mode:
- Uses the fast model tier (qwen2.5-coder:3b)
- No tools, no history, minimal prompt
- Ideal for quick factual questions

### Headless / Pipe Mode

Run queries without the TUI, ideal for scripting and CI:

```bash
# Prompt flag
vecai -p "list all exported functions"

# JSON output
vecai -p "summarize this file" --json

# Pipe stdin
cat main.go | vecai -p "review this code"
echo "explain goroutines" | vecai -p ""
```

### Capture Mode

Save AI responses to persistent memory:

```bash
vecai -c "what's the best way to handle errors in Go?"
# After response: "Save to notes? [y/N/e(dit)]"
```

Capture mode prompts to save responses via noted for future reference.

### Interactive Mode

Start a conversation with the full TUI:

```bash
vecai
```

Interactive commands:

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/mode fast` | Switch to fast model (qwen2.5-coder:3b) |
| `/mode smart` | Switch to smart model (qwen2.5-coder:7b) |
| `/mode genius` | Switch to genius model (qwen2.5-coder:14b) |
| `/plan <goal>` | Enter plan mode |
| `/skills` | List available skills |
| `/status` | Check vecgrep index status |
| `/reindex` | Update vecgrep search index |
| `/context` | Show context usage breakdown |
| `/compact [focus]` | Compact conversation history |
| `/sessions` | List saved sessions |
| `/resume [id]` | Resume a previous session |
| `/new` | Start a new session |
| `/delete <id>` | Delete a session |
| `/copy` | Copy conversation to clipboard |
| `/clear` | Clear conversation history |
| `/exit` | Exit interactive mode |

### Plan Mode

Break down complex tasks:

```bash
vecai plan "add user authentication"
```

Plan mode will:
1. Analyze your codebase
2. Ask clarifying questions
3. Create a step-by-step plan
4. Execute with your approval

### Analysis Mode

Token-efficient read-only mode for code reviews:

```bash
vecai --analyze "review the authentication module"
vecai -a "check for security issues"
```

Analysis mode:
- Uses only read-only tools (no writes or executes)
- Optimized prompts for lower token usage
- Aggressive context compaction
- Ideal for large codebase reviews

### Permission Modes

Control tool execution:

```bash
# Default: prompt for writes/executes, auto-approve reads
vecai "refactor this function"

# Auto-approve everything (use with caution)
vecai --auto "fix all lint errors"

# Prompt for everything including reads
vecai --strict "review security"

# Analysis mode: read-only, block all writes
vecai --analyze "review this code"
```

## Configuration

vecai looks for configuration in this order:
1. `./vecai.yaml`
2. `./.vecai/config.yaml`
3. `~/.config/vecai/config.yaml`

### Example Configuration

```yaml
# Provider (currently only ollama is supported)
provider: ollama

# Ollama settings
ollama:
  base_url: "http://localhost:11434"
  model_fast: "qwen2.5-coder:3b"
  model_smart: "qwen2.5-coder:7b"
  model_genius: "qwen2.5-coder:14b"
  keep_alive: "10m"

# Default model tier: fast, smart, or genius
default_tier: fast

# Maximum tokens in response
max_tokens: 8192

# Temperature for generation (0.0-1.0)
temperature: 0.7

# Directory containing skill files
skills_dir: skills

# Path to vecgrep binary
vecgrep_path: vecgrep

# Context management
context:
  auto_compact_threshold: 0.70    # Auto-compact at 70% context usage
  warn_threshold: 0.50            # Warn at 50% context usage
  preserve_last: 2                # Keep last 2 messages when compacting
  enable_auto_compact: true
  context_window: 32768           # Token limit (capped per-model via ModelContextWindows)

# Token-efficient analysis mode
analysis:
  enabled: false
  max_file_tokens: 2000
  aggressive_compaction: true
  smart_tool_selection: true

# Memory layer settings
memory:
  enabled: true
  project_dir: ".vecai/memory"
  global_dir: "~/.config/vecai/memory"
  store_max_entries: 10000
  store_max_disk_mb: 10

# Tool configuration
tools:
  vecgrep:
    enabled: true
    default_mode: "hybrid"    # hybrid, semantic, or keyword
    default_limit: 10
  noted:
    enabled: true
    include_in_context: true  # Include notes in prompt context
    max_context_notes: 5
  gpeek:
    enabled: true
  sandbox:
    enabled: true
    allow_net: false
```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OLLAMA_HOST` | No | Ollama server URL (overrides config) |
| `VECAI_DEBUG` | No | Set to "1" to enable full debug tracing |
| `VECAI_DEBUG_DIR` | No | Override debug trace directory (default: `/tmp/vecai-debug`) |
| `VECAI_DEBUG_LLM` | No | Set to "1" to log full LLM request/response payloads |
| `VECAI_LOG_LEVEL` | No | Console log level: `debug`, `info`, `warn`, `error` (default: `info`) |
| `TAVILY_API_KEY` | No | API key for web search tool |

## CLI Flags

| Flag | Description |
|------|-------------|
| `-p, --prompt <text>` | Headless mode: run prompt without TUI (pipe-friendly) |
| `--json` | Output JSON instead of plain text (use with -p) |
| `-q, --quick` | Quick mode: fast response, no tools |
| `-c, --capture` | Capture mode: prompt to save responses to notes |
| `--model <name>` | Override model (e.g., "qwen2.5-coder:7b") |
| `--ollama-url <url>` | Override Ollama URL (default: http://localhost:11434) |
| `--auto` | Auto-approve all tool executions |
| `--strict` | Prompt for all tool executions (including reads) |
| `--analyze, -a` | Token-efficient analysis mode (read-only) |
| `--debug, -d` | Enable full debug tracing to /tmp/vecai-debug/ |
| `--verbose, -V` | Enable verbose logging (debug level without full tracing) |
| `-v, --version` | Show version |
| `-h, --help` | Show help |

## Subcommands

### Models Management

Manage Ollama models used by vecai:

```bash
# Show configured model tiers and local availability
vecai models list

# Benchmark each model tier
vecai models test

# Pull all configured models from Ollama
vecai models pull
```

## Tools

vecai can use these tools to interact with your codebase:

### Core Tools

| Tool | Permission | Description |
|------|------------|-------------|
| `read_file` | Read | Read file contents |
| `list_files` | Read | List directory contents |
| `grep` | Read | Pattern search (ripgrep) |
| `write_file` | Write | Create or overwrite files |
| `edit_file` | Write | Make targeted edits |
| `bash` | Execute | Run shell commands (sandboxed) |

### Go-Specific Tools

| Tool | Permission | Description |
|------|------------|-------------|
| `ast_parse` | Read | Parse and analyze Go AST |
| `lsp_query` | Read | Go language server queries |
| `linter` | Read | Run golangci-lint |
| `test_runner` | Execute | Run Go tests |

### Semantic Search (vecgrep)

| Tool | Permission | Description |
|------|------------|-------------|
| `vecgrep_search` | Read | Semantic, keyword, or hybrid code search with rich filtering |
| `vecgrep_similar` | Read | Find similar code patterns by snippet, location, or chunk ID |
| `vecgrep_status` | Read | Check search index status |
| `vecgrep_index` | Write | Index or re-index files for semantic search |
| `vecgrep_clean` | Write | Remove orphaned data and optimize the database |
| `vecgrep_delete` | Write | Delete a file from the search index |
| `vecgrep_init` | Write | Initialize vecgrep in the current project |

**Search Modes:**
- `hybrid` - Combines vector similarity with keyword matching (default)
- `semantic` - Pure vector similarity search for natural language queries
- `keyword` - Text-based search using pattern matching

**Filtering Options:**
- Filter by language(s), chunk type(s), file pattern, directory, or line range
- Use `--explain` mode for search diagnostics

### Git Visualization (gpeek)

| Tool | Permission | Description |
|------|------------|-------------|
| `gpeek_status` | Read | Repository status |
| `gpeek_diff` | Read | Structured diffs with hunks |
| `gpeek_log` | Read | Commit history with filters |
| `gpeek_summary` | Read | Complete repo snapshot |
| `gpeek_blame` | Read | Line-by-line attribution |
| `gpeek_branches` | Read | List branches |
| `gpeek_stashes` | Read | List stashes |
| `gpeek_tags` | Read | List tags |
| `gpeek_changes_between` | Read | Changes between refs |
| `gpeek_conflict_check` | Read | Predict merge conflicts |

### Web Search

| Tool | Permission | Description |
|------|------------|-------------|
| `web_search` | Read | Search the web (requires Tavily API key) |

### Memory (noted)

| Tool | Permission | Description |
|------|------------|-------------|
| `noted_remember` | Write | Store memories with tags and importance |
| `noted_recall` | Read | Search memories semantically |
| `noted_forget` | Write | Delete memories by ID, tags, or age |

Memory tools are available when [noted](https://github.com/abdul-hamid-achik/noted) is installed.

## Skills

Skills are reusable prompts for common tasks. They trigger automatically based on keywords or regex patterns in your query.

### Skill Priority

Skills are matched against the user's query in this order:
1. **Regex triggers** (wrapped in `/`) are matched first using `regexp.MatchString`
2. **Plain text triggers** are matched via case-insensitive substring search

The first matching skill wins. If multiple skills could match, the one loaded first takes priority. Skills are loaded from directories in this order:

1. `./skills/` (project-local)
2. `./.vecai/skills/` (project config)
3. `~/.config/vecai/skills/` (user global)

Project-local skills override global skills with the same name.

### Built-in Skills

| Skill | Triggers | Description |
|-------|----------|-------------|
| `code-review` | "review" | Thorough code review with security and quality checks |
| `technical-spec` | "spec", "specification" | Create technical specification documents |
| `debug` | "debug", code+error patterns | Systematic debugging workflow |
| `explain` | code+explain patterns | Code explanation at multiple detail levels |
| `document` | code+document patterns | Generate documentation for code |
| `refactor` | "refactor" | Code refactoring guidance |
| `test` | "test" | Test writing assistance |
| `security` | "security" | Security review and hardening |

### Creating Custom Skills

Create a markdown file in your skills directory:

```markdown
---
name: my-skill
description: What this skill does
triggers:
  - "keyword"
  - "/regex pattern/"
tags:
  - category
---

# Instructions for the AI

Your custom prompt here...
```

Triggers can be:
- Plain text (case-insensitive substring match)
- Regex patterns (wrapped in `/`)

## Model Tiers

| Tier | Default Model | Best For |
|------|---------------|----------|
| `fast` | qwen2.5-coder:3b | Quick questions, simple lookups |
| `smart` | qwen2.5-coder:7b | Most tasks, good balance |
| `genius` | qwen2.5-coder:14b | Complex reasoning, architecture |

### Smart Tier Selection

vecai automatically selects the optimal model based on query complexity:

- **Simple queries** ("where is...", "find...", "list...") -> fast tier
- **Complex queries** ("analyze...", "review...", "refactor...") -> genius tier
- **Default** -> smart tier

This happens automatically. Override with `--model` if needed.

### Managing Models

```bash
# See current configuration and availability
vecai models list

# Pull all required models
vecai models pull

# Test response times for each tier
vecai models test
```

### Switch Tiers Manually

In interactive mode:
```
/mode fast
/mode smart
/mode genius
```

Or override via CLI:
```bash
vecai --model qwen2.5-coder:14b "explain the architecture"
```

## Semantic Search Setup

For semantic search capabilities, install and initialize vecgrep:

```bash
# Install vecgrep via Homebrew
brew tap abdul-hamid-achik/tap
brew install vecgrep

# Or via Go
go install github.com/abdul-hamid-achik/vecgrep@latest

# Initialize in your project
vecgrep init
vecgrep index

# Verify
vecai /status
```

Without vecgrep, vecai still works but uses pattern-based search only.

## Git Visualization Setup

For enhanced git visualization capabilities, install gpeek:

```bash
# Install gpeek via Homebrew
brew tap abdul-hamid-achik/tap
brew install gpeek

# Or via Go
go install github.com/abdul-hamid-achik/gpeek@latest

# Verify
gpeek --version
```

With gpeek, you can ask questions like:
```bash
vecai "what's the git status?"
vecai "show me recent commits"
vecai "what changed between main and this branch?"
vecai "will merging feature-branch cause conflicts?"
```

Without gpeek, vecai falls back to basic `git` commands via bash.

## Memory Setup

For persistent memory capabilities, install noted:

```bash
# Install noted via Homebrew
brew tap abdul-hamid-achik/tap
brew install noted

# Or via Go
go install github.com/abdul-hamid-achik/noted@latest

# Verify
noted --version
```

With noted, you can:
```bash
# Ask vecai to remember things
vecai "remember that I prefer tabs over spaces"

# Recall memories
vecai "what are my code style preferences?"

# Use capture mode to save responses
vecai -c "explain the visitor pattern"
# Then choose to save the response
```

Without noted, memory tools are not available but vecai still works.

## Memory Layer

vecai includes a unified memory layer that aggregates multiple memory systems to provide context-aware assistance.

### Memory Components

| Component | Scope | Purpose |
|-----------|-------|---------|
| **Session Memory** | Current session | Tracks goals, files touched, decisions, and errors |
| **Project Memory** | Per-project | Stores patterns, conventions, and architecture knowledge |
| **Correction Memory** | Global | Learns from mistakes and user corrections |
| **Solution Cache** | Global | Caches successful solutions for reuse |
| **Noted Integration** | Global | External persistent memory via noted CLI |

### How It Works

1. **Context Enrichment**: When you ask a question, vecai automatically enriches the prompt with relevant memories:
   - Project patterns and conventions
   - Current session context (goals, files, decisions)
   - Relevant corrections from past mistakes
   - Notes from noted that match your query

2. **Correction Detection**: When you correct vecai (saying "no", "wrong", "actually", etc.), it records the correction for future learning.

3. **Auto-Learning**: During conversation compaction, vecai extracts learnings about:
   - Your preferences and coding style
   - Project-specific patterns
   - Successful solutions to problems

### Configuration

```yaml
memory:
  enabled: true                    # Enable/disable memory layer
  project_dir: ".vecai/memory"     # Per-project memory location
  global_dir: "~/.config/vecai/memory"  # Global memory location
  store_max_entries: 10000         # Max entries per store (0 = unlimited)
  store_max_disk_mb: 10            # Max disk per store in MB (0 = unlimited)
```

### Memory Storage

- **Project memory**: Stored in `.vecai/memory/` within your project
- **Global memory**: Stored in `~/.config/vecai/corrections/` and `~/.config/vecai/solutions/`

## Tool Configuration

You can enable/disable and configure individual tool groups:

```yaml
tools:
  vecgrep:
    enabled: true           # Enable semantic search tools
    default_mode: "hybrid"  # Default search mode
    default_limit: 10       # Default result limit

  noted:
    enabled: true           # Enable noted memory tools
    include_in_context: true  # Include notes in prompt enrichment
    max_context_notes: 5    # Max notes to include

  gpeek:
    enabled: true           # Enable git visualization tools

  sandbox:
    enabled: true           # OS-level sandbox for bash commands
    allow_net: false         # Allow network access in sandbox
```

### Disabling Tool Groups

To disable a tool group entirely:

```yaml
tools:
  gpeek:
    enabled: false  # Disables all gpeek_* tools
```

This is useful for:
- Reducing token usage (fewer tool definitions)
- Restricting capabilities in certain environments
- Speeding up startup time

## Examples

### Understand Code

```bash
vecai "how does the payment processing work?"
vecai "what design patterns are used in this codebase?"
vecai "explain the data flow from API to database"
```

### Make Changes

```bash
vecai "add input validation to the user registration endpoint"
vecai "refactor the database connection to use connection pooling"
vecai --auto "fix all lint errors"
```

### Code Review

```bash
vecai --analyze "review the authentication module for security issues"
# Or trigger the skill automatically:
vecai "review this code"
```

### Plan Complex Tasks

```bash
vecai plan "migrate from REST to GraphQL"
vecai plan "add comprehensive test coverage"
vecai plan "implement caching layer"
```

### Git Operations

```bash
vecai "what's the current git status?"
vecai "show me the last 5 commits"
vecai "who last modified the auth module?"
vecai "what changed between v1.0 and HEAD?"
vecai "will merging feature-branch into main cause conflicts?"
```

### Session Management

```bash
# Start a session (sessions auto-save)
vecai

# List saved sessions
/sessions

# Resume a previous session
/resume abc123

# Start fresh
/new
```

## Troubleshooting

### "Ollama connection failed"

Make sure Ollama is running:
```bash
ollama serve
```

And verify you have models pulled:
```bash
ollama list
vecai models pull  # Pull all required models
```

### "vecgrep is not initialized"

Initialize the search index:
```bash
vecgrep init
vecgrep index
```

### Permission Denied Errors

Use `--auto` for trusted operations or respond to permission prompts.

### Slow Responses

Use quick mode for simple questions:
```bash
vecai -q "quick question"
```

Or switch to a faster model:
```
/mode fast
```

Or use a smaller Ollama model:
```bash
vecai --model qwen2.5-coder:3b "quick question"
```

### High Memory Usage

Compact the conversation context:
```
/compact
```

Or start a new session:
```
/new
```

### Logging and Debug Mode

vecai has a unified logging system (`internal/logging`) with three output channels:

1. **Console Output** (`stderr`) - Human-readable, respects log level
2. **Session Logs** (`.vecai/logs/`) - Always captures all levels
3. **Debug Traces** (`/tmp/vecai-debug/`) - Structured JSONL events (when enabled)

All components use prefixed loggers for clear identification (e.g., `[TUI-APP]`, `[TUI-RUN]`, `[agent]`).

**Enable verbose logging** (debug level without full tracing):
```bash
vecai --verbose "your query"
# or
vecai -V "your query"
```

**Enable full debug tracing** (includes JSONL event traces):
```bash
vecai --debug "your query"
# or
VECAI_DEBUG=1 vecai "your query"
```

**Log file locations:**
- Session logs: `.vecai/logs/session_*.log` (symlink: `latest.log`)
- Debug traces: `/tmp/vecai-debug/session_*.jsonl` (symlink: `latest.jsonl`)
- LLM payloads: `/tmp/vecai-debug/llm_*.jsonl` (when `VECAI_DEBUG_LLM=1`)

**View logs:**
```bash
# View latest session log
cat .vecai/logs/latest.log

# View debug trace events
cat /tmp/vecai-debug/latest.jsonl | jq .

# Follow logs in real-time
tail -f .vecai/logs/latest.log
```

**Set console log level:**
```bash
VECAI_LOG_LEVEL=debug vecai "your query"
```

## Development

### Build

```bash
task build      # Build binary
task test       # Run tests
task lint       # Run linter
task fmt        # Format code
task vet        # Run go vet
task coverage   # Run tests with coverage
task all        # Run fmt, vet, lint, test, build
task clean      # Remove build artifacts
```

### Project Structure

```
vecai/
├── cmd/vecai/          # CLI entry point
├── internal/
│   ├── agent/          # Core agent logic and planning
│   ├── config/         # Configuration management
│   ├── context/        # Context and token management
│   ├── errors/         # Structured error types
│   ├── llm/            # Ollama client with resilience
│   ├── logging/        # Unified logging system (console, file, JSONL tracing)
│   ├── memory/         # Persistent memory layer
│   ├── permissions/    # Permission system
│   ├── session/        # Session persistence
│   ├── skills/         # Skill loader
│   ├── tools/          # Tool implementations
│   ├── tui/            # Terminal UI (BubbleTea)
│   └── ui/             # Terminal output helpers
├── skills/             # Built-in skills
├── docs/adr/           # Architecture Decision Records
├── Taskfile.yml        # Build tasks
└── go.mod              # Dependencies
```

## License

MIT

## Contributing

Contributions welcome! Please open an issue or pull request.
