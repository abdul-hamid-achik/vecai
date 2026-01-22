---
name: technical-spec
description: Create a technical specification document
triggers:
  - "tech spec"
  - "technical spec"
  - "specification"
  - "/write a (tech|technical) spec/"
tags:
  - documentation
  - planning
---

# Technical Specification Instructions

You are creating a technical specification document. Follow this structure:

## Document Structure

### 1. Overview
- **Problem Statement**: What problem are we solving?
- **Proposed Solution**: High-level description of the solution
- **Goals**: What success looks like
- **Non-Goals**: What's explicitly out of scope

### 2. Background
- Context needed to understand the problem
- Relevant existing systems or code
- Previous attempts and why they didn't work

### 3. Detailed Design

#### 3.1 Architecture
- System components and their interactions
- Data flow diagrams (describe in text)
- API contracts

#### 3.2 Data Model
- New or modified data structures
- Database schema changes
- Data migration strategy

#### 3.3 Implementation Plan
- Phases of implementation
- Dependencies between components
- Risk areas and mitigation

### 4. Alternatives Considered
- Other approaches that were evaluated
- Pros/cons of each
- Why the chosen approach is preferred

### 5. Testing Strategy
- Unit test coverage
- Integration test scenarios
- Performance testing needs

### 6. Rollout Plan
- Feature flags
- Gradual rollout strategy
- Rollback plan

### 7. Open Questions
- Decisions that need input
- Areas of uncertainty

## Process

1. Use `vecgrep_search` to understand existing architecture
2. Use `read_file` to examine relevant code
3. Ask clarifying questions if needed
4. Create the specification following the structure above

Be thorough but concise. Use bullet points and short paragraphs.
