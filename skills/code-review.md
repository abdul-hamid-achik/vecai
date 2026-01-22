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

1. **Correctness**: Does the code do what it's supposed to do?
2. **Security**: Are there any security vulnerabilities?
3. **Performance**: Are there obvious performance issues?
4. **Readability**: Is the code easy to understand?
5. **Maintainability**: Will this code be easy to modify in the future?
6. **Error Handling**: Are errors handled appropriately?
7. **Testing**: Is the code testable? Are edge cases considered?

## Review Process

1. First, use `vecgrep_search` to understand the context and find relevant code
2. Use `read_file` to examine the specific files or changes
3. Look for patterns and anti-patterns
4. Identify potential bugs or issues
5. Suggest improvements with concrete examples

## Output Format

Organize your review into sections:

### Summary
Brief overview of what was reviewed.

### Issues Found
List any bugs, security issues, or problems. Rate severity (Critical/High/Medium/Low).

### Suggestions
Recommendations for improvement.

### Positive Notes
What's done well (important for team morale).

Be constructive and specific. Provide code examples when suggesting changes.
