# Skills Development Guide

Skills are reusable prompts that help vecai handle specific types of tasks. They automatically trigger based on keywords in your queries.

## Quick Start

Create a file in the `skills/` directory:

```markdown
---
name: my-skill
description: Brief description of what this skill does
triggers:
  - "keyword"
  - "another keyword"
tags:
  - category
---

# Instructions

Your prompt instructions here...
```

## Skill Format

Skills are markdown files with YAML frontmatter.

### Frontmatter Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | No | Skill identifier (defaults to filename without .md) |
| `description` | No | Human-readable description |
| `triggers` | Yes | Keywords or patterns that activate the skill |
| `tags` | No | Categories for organization |

### Trigger Types

**Plain Text Triggers**

Case-insensitive substring matching:

```yaml
triggers:
  - "review"       # Matches "review this code", "code review", etc.
  - "code review"  # More specific match
```

**Regex Triggers**

Wrap patterns in `/` for regex matching:

```yaml
triggers:
  - "/review (this|the|my) code/"  # Matches "review this code", "review my code"
  - "/fix(es|ing)? bug/"           # Matches "fix bug", "fixes bug", "fixing bug"
```

### Prompt Content

Everything after the frontmatter `---` is the prompt that gets prepended to the user's query.

## Example Skills

### Code Review Skill

```markdown
---
name: code-review
description: Perform a thorough code review
triggers:
  - "review"
  - "code review"
  - "/review (this|the|my)/"
tags:
  - review
  - quality
---

# Code Review Instructions

You are performing a code review. Follow these guidelines:

## What to Look For

1. **Correctness**: Does the code work as intended?
2. **Security**: Any vulnerabilities?
3. **Performance**: Any obvious issues?
4. **Readability**: Is it easy to understand?

## Output Format

### Summary
Brief overview.

### Issues Found
List problems with severity (Critical/High/Medium/Low).

### Suggestions
Recommendations for improvement.
```

### Bug Fix Skill

```markdown
---
name: bug-fix
description: Help diagnose and fix bugs
triggers:
  - "bug"
  - "fix"
  - "broken"
  - "/not working/"
tags:
  - debugging
---

# Bug Fix Assistant

Help the user diagnose and fix the issue:

1. First, understand the expected vs actual behavior
2. Use vecgrep_search to find relevant code
3. Read the specific files involved
4. Identify the root cause
5. Suggest a fix with code examples

Ask clarifying questions if the bug description is unclear.
```

### Documentation Skill

```markdown
---
name: document
description: Generate documentation for code
triggers:
  - "document"
  - "docs"
  - "README"
  - "/write (a )?readme/"
tags:
  - documentation
---

# Documentation Generator

Generate clear, helpful documentation:

1. Analyze the code structure
2. Identify public APIs and interfaces
3. Document:
   - Purpose and overview
   - Installation/setup
   - Usage examples
   - API reference
   - Configuration options

Use markdown formatting. Include code examples.
```

## Skill Directories

Skills are loaded from these locations (in order):

1. `./skills/` - Project-specific skills
2. `./.vecai/skills/` - Project config directory
3. `~/.config/vecai/skills/` - User-global skills

Later directories can override earlier ones (same name = override).

## Best Practices

### Be Specific in Triggers

```yaml
# Good - specific triggers
triggers:
  - "security review"
  - "security audit"
  - "/check.*(security|vulnerabilities)/"

# Avoid - too generic, may trigger unexpectedly
triggers:
  - "check"
  - "look"
```

### Structure Your Prompts

```markdown
# Clear Title

Brief context about what the skill does.

## Step-by-Step Process
1. First step
2. Second step
3. Third step

## Output Format
Describe expected output structure.

## Important Notes
- Key consideration
- Another consideration
```

### Use Available Tools

Reference the tools vecai can use:

```markdown
Use these tools in your process:
- `vecgrep_search` for finding relevant code by concept
- `read_file` to examine specific files
- `grep` for exact pattern matching
- `list_files` to explore directory structure
```

### Handle Edge Cases

```markdown
## If Information is Missing
Ask the user for clarification before proceeding.

## If Multiple Approaches Exist
Present options with trade-offs and let the user choose.
```

## Testing Skills

Test your skill by using its trigger words:

```bash
# Should trigger code-review skill
vecai "review the authentication module"

# Check which skill was triggered
# vecai shows "Using skill: skill-name" when a skill activates
```

## Debugging

If a skill isn't triggering:

1. Check trigger spelling matches your query
2. Verify the file has valid YAML frontmatter
3. Check file is in a skills directory
4. Use `/skills` command to list loaded skills

```bash
vecai
> /skills
# Lists all loaded skills with their triggers
```
