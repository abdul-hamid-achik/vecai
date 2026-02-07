//go:build !windows

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

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
