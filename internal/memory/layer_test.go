package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewMemoryLayer(t *testing.T) {
	// Create temp directory for test
	dir := t.TempDir()

	layer, err := NewMemoryLayer(dir)
	if err != nil {
		t.Fatalf("NewMemoryLayer failed: %v", err)
	}
	defer layer.Close()

	if layer == nil {
		t.Fatal("expected non-nil memory layer")
	}

	// Session memory should always be initialized
	if layer.Session == nil {
		t.Error("expected non-nil session memory")
	}
}

func TestMemoryLayer_SessionMemory(t *testing.T) {
	dir := t.TempDir()

	layer, err := NewMemoryLayer(dir)
	if err != nil {
		t.Fatalf("NewMemoryLayer failed: %v", err)
	}
	defer layer.Close()

	// Test SetGoal
	layer.SetGoal("Implement feature X")
	if layer.Session.GetGoal() != "Implement feature X" {
		t.Error("SetGoal failed")
	}

	// Test RecordFileAccess
	layer.RecordFileAccess("/path/to/file.go")
	files := layer.Session.GetTouchedFiles()
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}

	// Test RecordDecision
	layer.RecordDecision("Use pattern X", "Because of reason Y")
	decisions := layer.Session.GetDecisions()
	if len(decisions) != 1 {
		t.Errorf("expected 1 decision, got %d", len(decisions))
	}

	// Test RecordError
	layer.RecordError("test error", "test context")
	errors := layer.Session.GetUnresolvedErrors()
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestMemoryLayer_GetContextEnrichment(t *testing.T) {
	dir := t.TempDir()

	layer, err := NewMemoryLayer(dir)
	if err != nil {
		t.Fatalf("NewMemoryLayer failed: %v", err)
	}
	defer layer.Close()

	// Empty query should work without panicking
	ctx := layer.GetContextEnrichment("")
	// May be empty if no context set
	_ = ctx

	// Set some context
	layer.SetGoal("Test goal")
	layer.RecordDecision("Test decision", "Test rationale")

	ctx = layer.GetContextEnrichment("test query")
	// Should have some context now
	if ctx == "" {
		t.Error("expected non-empty context enrichment")
	}
}

func TestMemoryLayer_Close(t *testing.T) {
	dir := t.TempDir()

	layer, err := NewMemoryLayer(dir)
	if err != nil {
		t.Fatalf("NewMemoryLayer failed: %v", err)
	}

	err = layer.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestMemoryLayer_IsNotedAvailable(t *testing.T) {
	dir := t.TempDir()

	layer, err := NewMemoryLayer(dir)
	if err != nil {
		t.Fatalf("NewMemoryLayer failed: %v", err)
	}
	defer layer.Close()

	// Just ensure it doesn't panic
	_ = layer.IsNotedAvailable()
}

func TestMemoryLayer_ProjectMemoryCreatesDirectory(t *testing.T) {
	dir := t.TempDir()

	layer, err := NewMemoryLayer(dir)
	if err != nil {
		t.Fatalf("NewMemoryLayer failed: %v", err)
	}
	defer layer.Close()

	// Check that .vecai/memory directory was created
	memDir := filepath.Join(dir, ".vecai", "memory")
	if _, err := os.Stat(memDir); os.IsNotExist(err) {
		t.Error("expected .vecai/memory directory to be created")
	}
}

func TestMemoryLayer_NilChecks(t *testing.T) {
	// Test that methods handle nil stores gracefully
	layer := &MemoryLayer{
		Session: NewSessionMemory(),
		// Leave Project, Corrections, Solutions nil
	}

	// These should not panic
	err := layer.LearnCorrection("a", "b", "c", "d")
	if err == nil {
		t.Error("expected error when Corrections is nil")
	}

	err = layer.CacheSolution("request", "solution", []string{"tag"})
	if err == nil {
		t.Error("expected error when Solutions is nil")
	}

	sol := layer.FindSimilarSolution("test")
	if sol != nil {
		t.Error("expected nil when Solutions is nil")
	}

	// Close should handle nils
	err = layer.Close()
	if err != nil {
		t.Errorf("Close returned error with nil stores: %v", err)
	}
}
