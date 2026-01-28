package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Correction represents a learned correction from past mistakes
type Correction struct {
	ID          string    `json:"id"`
	Trigger     string    `json:"trigger"`     // What triggers this correction (error pattern, context)
	Problem     string    `json:"problem"`     // What went wrong
	Solution    string    `json:"solution"`    // How to fix it
	Context     string    `json:"context"`     // When this applies
	UseCount    int       `json:"use_count"`   // How many times this has been applied
	SuccessRate float64   `json:"success_rate"` // How often it worked
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    time.Time `json:"last_used"`
}

// CorrectionMemory learns from mistakes and applies corrections
type CorrectionMemory struct {
	store *Store
}

// NewCorrectionMemory creates a new correction memory
// Uses global config directory for corrections that apply across projects
func NewCorrectionMemory() (*CorrectionMemory, error) {
	store, err := NewStore("~/.config/vecai/corrections")
	if err != nil {
		return nil, err
	}

	return &CorrectionMemory{
		store: store,
	}, nil
}

// Learn records a new correction
func (c *CorrectionMemory) Learn(trigger, problem, solution, context string) error {
	id := c.generateID(trigger, problem)

	// Check if we already have this correction
	if existing, ok := c.store.Get(id); ok {
		// Update use count
		return c.store.IncrementUseCount(existing.ID)
	}

	correction := Correction{
		ID:          id,
		Trigger:     trigger,
		Problem:     problem,
		Solution:    solution,
		Context:     context,
		UseCount:    1,
		SuccessRate: 1.0,
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
	}

	entry := &MemoryEntry{
		ID:      id,
		Type:    MemoryTypeCorrection,
		Content: c.serializeCorrection(correction),
		Metadata: map[string]string{
			"trigger": trigger[:min(100, len(trigger))],
		},
	}

	return c.store.Add(entry)
}

// FindRelevant finds corrections relevant to the current context
func (c *CorrectionMemory) FindRelevant(errorMessage, context string) []Correction {
	entries := c.store.List(MemoryTypeCorrection)

	var relevant []Correction
	for _, entry := range entries {
		correction := c.parseCorrection(entry.Content)
		if correction == nil {
			continue
		}

		// Check if trigger matches
		if c.matches(correction.Trigger, errorMessage) {
			relevant = append(relevant, *correction)
			continue
		}

		// Check if context matches
		if context != "" && c.matches(correction.Context, context) {
			relevant = append(relevant, *correction)
		}
	}

	// Sort by use count and success rate (most useful first)
	for i := 0; i < len(relevant); i++ {
		for j := i + 1; j < len(relevant); j++ {
			scoreI := float64(relevant[i].UseCount) * relevant[i].SuccessRate
			scoreJ := float64(relevant[j].UseCount) * relevant[j].SuccessRate
			if scoreJ > scoreI {
				relevant[i], relevant[j] = relevant[j], relevant[i]
			}
		}
	}

	return relevant
}

// RecordSuccess records that a correction was successfully applied
func (c *CorrectionMemory) RecordSuccess(correctionID string) error {
	entry, ok := c.store.Get(correctionID)
	if !ok {
		return nil
	}

	correction := c.parseCorrection(entry.Content)
	if correction == nil {
		return nil
	}

	// Update success rate (weighted average)
	correction.SuccessRate = (correction.SuccessRate*float64(correction.UseCount) + 1.0) / float64(correction.UseCount+1)
	correction.UseCount++
	correction.LastUsed = time.Now()

	entry.Content = c.serializeCorrection(*correction)
	return c.store.Update(entry)
}

// RecordFailure records that a correction didn't work
func (c *CorrectionMemory) RecordFailure(correctionID string) error {
	entry, ok := c.store.Get(correctionID)
	if !ok {
		return nil
	}

	correction := c.parseCorrection(entry.Content)
	if correction == nil {
		return nil
	}

	// Update success rate
	correction.SuccessRate = (correction.SuccessRate * float64(correction.UseCount)) / float64(correction.UseCount+1)
	correction.UseCount++
	correction.LastUsed = time.Now()

	entry.Content = c.serializeCorrection(*correction)
	return c.store.Update(entry)
}

