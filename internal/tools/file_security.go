package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

var (
	// projectRoot is the cached, resolved project root directory.
	// Set once at startup via InitProjectRoot() and never changed.
	projectRoot     string
	projectRootOnce sync.Once
)

// InitProjectRoot resolves and caches the project root directory.
// Must be called once at startup before any file operations.
// If not called, ValidatePath will resolve it lazily from os.Getwd().
func InitProjectRoot() error {
	var initErr error
	projectRootOnce.Do(func() {
		wd, err := os.Getwd()
		if err != nil {
			initErr = fmt.Errorf("failed to get working directory: %w", err)
			return
		}
		resolved, err := filepath.EvalSymlinks(wd)
		if err != nil {
			initErr = fmt.Errorf("failed to resolve project root: %w", err)
			return
		}
		projectRoot = resolved
	})
	return initErr
}

// getProjectRoot returns the cached project root, initializing lazily if needed.
func getProjectRoot() (string, error) {
	if projectRoot != "" {
		return projectRoot, nil
	}
	// Lazy init if InitProjectRoot wasn't called
	if err := InitProjectRoot(); err != nil {
		return "", err
	}
	return projectRoot, nil
}

// ValidatePath checks that a given path resolves to within the project directory.
// It resolves each existing path component to prevent symlink-based traversal and
// TOCTOU races. For new files (write operations), it validates the parent directory.
func ValidatePath(absPath string) error {
	root, err := getProjectRoot()
	if err != nil {
		return err
	}

	// Clean the path to remove any .. or . components
	cleaned := filepath.Clean(absPath)

	// Resolve the existing portion of the path
	resolved, err := resolveExistingPath(cleaned)
	if err != nil {
		return fmt.Errorf("access denied: cannot resolve path %q: %w", absPath, err)
	}

	// Check that the resolved path is within the project root
	if !isWithinRoot(resolved, root) {
		return fmt.Errorf("access denied: path %q resolves outside the project directory", absPath)
	}

	return nil
}

// ValidatePathForWrite is like ValidatePath but additionally checks that
// the target is not a symlink (to prevent symlink swap attacks on writes).
func ValidatePathForWrite(absPath string) error {
	// First do the standard validation
	if err := ValidatePath(absPath); err != nil {
		return err
	}

	// If the file already exists, check it's not a symlink
	info, err := os.Lstat(absPath)
	if err == nil {
		// File exists - check it's not a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("access denied: refusing to write through symlink %q", absPath)
		}
	}
	// If file doesn't exist, that's fine for writes

	return nil
}

// resolveExistingPath resolves a path by finding the deepest existing ancestor,
// fully resolving it via EvalSymlinks, then appending the non-existent tail.
// This handles macOS firmlinks (e.g., /var → /private/var) and regular symlinks.
func resolveExistingPath(path string) (string, error) {
	// Start from the full path and walk up until we find something that exists
	current := path
	var tailParts []string

	for {
		_, err := os.Lstat(current)
		if err == nil {
			// This part exists — fully resolve it (handles symlinks, firmlinks, etc.)
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", fmt.Errorf("cannot resolve path %q: %w", current, err)
			}
			// Append the non-existent tail parts
			for i := len(tailParts) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tailParts[i])
			}
			return filepath.Clean(resolved), nil
		}

		if !os.IsNotExist(err) {
			return "", err
		}

		// This component doesn't exist; remember it and try the parent
		parent := filepath.Dir(current)
		if parent == current {
			// We've reached the root without finding anything
			return filepath.Clean(path), nil
		}
		tailParts = append(tailParts, filepath.Base(current))
		current = parent
	}
}

// isWithinRoot checks if path is within or equal to root.
func isWithinRoot(path, root string) bool {
	// Exact match
	if path == root {
		return true
	}
	// Must have the root as prefix followed by a separator
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

// openNoFollow opens a file for writing without following symlinks.
// This provides an additional defense against symlink swap attacks
// that could occur between validation and open.
func openNoFollow(path string, flag int, perm os.FileMode) (*os.File, error) {
	// Use O_NOFOLLOW to refuse to open symlinks
	// Note: O_NOFOLLOW is available on both macOS and Linux
	f, err := os.OpenFile(path, flag|syscall.O_NOFOLLOW, perm)
	if err != nil {
		return nil, err
	}

	// Double-check: verify the opened file's real path is what we expected
	// This catches race conditions where the file was replaced between our check and open
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("post-open path resolution failed: %w", err)
	}

	root, err := getProjectRoot()
	if err != nil {
		f.Close()
		return nil, err
	}

	if !isWithinRoot(realPath, root) {
		f.Close()
		return nil, fmt.Errorf("access denied: file %q resolved outside project after open", path)
	}

	return f, nil
}
