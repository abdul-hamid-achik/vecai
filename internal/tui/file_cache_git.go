package tui

import (
	"os/exec"
	"strings"
)

// gitLsFilesFromRoot runs `git ls-files` in the given directory and returns tracked files.
// Returns nil if git is not available or the directory is not a git repo.
func gitLsFilesFromRoot(root string) []string {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil
	}

	lines := strings.Split(raw, "\n")

	// Filter out binary/lock files
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name := line
		if idx := strings.LastIndex(line, "/"); idx >= 0 {
			name = line[idx+1:]
		}
		if shouldSkipFile(name, 0) {
			continue
		}
		files = append(files, line)
	}

	// Cap at 5000
	if len(files) > 5000 {
		files = files[:5000]
	}

	return files
}
