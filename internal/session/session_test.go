package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/llm"
)

func TestManager(t *testing.T) {
	// Create temp directory for test sessions
	tmpDir, err := os.MkdirTemp("", "vecai-session-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create manager with custom directory
	mgr := &Manager{dir: tmpDir}

	t.Run("StartNew", func(t *testing.T) {
		sess, err := mgr.StartNew()
		if err != nil {
			t.Fatal(err)
		}
		if sess.ID == "" {
			t.Error("expected non-empty ID")
		}
		if sess.CreatedAt.IsZero() {
			t.Error("expected non-zero CreatedAt")
		}
	})

	t.Run("Save and Load", func(t *testing.T) {
		sess, _ := mgr.StartNew()
		messages := []llm.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		}

		err := mgr.Save(messages, "claude-haiku-4-5-20251001")
		if err != nil {
			t.Fatal(err)
		}

		// Verify file exists
		path := filepath.Join(tmpDir, sess.ID+".json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("session file not created")
		}

		// Load and verify
		loaded, err := mgr.Load(sess.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(loaded.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(loaded.Messages))
		}
		if loaded.Model != "claude-haiku-4-5-20251001" {
			t.Errorf("expected model claude-haiku-4-5-20251001, got %s", loaded.Model)
		}
	})

	t.Run("List", func(t *testing.T) {
		// Create a few sessions
		for i := 0; i < 3; i++ {
			_, _ = mgr.StartNew()
			_ = mgr.Save([]llm.Message{{Role: "user", Content: "Test"}}, "test-model")
			time.Sleep(10 * time.Millisecond) // Ensure different timestamps
		}

		sessions, err := mgr.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(sessions) < 3 {
			t.Errorf("expected at least 3 sessions, got %d", len(sessions))
		}
		// Verify sorted by UpdatedAt desc
		for i := 1; i < len(sessions); i++ {
			if sessions[i].UpdatedAt.After(sessions[i-1].UpdatedAt) {
				t.Error("sessions not sorted by UpdatedAt desc")
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		sess, _ := mgr.StartNew()
		_ = mgr.Save([]llm.Message{{Role: "user", Content: "To delete"}}, "test")

		err := mgr.Delete(sess.ID)
		if err != nil {
			t.Fatal(err)
		}

		// Verify file removed
		path := filepath.Join(tmpDir, sess.ID+".json")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("session file not deleted")
		}
	})

	t.Run("GetCurrent with symlink", func(t *testing.T) {
		sess, _ := mgr.StartNew()
		_ = mgr.Save([]llm.Message{{Role: "user", Content: "Current"}}, "test")

		current, err := mgr.GetCurrent()
		if err != nil {
			t.Fatal(err)
		}
		if current == nil {
			t.Fatal("expected current session")
		}
		if current.ID != sess.ID {
			t.Errorf("expected current ID %s, got %s", sess.ID, current.ID)
		}
	})
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"a longer string", 10, "a longe..."},
		{"with\nnewlines", 15, "with newlines"},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestGenerateID(t *testing.T) {
	id1, err := generateID()
	if err != nil {
		t.Fatal(err)
	}
	id2, _ := generateID()

	if id1 == id2 {
		t.Error("generated IDs should be unique")
	}
	if len(id1) != 12 {
		t.Errorf("expected ID length 12, got %d", len(id1))
	}
}
