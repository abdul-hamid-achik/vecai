package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Solution represents a cached successful solution
type Solution struct {
	ID        string    `json:"id"`
	Request   string    `json:"request"`   // The original request
	Solution  string    `json:"solution"`  // The solution that worked
	Tags      []string  `json:"tags"`      // For categorization
	UseCount  int       `json:"use_count"` // How many times reused
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used"`
}

// SolutionCache caches successful completions for reuse
type SolutionCache struct {
	store           *Store
	similarityThreshold float64 // Minimum similarity to consider a match (0.0-1.0)
}

// NewSolutionCache creates a new solution cache
// Uses global config directory for solutions that apply across projects
func NewSolutionCache() (*SolutionCache, error) {
	store, err := NewStore("~/.config/vecai/solutions")
	if err != nil {
		return nil, err
	}

	return &SolutionCache{
		store:           store,
		similarityThreshold: 0.85,
	}, nil
}

// Cache stores a successful solution
func (s *SolutionCache) Cache(request, solution string, tags []string) error {
	id := s.generateID(request)

	// Check if we already have a similar solution
	if existing := s.FindSimilar(request); existing != nil {
		// Update the existing solution if this one is better (longer, more detailed)
		if len(solution) > len(existing.Solution) {
			existing.Solution = solution
			existing.LastUsed = time.Now()
			entry, _ := s.store.Get(existing.ID)
			if entry != nil {
				entry.Content = s.serializeSolution(*existing)
				return s.store.Update(entry)
			}
		}
		return s.store.IncrementUseCount(existing.ID)
	}

	sol := Solution{
		ID:        id,
		Request:   request,
		Solution:  solution,
		Tags:      tags,
		UseCount:  1,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}

	entry := &MemoryEntry{
		ID:      id,
		Type:    MemoryTypeSolution,
		Content: s.serializeSolution(sol),
		Metadata: map[string]string{
			"request_hash": s.hashRequest(request),
		},
	}

	return s.store.Add(entry)
}

// FindSimilar finds a cached solution for a similar request
func (s *SolutionCache) FindSimilar(request string) *Solution {
	entries := s.store.List(MemoryTypeSolution)

	var bestMatch *Solution
	var bestScore float64

	for _, entry := range entries {
		solution := s.parseSolution(entry.Content)
		if solution == nil {
			continue
		}

		score := s.calculateSimilarity(request, solution.Request)
		if score >= s.similarityThreshold && score > bestScore {
			bestScore = score
			bestMatch = solution
		}
	}

	return bestMatch
}

// FindByTags finds solutions matching any of the given tags
func (s *SolutionCache) FindByTags(tags []string) []Solution {
	entries := s.store.List(MemoryTypeSolution)

	var matches []Solution
	for _, entry := range entries {
		solution := s.parseSolution(entry.Content)
		if solution == nil {
			continue
		}

		for _, tag := range tags {
			for _, stag := range solution.Tags {
				if strings.EqualFold(tag, stag) {
					matches = append(matches, *solution)
					break
				}
			}
		}
	}

	return matches
}

// RecordUse records that a cached solution was used
func (s *SolutionCache) RecordUse(solutionID string) error {
	entry, ok := s.store.Get(solutionID)
	if !ok {
		return nil
	}

	solution := s.parseSolution(entry.Content)
	if solution == nil {
		return nil
	}

	solution.UseCount++
	solution.LastUsed = time.Now()

	entry.Content = s.serializeSolution(*solution)
	return s.store.Update(entry)
}

// GetFrequent returns the most frequently used solutions
func (s *SolutionCache) GetFrequent(limit int) []Solution {
	entries := s.store.List(MemoryTypeSolution)

	var solutions []Solution
	for _, entry := range entries {
		if solution := s.parseSolution(entry.Content); solution != nil {
			solutions = append(solutions, *solution)
		}
	}

	// Sort by use count (most used first)
	for i := 0; i < len(solutions); i++ {
		for j := i + 1; j < len(solutions); j++ {
			if solutions[j].UseCount > solutions[i].UseCount {
				solutions[i], solutions[j] = solutions[j], solutions[i]
			}
		}
	}

	if limit > 0 && len(solutions) > limit {
		solutions = solutions[:limit]
	}

	return solutions
}