// GetAll retrieves all corrections
func (c *CorrectionMemory) GetAll() []Correction {
	entries := c.store.List(MemoryTypeCorrection)

	var corrections []Correction
	for _, entry := range entries {
		if correction := c.parseCorrection(entry.Content); correction != nil {
			corrections = append(corrections, *correction)
		}
	}
	return corrections
}

// FormatForPrompt formats relevant corrections for inclusion in a prompt
func (c *CorrectionMemory) FormatForPrompt(corrections []Correction) string {
	if len(corrections) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Known Issues to Avoid\n\n")

	for _, corr := range corrections[:min(5, len(corrections))] {
		sb.WriteString(fmt.Sprintf("**Problem:** %s\n", corr.Problem))
		sb.WriteString(fmt.Sprintf("**Solution:** %s\n", corr.Solution))
		if corr.Context != "" {
			sb.WriteString(fmt.Sprintf("**Applies when:** %s\n", corr.Context))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Prune removes old, unused corrections
func (c *CorrectionMemory) Prune() error {
	// Remove corrections older than 30 days that have low success rate
	return c.store.Prune(30*24*time.Hour, 2)
}

// Close closes the correction memory store
func (c *CorrectionMemory) Close() error {
	return c.store.Close()
}

// Helper methods

func (c *CorrectionMemory) generateID(trigger, problem string) string {
	hash := sha256.Sum256([]byte(trigger + "|" + problem))
	return "correction-" + hex.EncodeToString(hash[:8])
}

func (c *CorrectionMemory) matches(pattern, text string) bool {
	// Simple substring matching (case-insensitive)
	return strings.Contains(
		strings.ToLower(text),
		strings.ToLower(pattern),
	)
}

func (c *CorrectionMemory) serializeCorrection(corr Correction) string {
	return fmt.Sprintf("ID:%s\nTRIGGER:%s\nPROBLEM:%s\nSOLUTION:%s\nCONTEXT:%s\nUSE_COUNT:%d\nSUCCESS_RATE:%.2f\nCREATED:%s\nLAST_USED:%s",
		corr.ID,
		corr.Trigger,
		corr.Problem,
		corr.Solution,
		corr.Context,
		corr.UseCount,
		corr.SuccessRate,
		corr.CreatedAt.Format(time.RFC3339),
		corr.LastUsed.Format(time.RFC3339))
}

func (c *CorrectionMemory) parseCorrection(content string) *Correction {
	corr := &Correction{}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "ID:") {
			corr.ID = strings.TrimPrefix(line, "ID:")
		} else if strings.HasPrefix(line, "TRIGGER:") {
			corr.Trigger = strings.TrimPrefix(line, "TRIGGER:")
		} else if strings.HasPrefix(line, "PROBLEM:") {
			corr.Problem = strings.TrimPrefix(line, "PROBLEM:")
		} else if strings.HasPrefix(line, "SOLUTION:") {
			corr.Solution = strings.TrimPrefix(line, "SOLUTION:")
		} else if strings.HasPrefix(line, "CONTEXT:") {
			corr.Context = strings.TrimPrefix(line, "CONTEXT:")
		} else if strings.HasPrefix(line, "USE_COUNT:") {
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "USE_COUNT:"), "%d", &corr.UseCount)
		} else if strings.HasPrefix(line, "SUCCESS_RATE:") {
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "SUCCESS_RATE:"), "%f", &corr.SuccessRate)
		} else if strings.HasPrefix(line, "CREATED:") {
			corr.CreatedAt, _ = time.Parse(time.RFC3339, strings.TrimPrefix(line, "CREATED:"))
		} else if strings.HasPrefix(line, "LAST_USED:") {
			corr.LastUsed, _ = time.Parse(time.RFC3339, strings.TrimPrefix(line, "LAST_USED:"))
		}
	}
	if corr.Problem == "" {
		return nil
	}
	return corr
}
