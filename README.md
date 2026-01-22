# vecai

AI-powered codebase assistant that combines semantic search with Claude's intelligence to help you understand, navigate, and modify code.

## Features

- **Semantic Code Search** - Find code by meaning, not just keywords (via vecgrep)
- **Multiple Model Tiers** - Choose between Haiku (fast), Sonnet (smart), or Opus (genius)
- **Plan Mode** - Break down complex tasks into steps with interactive questionnaires
- **Permission System** - Control what the AI can read, write, or execute
- **Skills** - Customizable prompts for common tasks like code review

## Quick Start

```bash
# Set your API key
export ANTHROPIC_API_KEY="your-key"

# Run a query
vecai "explain how authentication works in this codebase"

# Start interactive mode
vecai
```

## Installation

### Prerequisites

- Go 1.21 or later
- [vecgrep](https://github.com/abdul-hamid-achik/vecgrep) (optional, for semantic search)
- Anthropic API key

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
# vecai version 0.1.0
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

Start a conversation:

```bash
vecai
```

Then use these commands:

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/mode fast` | Switch to Haiku (faster, cheaper) |
| `/mode smart` | Switch to Sonnet (balanced) |
| `/mode genius` | Switch to Opus (most capable) |
| `/plan <goal>` | Enter plan mode |
| `/skills` | List available skills |
| `/status` | Check vecgrep index status |
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

### Permission Modes

Control tool execution:

```bash
# Default: prompt for writes/executes, auto-approve reads
vecai "refactor this function"

# Auto-approve everything (use with caution)
vecai --auto "fix all lint errors"

# Prompt for everything including reads
vecai --strict "review security"
```

## Configuration

vecai looks for configuration in this order:
1. `./vecai.yaml`
2. `./.vecai/config.yaml`
3. `~/.config/vecai/config.yaml`

### Example Configuration

```yaml
# Model tier: fast (Haiku), smart (Sonnet), genius (Opus)
default_tier: smart

# Maximum tokens in response
max_tokens: 8192

# Temperature for generation (0.0-1.0)
temperature: 0.7

# Directory containing skill files
skills_dir: skills

# Path to vecgrep binary
vecgrep_path: vecgrep
```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ANTHROPIC_API_KEY` | Yes | Your Anthropic API key |

## Tools

vecai can use these tools to interact with your codebase:

| Tool | Permission | Description |
|------|------------|-------------|
| `vecgrep_search` | Read | Semantic code search |
| `vecgrep_status` | Read | Check search index status |
| `read_file` | Read | Read file contents |
| `list_files` | Read | List directory contents |
| `grep` | Read | Pattern search (ripgrep) |
| `write_file` | Write | Create or overwrite files |
| `edit_file` | Write | Make targeted edits |
| `bash` | Execute | Run shell commands |

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

For detailed skill development documentation, see [docs/SKILLS.md](docs/SKILLS.md).

## Model Tiers

| Tier | Model | Best For |
|------|-------|----------|
| `fast` | Claude Haiku | Quick questions, simple tasks |
| `smart` | Claude Sonnet | Most tasks, good balance |
| `genius` | Claude Opus | Complex reasoning, architecture |

Switch tiers in interactive mode:
```
/mode fast
/mode smart
/mode genius
```

## Semantic Search Setup

For semantic search capabilities, install and initialize vecgrep:

```bash
# Install vecgrep
go install github.com/abdul-hamid-achik/vecgrep@latest

# Initialize in your project
vecgrep init
vecgrep index

# Verify
vecai /status
```

Without vecgrep, vecai still works but uses pattern-based search only.

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
vecai --auto "fix all TypeScript type errors"
```

### Code Review

```bash
vecai "review the authentication module for security issues"
# Or trigger the skill automatically:
vecai "review this code"
```

### Plan Complex Tasks

```bash
vecai plan "migrate from REST to GraphQL"
vecai plan "add comprehensive test coverage"
vecai plan "implement caching layer"
```

## Troubleshooting

### "ANTHROPIC_API_KEY environment variable is required"

Set your API key:
```bash
export ANTHROPIC_API_KEY="sk-ant-..."
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
│   ├── agent/          # Core agent logic
│   ├── config/         # Configuration
│   ├── llm/            # Anthropic SDK wrapper
│   ├── permissions/    # Permission system
│   ├── skills/         # Skill loader
│   ├── tools/          # Tool implementations
│   └── ui/             # Terminal UI
├── skills/             # Built-in skills
├── Taskfile.yml        # Build tasks
└── go.mod              # Dependencies
```

## License

MIT

## Contributing

Contributions welcome! Please open an issue or pull request.
