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

// StoreConfig holds configuration for the memory store
type StoreConfig struct {
	MaxEntries    int           // Maximum entries (default: 10000, 0 = unlimited)
	MaxDiskBytes  int64         // Maximum disk size in bytes (default: 10MB, 0 = unlimited)
	DefaultTTL    time.Duration // Default TTL for entries (default: 0 = no expiry)
	PruneInterval time.Duration // Auto-prune interval (default: 1h)
	WriteDebounce time.Duration // Debounce writes (default: 5s)
}

// DefaultStoreConfig returns sensible defaults for store configuration
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		MaxEntries:    10000,
		MaxDiskBytes:  10 * 1024 * 1024, // 10MB
		DefaultTTL:    0,                 // No expiry
		PruneInterval: 1 * time.Hour,
		WriteDebounce: 5 * time.Second,
	}
}

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
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

// Store provides persistent memory storage
type Store struct {
	basePath string
	entries  map[string]*MemoryEntry
	mu       sync.RWMutex
	config   StoreConfig

	// Write debouncing
	savePending bool
	saveTimer   *time.Timer

	// Background goroutine control
	done chan struct{}
	wg   sync.WaitGroup
}

// NewStore creates a new memory store with default configuration.
// Maintains backward compatibility - all existing callers continue to work.
func NewStore(basePath string) (*Store, error) {
	return NewStoreWithConfig(basePath, DefaultStoreConfig())
}

// NewStoreWithConfig creates a new memory store with the given configuration
func NewStoreWithConfig(basePath string, cfg StoreConfig) (*Store, error) {
	// Expand ~ to home directory
	if len(basePath) > 0 && basePath[0] == '~' {
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
		config:   cfg,
		done:     make(chan struct{}),
	}

	// Load existing entries
	if err := store.load(); err != nil {
		// Not a fatal error - just start fresh
		store.entries = make(map[string]*MemoryEntry)
	}

	// Start background auto-prune goroutine
	if cfg.PruneInterval > 0 {
		store.wg.Add(1)
		go store.autoPruneLoop()
	}

	return store, nil
}

// Add adds a new entry to the store
func (s *Store) Add(entry *MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entry.CreatedAt = now
	entry.UpdatedAt = now

	// Apply default TTL if configured and entry has no expiry set
	if s.config.DefaultTTL > 0 && entry.ExpiresAt.IsZero() {
		entry.ExpiresAt = now.Add(s.config.DefaultTTL)
	}

	s.entries[entry.ID] = entry

	// Enforce MaxEntries using LRU eviction
	if s.config.MaxEntries > 0 && len(s.entries) > s.config.MaxEntries {
		s.evictLRU(len(s.entries) - s.config.MaxEntries)
	}

	return s.debouncedSave()
}

// Get retrieves an entry by ID
func (s *Store) Get(id string) (*MemoryEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[id]
	if !ok {
		return nil, false
	}
	// Skip expired entries
	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry, ok
}

// Update updates an existing entry
func (s *Store) Update(entry *MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry.UpdatedAt = time.Now()
	s.entries[entry.ID] = entry

	return s.debouncedSave()
}

// Delete removes an entry
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, id)
	return s.debouncedSave()
}

// List returns all entries of a specific type
func (s *Store) List(memType MemoryType) []*MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var result []*MemoryEntry
	for _, entry := range s.entries {
		if entry.Type == memType {
			// Skip expired entries
			if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
				continue
			}
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

	now := time.Now()
	var result []*MemoryEntry
	for _, entry := range s.entries {
		if memType != "" && entry.Type != memType {
			continue
		}

		// Skip expired entries
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
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
		return s.debouncedSave()
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

	return s.saveImmediate()
}

// PruneExpired removes entries that have passed their ExpiresAt time
func (s *Store) PruneExpired() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, entry := range s.entries {
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			delete(s.entries, id)
		}
	}

	return s.saveImmediate()
}

// EntryCount returns the number of entries in the store (including expired ones still in memory)
func (s *Store) EntryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// evictLRU removes the N least recently used entries.
// Must be called while holding s.mu write lock.
func (s *Store) evictLRU(count int) {
	if count <= 0 {
		return
	}

	// Find the N entries with the oldest UpdatedAt
	type entryAge struct {
		id        string
		updatedAt time.Time
	}

	ages := make([]entryAge, 0, len(s.entries))
	for id, entry := range s.entries {
		ages = append(ages, entryAge{id: id, updatedAt: entry.UpdatedAt})
	}

	// Sort by UpdatedAt ascending (oldest first) using simple selection
	for i := 0; i < count && i < len(ages); i++ {
		minIdx := i
		for j := i + 1; j < len(ages); j++ {
			if ages[j].updatedAt.Before(ages[minIdx].updatedAt) {
				minIdx = j
			}
		}
		ages[i], ages[minIdx] = ages[minIdx], ages[i]
	}

	// Remove the oldest N entries
	for i := 0; i < count && i < len(ages); i++ {
		delete(s.entries, ages[i].id)
	}
}

// autoPruneLoop runs in the background and periodically prunes expired entries
func (s *Store) autoPruneLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.PruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = s.PruneExpired()
		case <-s.done:
			return
		}
	}
}

// debouncedSave schedules a save using debouncing.
// Must be called while holding s.mu lock.
func (s *Store) debouncedSave() error {
	if s.config.WriteDebounce <= 0 {
		return s.saveImmediate()
	}

	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}

	s.savePending = true
	s.saveTimer = time.AfterFunc(s.config.WriteDebounce, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.savePending {
			_ = s.saveImmediate()
			s.savePending = false
		}
	})

	return nil
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

// saveImmediate writes entries to disk immediately.
// Must be called while holding s.mu lock.
func (s *Store) saveImmediate() error {
	dataFile := filepath.Join(s.basePath, "memory.json")

	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}

	// Check disk size limit
	if s.config.MaxDiskBytes > 0 && int64(len(data)) > s.config.MaxDiskBytes {
		// Evict entries until we're under the limit
		for int64(len(data)) > s.config.MaxDiskBytes && len(s.entries) > 0 {
			s.evictLRU(1)
			data, err = json.MarshalIndent(s.entries, "", "  ")
			if err != nil {
				return err
			}
		}
	}

	return os.WriteFile(dataFile, data, 0644)
}

// save writes entries to disk (kept for compatibility with existing internal callers)
func (s *Store) save() error {
	return s.saveImmediate()
}

// Close stops background goroutines and does a final save
func (s *Store) Close() error {
	// Signal background goroutines to stop
	select {
	case <-s.done:
		// Already closed
	default:
		close(s.done)
	}

	// Stop any pending debounced save
	s.mu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
	s.savePending = false
	s.mu.Unlock()

	// Wait for background goroutines
	s.wg.Wait()

	// Final save
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveImmediate()
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
