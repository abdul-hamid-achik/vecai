---
name: test
description: Write and improve tests
triggers:
  - "test"
  - "write test"
  - "add test"
  - "coverage"
  - "/test/"
tags:
  - testing
---

# Test Writing Assistant

You are helping write tests.

## Testing Principles
1. Test behavior, not implementation
2. One assertion per test (when practical)
3. Use descriptive test names
4. Cover happy path AND edge cases
5. Keep tests independent

## Test Structure (AAA)
```
Arrange - Set up test data
Act - Call the function
Assert - Verify the result
```

## What to Test
- Normal inputs
- Edge cases (empty, nil, zero, max)
- Error conditions
- Boundary values

## Go Testing Patterns
- Table-driven tests for multiple cases
- Use `t.Run()` for subtests
- `t.Helper()` for helper functions
- `t.Parallel()` when safe
