package memory

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// SessionMemory tracks context for the current conversation
type SessionMemory struct {
	CurrentGoal  string
	FilesTouched map[string]bool
	Decisions    []Decision
	ErrorsHit    []ErrorRecord
	StartTime    time.Time
	LastActivity time.Time
	mu           sync.RWMutex
}

// Decision represents a decision made during the session
type Decision struct {
	Description string
	Rationale   string
	Timestamp   time.Time
}

// ErrorRecord represents an error encountered during the session
type ErrorRecord struct {
	Error     string
	Context   string
	Timestamp time.Time
	Resolved  bool
}

// NewSessionMemory creates a new session memory
func NewSessionMemory() *SessionMemory {
	now := time.Now()
	return &SessionMemory{
		FilesTouched: make(map[string]bool),
		StartTime:    now,
		LastActivity: now,
	}
}

// SetGoal sets the current goal for the session
func (s *SessionMemory) SetGoal(goal string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentGoal = goal
	s.LastActivity = time.Now()
}

// GetGoal returns the current goal
func (s *SessionMemory) GetGoal() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentGoal
}

// TouchFile records that a file was accessed or modified
func (s *SessionMemory) TouchFile(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FilesTouched[path] = true
	s.LastActivity = time.Now()
}

// GetTouchedFiles returns all files touched in this session
func (s *SessionMemory) GetTouchedFiles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files := make([]string, 0, len(s.FilesTouched))
	for f := range s.FilesTouched {
		files = append(files, f)
	}
	return files
}

// AddDecision records a decision made during the session
func (s *SessionMemory) AddDecision(description, rationale string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Decisions = append(s.Decisions, Decision{
		Description: description,
		Rationale:   rationale,
		Timestamp:   time.Now(),
	})
	s.LastActivity = time.Now()
}

// GetDecisions returns all decisions made in this session
func (s *SessionMemory) GetDecisions() []Decision {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy
	decisions := make([]Decision, len(s.Decisions))
	copy(decisions, s.Decisions)
	return decisions
}

// RecordError records an error that occurred
func (s *SessionMemory) RecordError(err, context string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ErrorsHit = append(s.ErrorsHit, ErrorRecord{
		Error:     err,
		Context:   context,
		Timestamp: time.Now(),
		Resolved:  false,
	})
	s.LastActivity = time.Now()
}

// MarkErrorResolved marks the most recent matching error as resolved
func (s *SessionMemory) MarkErrorResolved(errorSubstring string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find most recent matching error
	for i := len(s.ErrorsHit) - 1; i >= 0; i-- {
		if strings.Contains(s.ErrorsHit[i].Error, errorSubstring) {
			s.ErrorsHit[i].Resolved = true
			break
		}
	}
}

// GetUnresolvedErrors returns errors that haven't been resolved
func (s *SessionMemory) GetUnresolvedErrors() []ErrorRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var unresolved []ErrorRecord
	for _, e := range s.ErrorsHit {
		if !e.Resolved {
			unresolved = append(unresolved, e)
		}
	}
	return unresolved
}

// GetContextSummary returns a summary of the current session context
func (s *SessionMemory) GetContextSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder

	if s.CurrentGoal != "" {
		sb.WriteString(fmt.Sprintf("Current goal: %s\n", s.CurrentGoal))
	}

	if len(s.FilesTouched) > 0 {
		sb.WriteString(fmt.Sprintf("Files touched: %d\n", len(s.FilesTouched)))
		// List up to 5 most recently touched
		count := 0
		for f := range s.FilesTouched {
			sb.WriteString(fmt.Sprintf("  - %s\n", f))
			count++
			if count >= 5 {
				remaining := len(s.FilesTouched) - 5
				if remaining > 0 {
					sb.WriteString(fmt.Sprintf("  ... and %d more\n", remaining))
				}
				break
			}
		}
	}

	if len(s.Decisions) > 0 {
		sb.WriteString(fmt.Sprintf("Decisions made: %d\n", len(s.Decisions)))
		// Show last 3 decisions
		start := len(s.Decisions) - 3
		if start < 0 {
			start = 0
		}
		for _, d := range s.Decisions[start:] {
			sb.WriteString(fmt.Sprintf("  - %s\n", d.Description))
		}
	}

	// Count unresolved errors inline to avoid recursive RLock deadlock
	// (GetUnresolvedErrors would try to acquire RLock while we already hold it)
	unresolvedCount := 0
	for _, e := range s.ErrorsHit {
		if !e.Resolved {
			unresolvedCount++
		}
	}
	if unresolvedCount > 0 {
		sb.WriteString(fmt.Sprintf("Unresolved errors: %d\n", unresolvedCount))
	}

	return sb.String()
}

// Clear resets the session memory
func (s *SessionMemory) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.CurrentGoal = ""
	s.FilesTouched = make(map[string]bool)
	s.Decisions = nil
	s.ErrorsHit = nil
	s.LastActivity = time.Now()
}

// Duration returns how long the session has been active
func (s *SessionMemory) Duration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.StartTime)
}

// IdleTime returns time since last activity
func (s *SessionMemory) IdleTime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.LastActivity)
}