// GetRecent returns the most recently used solutions
func (s *SolutionCache) GetRecent(limit int) []Solution {
	entries := s.store.List(MemoryTypeSolution)

	var solutions []Solution
	for _, entry := range entries {
		if solution := s.parseSolution(entry.Content); solution != nil {
			solutions = append(solutions, *solution)
		}
	}

	// Sort by last used (most recent first)
	for i := 0; i < len(solutions); i++ {
		for j := i + 1; j < len(solutions); j++ {
			if solutions[j].LastUsed.After(solutions[i].LastUsed) {
				solutions[i], solutions[j] = solutions[j], solutions[i]
			}
		}
	}

	if limit > 0 && len(solutions) > limit {
		solutions = solutions[:limit]
	}

	return solutions
}

// SetSimilarityThreshold sets the minimum similarity for matching
func (s *SolutionCache) SetSimilarityThreshold(threshold float64) {
	if threshold >= 0.0 && threshold <= 1.0 {
		s.similarityThreshold = threshold
	}
}

// Prune removes old, unused solutions
func (s *SolutionCache) Prune() error {
	// Remove solutions older than 60 days that were only used once
	return s.store.Prune(60*24*time.Hour, 2)
}

// Close closes the solution cache store
func (s *SolutionCache) Close() error {
	return s.store.Close()
}

// Helper methods

func (s *SolutionCache) generateID(request string) string {
	hash := sha256.Sum256([]byte(request))
	return "solution-" + hex.EncodeToString(hash[:8])
}

func (s *SolutionCache) hashRequest(request string) string {
	hash := sha256.Sum256([]byte(request))
	return hex.EncodeToString(hash[:16])
}

// calculateSimilarity calculates similarity between two strings
// Uses a combination of token overlap and length similarity
func (s *SolutionCache) calculateSimilarity(a, b string) float64 {
	// Tokenize both strings
	tokensA := tokenize(a)
	tokensB := tokenize(b)

	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0.0
	}

	// Calculate Jaccard similarity
	setA := make(map[string]bool)
	for _, t := range tokensA {
		setA[t] = true
	}

	setB := make(map[string]bool)
	for _, t := range tokensB {
		setB[t] = true
	}

	intersection := 0
	for t := range setA {
		if setB[t] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}

	jaccard := float64(intersection) / float64(union)

	// Also consider length similarity
	lengthRatio := float64(min(len(a), len(b))) / float64(max(len(a), len(b)))

	// Weighted combination
	return 0.7*jaccard + 0.3*lengthRatio
}

// tokenize splits text into normalized tokens
func tokenize(text string) []string {
	// Simple tokenization: split on whitespace and punctuation
	text = strings.ToLower(text)

	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func (s *SolutionCache) serializeSolution(sol Solution) string {
	return fmt.Sprintf("ID:%s\nREQUEST:%s\nSOLUTION:%s\nTAGS:%s\nUSE_COUNT:%d\nCREATED:%s\nLAST_USED:%s",
		sol.ID,
		sol.Request,
		sol.Solution,
		strings.Join(sol.Tags, ","),
		sol.UseCount,
		sol.CreatedAt.Format(time.RFC3339),
		sol.LastUsed.Format(time.RFC3339))
}

func (s *SolutionCache) parseSolution(content string) *Solution {
	sol := &Solution{}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "ID:") {
			sol.ID = strings.TrimPrefix(line, "ID:")
		} else if strings.HasPrefix(line, "REQUEST:") {
			sol.Request = strings.TrimPrefix(line, "REQUEST:")
		} else if strings.HasPrefix(line, "SOLUTION:") {
			sol.Solution = strings.TrimPrefix(line, "SOLUTION:")
		} else if strings.HasPrefix(line, "TAGS:") {
			tags := strings.TrimPrefix(line, "TAGS:")
			if tags != "" {
				sol.Tags = strings.Split(tags, ",")
			}
		} else if strings.HasPrefix(line, "USE_COUNT:") {
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "USE_COUNT:"), "%d", &sol.UseCount)
		} else if strings.HasPrefix(line, "CREATED:") {
			sol.CreatedAt, _ = time.Parse(time.RFC3339, strings.TrimPrefix(line, "CREATED:"))
		} else if strings.HasPrefix(line, "LAST_USED:") {
			sol.LastUsed, _ = time.Parse(time.RFC3339, strings.TrimPrefix(line, "LAST_USED:"))
		}
	}
	if sol.Request == "" {
		return nil
	}
	return sol
}
