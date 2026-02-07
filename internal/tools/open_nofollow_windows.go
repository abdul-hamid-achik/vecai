//go:build windows

package tools

import (
	"fmt"
	"os"
	"path/filepath"
)

// openNoFollow opens a file for writing on Windows where O_NOFOLLOW is not available.
// We still perform post-open validation to mitigate symlink swaps.
func openNoFollow(path string, flag int, perm os.FileMode) (*os.File, error) {
	f, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}

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
