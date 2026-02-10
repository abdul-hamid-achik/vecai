package agent

import (
	"fmt"
	"os"
	"sync"
)

const maxCheckpoints = 10

// checkpoint stores the original state of files before an agent loop iteration.
type checkpoint struct {
	Prompt string
	// Files maps path → original content (nil means the file didn't exist).
	Files map[string][]byte
}

// CheckpointManager tracks file states across agent loop iterations for /rewind.
type CheckpointManager struct {
	mu          sync.Mutex
	checkpoints []checkpoint
	current     *checkpoint
}

// NewCheckpointManager creates a new checkpoint manager.
func NewCheckpointManager() *CheckpointManager {
	return &CheckpointManager{}
}

// StartCheckpoint begins a new checkpoint for the given prompt.
// Call this before each agent loop iteration.
func (cm *CheckpointManager) StartCheckpoint(prompt string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.current = &checkpoint{
		Prompt: prompt,
		Files:  make(map[string][]byte),
	}
}

// SaveFileState records the current state of a file before it's modified.
// Should be called before write_file or edit_file executions.
// Only saves if the file hasn't already been saved in this checkpoint.
func (cm *CheckpointManager) SaveFileState(path string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.current == nil {
		return
	}

	// Already saved in this checkpoint
	if _, exists := cm.current.Files[path]; exists {
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist yet — record nil so rewind deletes it
		cm.current.Files[path] = nil
	} else {
		cm.current.Files[path] = content
	}
}

// CommitCheckpoint finalizes the current checkpoint.
// Only commits if files were actually saved (i.e., writes happened).
func (cm *CheckpointManager) CommitCheckpoint() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.current == nil || len(cm.current.Files) == 0 {
		cm.current = nil
		return
	}

	cm.checkpoints = append(cm.checkpoints, *cm.current)
	cm.current = nil

	// Trim to max checkpoints
	if len(cm.checkpoints) > maxCheckpoints {
		cm.checkpoints = cm.checkpoints[len(cm.checkpoints)-maxCheckpoints:]
	}
}

// Rewind restores all files from the last checkpoint to their pre-modification state.
// Returns the list of restored file paths and any error.
func (cm *CheckpointManager) Rewind() ([]string, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.checkpoints) == 0 {
		return nil, fmt.Errorf("no checkpoints to rewind")
	}

	// Pop the last checkpoint
	last := cm.checkpoints[len(cm.checkpoints)-1]
	cm.checkpoints = cm.checkpoints[:len(cm.checkpoints)-1]

	var restored []string
	var errors []string

	for path, content := range last.Files {
		if content == nil {
			// File didn't exist before — delete it
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("failed to remove %s: %v", path, err))
			} else {
				restored = append(restored, path+" (deleted)")
			}
		} else {
			// Restore original content
			if err := os.WriteFile(path, content, 0644); err != nil {
				errors = append(errors, fmt.Sprintf("failed to restore %s: %v", path, err))
			} else {
				restored = append(restored, path)
			}
		}
	}

	if len(errors) > 0 {
		return restored, fmt.Errorf("partial rewind: %s", errors[0])
	}

	return restored, nil
}

// HasCheckpoints returns true if there are checkpoints available to rewind.
func (cm *CheckpointManager) HasCheckpoints() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.checkpoints) > 0
}

// Count returns the number of stored checkpoints.
func (cm *CheckpointManager) Count() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.checkpoints)
}
