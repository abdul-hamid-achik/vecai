package memory

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/logging"
)

// MemoryLayer provides unified access to all memory stores
type MemoryLayer struct {
	Session     *SessionMemory
	Project     *ProjectMemory
	Corrections *CorrectionMemory
	Solutions   *SolutionCache
	notedAvail  bool
}

// NewMemoryLayer creates a new memory layer for the given project path
func NewMemoryLayer(projectPath string) (*MemoryLayer, error) {
	layer := &MemoryLayer{
		Session: NewSessionMemory(),
	}

	// Initialize project memory
	projectMem, err := NewProjectMemory(projectPath)
	if err != nil {
		logWarn("Failed to initialize project memory: %v", err)
	} else {
		layer.Project = projectMem
	}

	// Initialize correction memory
	corrMem, err := NewCorrectionMemory()
	if err != nil {
		logWarn("Failed to initialize correction memory: %v", err)
	} else {
		layer.Corrections = corrMem
	}

	// Initialize solution cache
	solCache, err := NewSolutionCache()
	if err != nil {
		logWarn("Failed to initialize solution cache: %v", err)
	} else {
		layer.Solutions = solCache
	}

	// Check if noted binary is available
	if _, err := exec.LookPath("noted"); err == nil {
		layer.notedAvail = true
	}

	return layer, nil
}

// GetContextEnrichment returns formatted memory context for inclusion in prompts
func (m *MemoryLayer) GetContextEnrichment(query string) string {
	var sections []string

	// Get project summary if available
	if m.Project != nil {
		if summary := m.Project.GetProjectSummary(); summary != "" {
			sections = append(sections, "## Project Knowledge\n\n"+summary)
		}
	}

	// Get session context if available
	if m.Session != nil {
		if summary := m.Session.GetContextSummary(); summary != "" {
			sections = append(sections, "## Current Session\n\n"+summary)
		}
	}

	// Get relevant corrections if available
	if m.Corrections != nil {
		corrections := m.Corrections.FindRelevant("", query)
		if formatted := m.Corrections.FormatForPrompt(corrections); formatted != "" {
			sections = append(sections, formatted)
		}
	}

	// Get notes from noted if available
	if m.notedAvail && query != "" {
		if notes := m.recallFromNoted(query); notes != "" {
			sections = append(sections, "## Relevant Notes\n\n"+notes)
		}
	}

	if len(sections) == 0 {
		return ""
	}

	return strings.Join(sections, "\n\n")
}

// recallFromNoted queries the noted CLI for relevant memories
func (m *MemoryLayer) recallFromNoted(query string) string {
	cmd := exec.Command("noted", "recall", query, "--limit", "5", "--format", "text")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Silently fail - noted might have no matching memories
		return ""
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" || strings.Contains(result, "No memories found") {
		return ""
	}

	return result
}

// RecordFileAccess records that a file was accessed in the session
func (m *MemoryLayer) RecordFileAccess(path string) {
	if m.Session != nil {
		m.Session.TouchFile(path)
	}
}

// RecordDecision records a decision made during the session
func (m *MemoryLayer) RecordDecision(description, rationale string) {
	if m.Session != nil {
		m.Session.AddDecision(description, rationale)
	}
}

// RecordError records an error that occurred
func (m *MemoryLayer) RecordError(err, context string) {
	if m.Session != nil {
		m.Session.RecordError(err, context)
	}
}

// LearnCorrection records a learned correction from a mistake
func (m *MemoryLayer) LearnCorrection(trigger, problem, solution, context string) error {
	if m.Corrections == nil {
		return fmt.Errorf("correction memory not available")
	}
	return m.Corrections.Learn(trigger, problem, solution, context)
}

// CacheSolution stores a successful solution for future reuse
func (m *MemoryLayer) CacheSolution(request, solution string, tags []string) error {
	if m.Solutions == nil {
		return fmt.Errorf("solution cache not available")
	}
	return m.Solutions.Cache(request, solution, tags)
}

// FindSimilarSolution finds a cached solution for a similar request
func (m *MemoryLayer) FindSimilarSolution(request string) *Solution {
	if m.Solutions == nil {
		return nil
	}
	return m.Solutions.FindSimilar(request)
}

// SetGoal sets the current goal for the session
func (m *MemoryLayer) SetGoal(goal string) {
	if m.Session != nil {
		m.Session.SetGoal(goal)
	}
}

// IsNotedAvailable returns whether the noted CLI is available
func (m *MemoryLayer) IsNotedAvailable() bool {
	return m.notedAvail
}

// Close closes all memory stores
func (m *MemoryLayer) Close() error {
	var errs []string

	if m.Project != nil {
		if err := m.Project.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("project: %v", err))
		}
	}

	if m.Corrections != nil {
		if err := m.Corrections.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("corrections: %v", err))
		}
	}

	if m.Solutions != nil {
		if err := m.Solutions.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("solutions: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("memory layer close errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// logWarn logs a warning message using the new logging package.
func logWarn(format string, args ...any) {
	if log := logging.Global(); log != nil {
		log.Warn(fmt.Sprintf(format, args...))
	}
}
