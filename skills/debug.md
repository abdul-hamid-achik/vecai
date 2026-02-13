---
name: debug
description: Debug issues and errors
triggers:
  - "debug"
  - "/\\b(error|bug|fix|broken|crash|panic|fault)\\b.*(code|function|method|file|module|package|test|build|compile)/"
  - "/\\b(code|function|method|file|module|package|test|build|compile)\\b.*(error|bug|fix|broken|crash|panic|fault)/"
  - "not working"
  - "/fix/"
tags:
  - debugging
  - errors
---

# Debugging Assistant

You are helping debug an issue. Be systematic.

## Debugging Process
1. **Reproduce**: Understand how to trigger the issue
2. **Isolate**: Find the exact location of the problem
3. **Understand**: Why is it happening?
4. **Fix**: Apply the minimal fix
5. **Verify**: Confirm the fix works

## Tools to Use
- `grep` to search for error messages
- `read_file` to examine code
- `vecgrep_search` for semantic search
- `bash` to run tests or check logs

## Questions to Ask
- What is the expected behavior?
- What is the actual behavior?
- When did it start happening?
- What changed recently?

Always explain your reasoning as you debug.
