package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MemoryType represents the type of memory entry
type MemoryType string

const (
	MemoryTypeProject    MemoryType = "project"    // Project-specific patterns
	MemoryTypeSession    MemoryType = "session"    // Current session context
	MemoryTypeCorrection MemoryType = "correction" // Learned corrections
	MemoryTypeSolution   MemoryType = "solution"   // Cached solutions
)

// MemoryEntry represents a single memory entry
type MemoryEntry struct {
	ID        string            `json:"id"`
	Type      MemoryType        `json:"type"`
	Content   string            `json:"content"`
	Embedding []float32         `json:"embedding,omitempty"` // For semantic search
	Metadata  map[string]string `json:"metadata,omitempty"`
	UseCount  int               `json:"use_count"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Store provides persistent memory storage
type Store struct {
	basePath string
	entries  map[string]*MemoryEntry
	mu       sync.RWMutex
}

// NewStore creates a new memory store
func NewStore(basePath string) (*Store, error) {
	// Expand ~ to home directory
	if basePath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		basePath = filepath.Join(home, basePath[1:])
	}

	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	store := &Store{
		basePath: basePath,
		entries:  make(map[string]*MemoryEntry),
	}

	// Load existing entries
	if err := store.load(); err != nil {
		// Not a fatal error - just start fresh
		store.entries = make(map[string]*MemoryEntry)
	}

	return store, nil
}

// Add adds a new entry to the store
func (s *Store) Add(entry *MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()
	s.entries[entry.ID] = entry

	return s.save()
}

// Get retrieves an entry by ID
func (s *Store) Get(id string) (*MemoryEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[id]
	return entry, ok
}

// Update updates an existing entry
func (s *Store) Update(entry *MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry.UpdatedAt = time.Now()
	s.entries[entry.ID] = entry

	return s.save()
}

// Delete removes an entry
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, id)
	return s.save()
}

// List returns all entries of a specific type
func (s *Store) List(memType MemoryType) []*MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*MemoryEntry
	for _, entry := range s.entries {
		if entry.Type == memType {
			result = append(result, entry)
		}
	}
	return result
}

// Search finds entries matching a query (simple substring match)
// For more advanced search, use with veclite/embeddings
func (s *Store) Search(query string, memType MemoryType, limit int) []*MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*MemoryEntry
	for _, entry := range s.entries {
		if memType != "" && entry.Type != memType {
			continue
		}

		// Simple substring match
		if contains(entry.Content, query) {
			result = append(result, entry)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// IncrementUseCount increases the use count for an entry
func (s *Store) IncrementUseCount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.entries[id]; ok {
		entry.UseCount++
		entry.UpdatedAt = time.Now()
		return s.save()
	}
	return nil
}

// Prune removes old entries that haven't been used
func (s *Store) Prune(maxAge time.Duration, minUseCount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, entry := range s.entries {
		if entry.UpdatedAt.Before(cutoff) && entry.UseCount < minUseCount {
			delete(s.entries, id)
		}
	}

	return s.save()
}

// load reads entries from disk
func (s *Store) load() error {
	dataFile := filepath.Join(s.basePath, "memory.json")

	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &s.entries)
}

// save writes entries to disk
func (s *Store) save() error {
	dataFile := filepath.Join(s.basePath, "memory.json")

	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(dataFile, data, 0644)
}

// Close persists and closes the store
func (s *Store) Close() error {
	return s.save()
}

// contains checks if s contains substr (case-insensitive)
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) >= len(substr) && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	// Simple case-insensitive contains
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			// Convert to lowercase
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
