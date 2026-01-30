---
name: refactor
description: Refactor code with best practices
triggers:
  - "refactor"
  - "/refactor/"
  - "clean up"
  - "improve code"
tags:
  - code
  - refactoring
---

# Code Refactoring Assistant

You are helping refactor code. Follow these principles:

## Approach
1. First understand the current code structure
2. Identify code smells and issues
3. Propose specific improvements
4. Make incremental changes, testing each step

## Refactoring Patterns
- Extract functions for repeated code
- Simplify complex conditionals
- Remove dead code
- Improve naming for clarity
- Reduce function length (< 50 lines ideal)
- Single responsibility principle

## Process
1. Read the target files
2. Identify issues
3. Propose changes (explain why)
4. Apply changes incrementally
5. Verify the code still works

Never change behavior while refactoring - only structure.
