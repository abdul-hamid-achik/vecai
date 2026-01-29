# vecai

AI-powered codebase assistant that combines semantic search with local LLM intelligence to help you understand, navigate, and modify code.

## Features

- **Semantic Code Search** - Find code by meaning, not just keywords (via vecgrep)
- **Local LLM** - Runs entirely on Ollama, no cloud API required
- **Multiple Model Tiers** - Choose between fast, smart, or genius modes
- **Rich TUI** - Full-featured terminal interface with input queue and visual feedback
- **Plan Mode** - Break down complex tasks into steps with interactive planning
- **Session Management** - Save, resume, and manage conversation sessions
- **Permission System** - Control what the AI can read, write, or execute
- **Skills** - Customizable prompts for common tasks like code review
- **Analysis Mode** - Token-efficient read-only mode for code reviews
- **Context Management** - Auto-compaction to handle long conversations

## Quick Start

```bash
# Start Ollama
ollama serve

# Pull a model
ollama pull qwen3:8b

# Run a query
vecai "explain how authentication works in this codebase"

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

### Prerequisites

- [Ollama](https://ollama.ai) - Local LLM server (required)
- [vecgrep](https://github.com/abdul-hamid-achik/vecgrep) - Semantic code search (optional)
- [gpeek](https://github.com/abdul-hamid-achik/gpeek) - Git visualization (optional)

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

### Interactive Mode

Start a conversation with the full TUI:

```bash
vecai
```

Interactive commands:

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/mode fast` | Switch to fast model (qwen3:8b) |
| `/mode smart` | Switch to smart model (qwen2.5-coder:7b) |
| `/mode genius` | Switch to genius model (cogito:14b) |
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
  model_fast: "qwen3:8b"
  model_smart: "qwen2.5-coder:7b"
  model_genius: "cogito:14b"
  keep_alive: "5m"

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
  auto_compact_threshold: 0.95    # Auto-compact at 95% context usage
  warn_threshold: 0.80            # Warn at 80% context usage
  preserve_last: 4                # Keep last 4 messages when compacting
  enable_auto_compact: true
  context_window: 32768           # Token limit for model

# Token-efficient analysis mode
analysis:
  enabled: false
  max_file_tokens: 2000
  aggressive_compaction: true
  smart_tool_selection: true
```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OLLAMA_HOST` | No | Ollama server URL (overrides config) |
| `VECAI_DEBUG` | No | Set to "1" to enable debug tracing |
| `VECAI_DEBUG_DIR` | No | Override debug log directory |
| `VECAI_DEBUG_LLM` | No | Set to "1" to log full LLM payloads |
| `TAVILY_API_KEY` | No | API key for web search tool |

## CLI Flags

| Flag | Description |
|------|-------------|
| `--model <name>` | Override model (e.g., "qwen3:8b", "cogito:14b") |
| `--ollama-url <url>` | Override Ollama URL (default: http://localhost:11434) |
| `--auto` | Auto-approve all tool executions |
| `--strict` | Prompt for all tool executions (including reads) |
| `--analyze, -a` | Token-efficient analysis mode (read-only) |
| `-v, --version` | Show version |
| `-h, --help` | Show help |

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
| `bash` | Execute | Run shell commands |

### Go-Specific Tools

| Tool | Permission | Description |
|------|------------|-------------|
| `ast` | Read | Parse and analyze Go AST |
| `lsp` | Read | Go language server queries |
| `linter` | Read | Run golangci-lint |
| `test` | Execute | Run Go tests |

### Semantic Search (vecgrep)

| Tool | Permission | Description |
|------|------------|-------------|
| `vecgrep_search` | Read | Semantic code search |
| `vecgrep_status` | Read | Check search index status |

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

## Skills

Skills are reusable prompts for common tasks. They trigger automatically based on keywords.

### Built-in Skills

- **code-review** - Thorough code review with security and quality checks
- **technical-spec** - Create technical specification documents

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
- Plain text (case-insensitive match)
- Regex patterns (wrapped in `/`)

### Skill Locations

Skills are loaded from:
1. `./skills/`
2. `./.vecai/skills/`
3. `~/.config/vecai/skills/`

## Model Tiers

| Tier | Default Model | Best For |
|------|---------------|----------|
| `fast` | qwen3:8b | Quick questions, simple tasks |
| `smart` | qwen2.5-coder:7b | Most tasks, good balance |
| `genius` | cogito:14b | Complex reasoning, architecture |

Switch tiers in interactive mode:
```
/mode fast
/mode smart
/mode genius
```

Or override via CLI:
```bash
vecai --model cogito:14b "explain the architecture"
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

And verify you have a model pulled:
```bash
ollama list
ollama pull qwen3:8b
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

Switch to a faster model:
```
/mode fast
```

Or use a smaller Ollama model:
```bash
vecai --model qwen3:4b "quick question"
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

### Debug Mode

Enable debug logging to troubleshoot issues:
```bash
VECAI_DEBUG=1 vecai "your query"
```

Debug logs are written to `/tmp/vecai-debug/` by default.

## Development

### Build

```bash
task build      # Build binary
task test       # Run tests
task lint       # Run linter
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
│   ├── llm/            # Ollama client
│   ├── memory/         # Persistent memory layer
│   ├── permissions/    # Permission system
│   ├── session/        # Session persistence
│   ├── skills/         # Skill loader
│   ├── tools/          # Tool implementations
│   ├── tui/            # Terminal UI (BubbleTea)
│   └── ui/             # Terminal output helpers
├── skills/             # Built-in skills
├── Taskfile.yml        # Build tasks
└── go.mod              # Dependencies
```

## License

MIT

## Contributing

Contributions welcome! Please open an issue or pull request.
