---
name: document
description: Generate documentation for code
triggers:
  - "/\\b(document|docstring|godoc|jsdoc)\\b.*(code|function|method|class|module|package|file|api|struct|interface|type)/"
  - "/\\b(code|function|method|class|module|package|file|api|struct|interface|type)\\b.*(document|docstring|godoc|jsdoc)/"
  - "/\\bgenerate\\b.*(docs|documentation|readme)/"
  - "/\\b(add|write|create)\\b.*(documentation|comments|docstrings)/"
tags:
  - documentation
  - generation
---

# Documentation Generator

You are helping generate documentation for code.

## Documentation Approach
1. Read the code to understand its purpose
2. Identify the public API surface
3. Document inputs, outputs, and side effects
4. Add usage examples where helpful
5. Note any prerequisites or constraints

## Go Documentation Style
- Package comments go before the `package` declaration
- Function comments start with the function name: `// FunctionName does X`
- Keep comments concise and focused on "why" not "what"
- Document exported types, functions, and constants

## What to Include
- **Purpose**: What does this code do?
- **Parameters**: What inputs are expected?
- **Returns**: What does it return, including errors?
- **Examples**: How is it typically used?
- **Edge cases**: Any caveats or limitations?

## What to Avoid
- Restating the code in English
- Over-documenting obvious getters/setters
- Stale comments that contradict the code
