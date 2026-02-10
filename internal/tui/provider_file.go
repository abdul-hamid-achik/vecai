package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileMentionProvider provides completion for @ file mentions.
// Uses a cached file list for fast substring/fuzzy matching.
type FileMentionProvider struct {
	cache *FileCache
}

// NewFileMentionProvider creates a provider that searches files under projectRoot.
// Pass "" to use the current working directory.
func NewFileMentionProvider(projectRoot string) *FileMentionProvider {
	if projectRoot == "" {
		projectRoot, _ = os.Getwd()
	}
	return &FileMentionProvider{
		cache: NewFileCache(projectRoot),
	}
}

// Trigger returns TriggerAt
func (p *FileMentionProvider) Trigger() TriggerChar {
	return TriggerAt
}

// IsAsync returns false for the fast path (Phase 2).
// Phase 3 will add async vecgrep search alongside this sync path.
func (p *FileMentionProvider) IsAsync() bool {
	return false
}

// Complete returns files matching the query by substring and fuzzy matching.
func (p *FileMentionProvider) Complete(query string) []CompletionItem {
	if query == "" {
		return nil
	}

	files := p.cache.Files()
	if len(files) == 0 {
		return nil
	}

	query = strings.ToLower(query)
	var items []CompletionItem

	for _, f := range files {
		lower := strings.ToLower(f)
		score := matchScore(lower, query)
		if score <= 0 {
			continue
		}

		lang := langFromExt(filepath.Ext(f))
		dir := filepath.Dir(f)
		if dir == "." {
			dir = ""
		}

		items = append(items, CompletionItem{
			Label:      filepath.Base(f),
			Detail:     dir,
			Kind:       KindFile,
			InsertText: "@" + f + " ",
			Score:      score,
			FilePath:   filepath.Join(p.cache.root, f),
			Language:   lang,
		})
	}

	// Sort by score descending, then by path length ascending (shorter = more relevant)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return len(items[i].FilePath) < len(items[j].FilePath)
	})

	// Cap results
	if len(items) > 12 {
		items = items[:12]
	}

	return items
}

// matchScore returns a relevance score for how well the file path matches the query.
// Returns 0 if no match. Higher = better.
func matchScore(path, query string) float64 {
	base := filepath.Base(path)

	// Exact base name match (without extension)
	nameNoExt := strings.TrimSuffix(base, filepath.Ext(base))
	if nameNoExt == query {
		return 1.0
	}

	// Base name starts with query
	if strings.HasPrefix(base, query) {
		return 0.9
	}

	// Base name contains query
	if strings.Contains(base, query) {
		return 0.8
	}

	// Full path contains query
	if strings.Contains(path, query) {
		return 0.6
	}

	// Fuzzy: all query chars appear in order in base name
	if fuzzyMatch(base, query) {
		return 0.4
	}

	return 0
}

// fuzzyMatch checks if all characters in query appear in order in target
func fuzzyMatch(target, query string) bool {
	qi := 0
	for i := 0; i < len(target) && qi < len(query); i++ {
		if target[i] == query[qi] {
			qi++
		}
	}
	return qi == len(query)
}

// langFromExt maps file extension to language name
func langFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".js":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".rs":
		return "Rust"
	case ".rb":
		return "Ruby"
	case ".java":
		return "Java"
	case ".c", ".h":
		return "C"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "C++"
	case ".yaml", ".yml":
		return "YAML"
	case ".json":
		return "JSON"
	case ".md":
		return "Markdown"
	case ".sh", ".bash":
		return "Shell"
	case ".sql":
		return "SQL"
	case ".html", ".htm":
		return "HTML"
	case ".css":
		return "CSS"
	case ".toml":
		return "TOML"
	default:
		return ""
	}
}

// FileCache maintains a cached list of project files for fast matching.
// Thread-safe. Auto-refreshes after TTL expires.
type FileCache struct {
	root string

	mu      sync.RWMutex
	files   []string  // Relative paths from root
	loadedAt time.Time
	ttl     time.Duration
}

// NewFileCache creates a cache for the given project root
func NewFileCache(root string) *FileCache {
	return &FileCache{
		root: root,
		ttl:  30 * time.Second,
	}
}

// Files returns the cached file list, refreshing if stale
func (c *FileCache) Files() []string {
	c.mu.RLock()
	if time.Since(c.loadedAt) < c.ttl && c.files != nil {
		files := c.files
		c.mu.RUnlock()
		return files
	}
	c.mu.RUnlock()

	// Refresh
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(c.loadedAt) < c.ttl && c.files != nil {
		return c.files
	}

	c.files = c.walkProject()
	c.loadedAt = time.Now()
	return c.files
}

// Invalidate forces the cache to refresh on next access
func (c *FileCache) Invalidate() {
	c.mu.Lock()
	c.loadedAt = time.Time{}
	c.mu.Unlock()
}

// walkProject collects all relevant files under root.
// Uses git ls-files if available, falls back to filepath.Walk.
func (c *FileCache) walkProject() []string {
	// Try git ls-files first (fast, respects .gitignore)
	if files := c.gitLsFiles(); len(files) > 0 {
		return files
	}
	return c.fallbackWalk()
}

// gitLsFiles uses git to list tracked files
func (c *FileCache) gitLsFiles() []string {
	// Use os/exec here to avoid import cycle
	// We import it at file level
	return gitLsFilesFromRoot(c.root)
}

// fallbackWalk uses filepath.Walk with ignore patterns
func (c *FileCache) fallbackWalk() []string {
	var files []string
	_ = filepath.Walk(c.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		name := info.Name()

		// Skip hidden dirs and common non-code dirs
		if info.IsDir() {
			if shouldSkipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip binary/large files
		if shouldSkipFile(name, info.Size()) {
			return nil
		}

		// Make relative to root
		rel, err := filepath.Rel(c.root, path)
		if err != nil {
			return nil
		}

		files = append(files, rel)
		return nil
	})

	// Cap at 5000 files to prevent memory issues on huge repos
	if len(files) > 5000 {
		files = files[:5000]
	}

	return files
}

// shouldSkipDir returns true for directories to skip
func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".vecgrep", "node_modules", "vendor", ".idea", ".vscode",
		"__pycache__", ".mypy_cache", ".pytest_cache", "dist", "build",
		".next", ".nuxt", "target", "bin", ".terraform":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// shouldSkipFile returns true for files to skip
func shouldSkipFile(name string, size int64) bool {
	// Skip large files (>1MB)
	if size > 1024*1024 {
		return true
	}

	// Skip binary extensions
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".exe", ".dll", ".so", ".dylib", ".o", ".a",
		".zip", ".tar", ".gz", ".bz2", ".xz", ".7z",
		".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg",
		".woff", ".woff2", ".ttf", ".eot",
		".pdf", ".doc", ".docx",
		".lock", ".sum":
		return true
	}

	// Skip specific filenames
	switch name {
	case "package-lock.json", "yarn.lock", "go.sum", "Cargo.lock",
		"pnpm-lock.yaml", "composer.lock", "Gemfile.lock":
		return true
	}

	return false
}
