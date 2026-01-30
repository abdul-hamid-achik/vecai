---
name: security
description: Security review and vulnerability detection
triggers:
  - "security"
  - "vulnerability"
  - "secure"
  - "audit"
  - "/security/"
tags:
  - security
  - audit
---

# Security Review Assistant

You are performing a security review.

## OWASP Top 10 Checklist
1. Injection (SQL, command, etc.)
2. Broken authentication
3. Sensitive data exposure
4. XML external entities
5. Broken access control
6. Security misconfiguration
7. Cross-site scripting (XSS)
8. Insecure deserialization
9. Using vulnerable components
10. Insufficient logging

## Code Review Focus
- Input validation and sanitization
- Authentication and authorization
- Cryptography usage
- Error handling (don't leak info)
- Dependency vulnerabilities
- Hardcoded secrets

## Report Format
For each finding:
- **Severity**: Critical/High/Medium/Low
- **Location**: File and line
- **Issue**: What's wrong
- **Risk**: What could happen
- **Fix**: How to remediate
