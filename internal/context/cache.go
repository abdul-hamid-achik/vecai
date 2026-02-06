package context

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ToolResultCache stores full tool results outside the LLM context
// with summarized versions for context efficiency
type ToolResultCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	ttl     time.Duration
}

// CacheEntry represents a cached tool result
type CacheEntry struct {
	ToolName   string
	Input      map[string]any
	FullResult string
	Summary    string
	CreatedAt  time.Time
	AccessedAt time.Time
}

// DefaultCacheTTL is the default time-to-live for cache entries (5 minutes)
const DefaultCacheTTL = 5 * time.Minute

// MaxSummaryLength is the maximum length of a summary in characters
const MaxSummaryLength = 500

// MaxSummaryLines is the maximum number of lines in a summary
const MaxSummaryLines = 10

// NewToolResultCache creates a new tool result cache
func NewToolResultCache(ttl time.Duration) *ToolResultCache {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	cache := &ToolResultCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
	// Start background cleanup goroutine
	go cache.cleanupLoop()
	return cache
}

// Store caches a tool result and returns a summarized version
func (c *ToolResultCache) Store(toolName string, input map[string]any, result string) (summary string, cacheKey string) {
	key := c.generateKey(toolName, input)
	summary = c.summarize(result, toolName)

	c.mu.Lock()
	c.entries[key] = &CacheEntry{
		ToolName:   toolName,
		Input:      input,
		FullResult: result,
		Summary:    summary,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
	}
	c.mu.Unlock()

	return summary, key
}

// Get retrieves the full result from cache
func (c *ToolResultCache) Get(key string) (string, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return "", false
	}

	// Update access time
	c.mu.Lock()
	entry.AccessedAt = time.Now()
	c.mu.Unlock()

	return entry.FullResult, true
}

// GetByTool retrieves a cached result by tool name and input
func (c *ToolResultCache) GetByTool(toolName string, input map[string]any) (string, bool) {
	key := c.generateKey(toolName, input)
	return c.Get(key)
}

// Clear removes all entries from the cache
func (c *ToolResultCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*CacheEntry)
	c.mu.Unlock()
}

// Size returns the number of entries in the cache
func (c *ToolResultCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// generateKey creates a unique cache key from tool name and input.
// Keys in the input map are sorted to ensure deterministic output.
func (c *ToolResultCache) generateKey(toolName string, input map[string]any) string {
	var parts []string
	parts = append(parts, toolName)

	// Sort map keys for deterministic key generation
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, input[k]))
	}
	data := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter keys
}

// summarize creates a concise summary of a tool result
func (c *ToolResultCache) summarize(result string, toolName string) string {
	// If result is already short enough, return as-is
	if len(result) <= MaxSummaryLength {
		return result
	}

	lines := strings.Split(result, "\n")

	// Special handling for different tool types
	switch {
	case strings.HasPrefix(toolName, "vecgrep"):
		return c.summarizeVecgrepResult(lines, result)
	case toolName == "read_file":
		return c.summarizeFileResult(lines, result)
	case strings.HasPrefix(toolName, "gpeek"):
		return c.summarizeGitResult(lines, result)
	default:
		return c.summarizeGeneric(lines, result)
	}
}

// summarizeVecgrepResult summarizes vecgrep search results
func (c *ToolResultCache) summarizeVecgrepResult(lines []string, result string) string {
	var sb strings.Builder
	matchCount := 0

	for i, line := range lines {
		if i >= MaxSummaryLines {
			break
		}
		sb.WriteString(line)
		sb.WriteString("\n")
		if strings.Contains(line, ":") {
			matchCount++
		}
	}

	if len(lines) > MaxSummaryLines {
		remaining := len(lines) - MaxSummaryLines
		fmt.Fprintf(&sb, "\n[... %d more results, %d bytes total. Use cache key to retrieve full results]\n", remaining, len(result))
	}

	return sb.String()
}

// summarizeFileResult summarizes file read results
func (c *ToolResultCache) summarizeFileResult(lines []string, result string) string {
	var sb strings.Builder

	// Show first few lines
	previewLines := MaxSummaryLines / 2
	for i := 0; i < previewLines && i < len(lines); i++ {
		sb.WriteString(lines[i])
		sb.WriteString("\n")
	}

	if len(lines) > previewLines {
		fmt.Fprintf(&sb, "\n[... %d more lines, %d bytes total]\n", len(lines)-previewLines, len(result))
	}

	return sb.String()
}

// summarizeGitResult summarizes git-related results
func (c *ToolResultCache) summarizeGitResult(lines []string, result string) string {
	var sb strings.Builder

	for i, line := range lines {
		if i >= MaxSummaryLines {
			break
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	if len(lines) > MaxSummaryLines {
		fmt.Fprintf(&sb, "\n[... %d more lines]\n", len(lines)-MaxSummaryLines)
	}

	return sb.String()
}

// summarizeGeneric provides a generic summary
func (c *ToolResultCache) summarizeGeneric(lines []string, result string) string {
	var sb strings.Builder

	for i, line := range lines {
		if i >= MaxSummaryLines {
			break
		}
		// Truncate very long lines
		if len(line) > 100 {
			line = line[:97] + "..."
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	if len(lines) > MaxSummaryLines || len(result) > MaxSummaryLength {
		fmt.Fprintf(&sb, "\n[Truncated: %d lines, %d bytes total]\n", len(lines), len(result))
	}

	return sb.String()
}

// cleanupLoop periodically removes expired entries
func (c *ToolResultCache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired entries
func (c *ToolResultCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.Sub(entry.AccessedAt) > c.ttl {
			delete(c.entries, key)
		}
	}
}

// ShouldCache determines if a result should be cached based on size
// Small results don't benefit from caching
func ShouldCache(result string) bool {
	return len(result) > MaxSummaryLength
}
