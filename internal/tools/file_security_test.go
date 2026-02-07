package tools

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestInitProjectRoot(t *testing.T) {
	// Reset for testing
	projectRoot = ""
	projectRootOnce = syncOnce()

	err := InitProjectRoot()
	if err != nil {
		t.Fatalf("InitProjectRoot failed: %v", err)
	}

	root, err := getProjectRoot()
	if err != nil {
		t.Fatalf("getProjectRoot failed: %v", err)
	}

	if root == "" {
		t.Fatal("project root should not be empty after init")
	}

	// Should be an absolute path
	if !filepath.IsAbs(root) {
		t.Errorf("project root should be absolute, got: %s", root)
	}
}

func TestValidatePath_WithinProject(t *testing.T) {
	// Set up a temp project dir (resolve symlinks for macOS /tmp → /private/tmp)
	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projectRoot = tmpDir

	// Create a file within the project
	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should succeed - file within project
	if err := ValidatePath(testFile); err != nil {
		t.Errorf("ValidatePath should allow file within project: %v", err)
	}

	// Should succeed - subdirectory file
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	subFile := filepath.Join(subDir, "test.go")
	if err := os.WriteFile(subFile, []byte("package sub"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePath(subFile); err != nil {
		t.Errorf("ValidatePath should allow file in subdirectory: %v", err)
	}

	// Should succeed - new file (doesn't exist yet) within project
	newFile := filepath.Join(tmpDir, "new.go")
	if err := ValidatePath(newFile); err != nil {
		t.Errorf("ValidatePath should allow new file within project: %v", err)
	}
}

func TestValidatePath_OutsideProject(t *testing.T) {
	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projectRoot = tmpDir

	// Should fail - path outside project
	outsidePath := filepath.Join(filepath.Dir(tmpDir), "outside.txt")
	if err := ValidatePath(outsidePath); err == nil {
		t.Error("ValidatePath should reject path outside project")
	}

	// Should fail - double-dot traversal
	traversalPath := filepath.Join(tmpDir, "..", "outside.txt")
	if err := ValidatePath(traversalPath); err == nil {
		t.Error("ValidatePath should reject double-dot traversal")
	}

	// Should fail - absolute path to root
	if err := ValidatePath("/etc/passwd"); err == nil {
		t.Error("ValidatePath should reject /etc/passwd")
	}
}

func TestValidatePath_SymlinkTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	// Resolve symlinks (macOS /tmp → /private/tmp) so projectRoot matches resolved paths
	resolvedTmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	projectRoot = resolvedTmpDir

	// Create a directory outside the project
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside the project pointing outside
	symlinkPath := filepath.Join(resolvedTmpDir, "sneaky_link")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Skipf("Cannot create symlinks: %v", err)
	}

	// Trying to access a file through the symlink should fail
	traversalPath := filepath.Join(symlinkPath, "secret.txt")
	if err := ValidatePath(traversalPath); err == nil {
		t.Error("ValidatePath should reject symlink-based traversal")
	}
}

func TestValidatePathForWrite_RejectsSymlink(t *testing.T) {
	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projectRoot = tmpDir

	// Create a real file outside
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "target.txt")
	if err := os.WriteFile(outsideFile, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside project pointing to the outside file
	symlinkPath := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Skipf("Cannot create symlinks: %v", err)
	}

	// Write validation should reject the symlink
	if err := ValidatePathForWrite(symlinkPath); err == nil {
		t.Error("ValidatePathForWrite should reject symlink targets")
	}
}

func TestValidatePathForWrite_AllowsRegularFile(t *testing.T) {
	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projectRoot = tmpDir

	// Create a regular file within the project
	testFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should succeed for regular file
	if err := ValidatePathForWrite(testFile); err != nil {
		t.Errorf("ValidatePathForWrite should allow regular file: %v", err)
	}

	// Should succeed for new file
	newFile := filepath.Join(tmpDir, "new.txt")
	if err := ValidatePathForWrite(newFile); err != nil {
		t.Errorf("ValidatePathForWrite should allow new file: %v", err)
	}
}

func TestIsWithinRoot(t *testing.T) {
	tests := []struct {
		path   string
		root   string
		expect bool
	}{
		{"/project/file.go", "/project", true},
		{"/project/sub/file.go", "/project", true},
		{"/project", "/project", true},
		{"/other/file.go", "/project", false},
		{"/projectextra/file.go", "/project", false},
		{"/", "/project", false},
	}

	for _, tt := range tests {
		result := isWithinRoot(tt.path, tt.root)
		if result != tt.expect {
			t.Errorf("isWithinRoot(%q, %q) = %v, want %v", tt.path, tt.root, result, tt.expect)
		}
	}
}

func TestResolveExistingPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a nested structure
	subDir := filepath.Join(tmpDir, "a", "b")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Resolve existing path
	resolved, err := resolveExistingPath(subDir)
	if err != nil {
		t.Fatalf("resolveExistingPath failed: %v", err)
	}

	// Should resolve to the real path
	realSubDir, _ := filepath.EvalSymlinks(subDir)
	if resolved != realSubDir {
		t.Errorf("expected %q, got %q", realSubDir, resolved)
	}

	// Resolve path with non-existent tail
	newPath := filepath.Join(subDir, "new", "file.go")
	resolved, err = resolveExistingPath(newPath)
	if err != nil {
		t.Fatalf("resolveExistingPath with new tail failed: %v", err)
	}

	expected := filepath.Join(realSubDir, "new", "file.go")
	if resolved != expected {
		t.Errorf("expected %q, got %q", expected, resolved)
	}
}

// syncOnce returns a new sync.Once (helper for test reset)
func syncOnce() sync.Once {
	return sync.Once{}
}
