package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/llm"
)

const (
	// MaxSessions is the maximum number of sessions to retain
	MaxSessions = 10
	// CurrentSessionLink is the name of the symlink to the current session
	CurrentSessionLink = "current.json"
)

// Session represents a saved conversation session
type Session struct {
	ID        string        `json:"id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Model     string        `json:"model"`
	Messages  []llm.Message `json:"messages"`
	Summary   string        `json:"summary,omitempty"`
}

// SessionInfo contains summary information about a session for listing
type SessionInfo struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	Model     string
	Preview   string // First user message or summary
	MsgCount  int
}

// Manager handles session persistence
type Manager struct {
	dir     string   // ~/.vecai/sessions/
	current *Session // Currently active session
}

// NewManager creates a new session manager
func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(home, ".vecai", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &Manager{
		dir: dir,
	}, nil
}

// Save saves the current session with the given messages and model
func (m *Manager) Save(messages []llm.Message, model string) error {
	if m.current == nil {
		// Start a new session if none exists
		session, err := m.StartNew()
		if err != nil {
			return err
		}
		m.current = session
	}

	m.current.Messages = messages
	m.current.Model = model
	m.current.UpdatedAt = time.Now()

	// Write session file
	data, err := json.MarshalIndent(m.current, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	sessionPath := m.sessionPath(m.current.ID)
	if err := os.WriteFile(sessionPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	// Update current symlink
	if err := m.updateCurrentLink(m.current.ID); err != nil {
		return fmt.Errorf("failed to update current link: %w", err)
	}

	// Cleanup old sessions
	if err := m.cleanupOldSessions(); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup old sessions: %v\n", err)
	}

	return nil
}

// Load loads a session by ID
func (m *Manager) Load(id string) (*Session, error) {
	path := m.sessionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}

	return &session, nil
}

// List returns information about all saved sessions
func (m *Manager) List() ([]SessionInfo, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		name := entry.Name()
		// Skip non-JSON files and the current symlink
		if !strings.HasSuffix(name, ".json") || name == CurrentSessionLink {
			continue
		}

		id := strings.TrimSuffix(name, ".json")
		session, err := m.Load(id)
		if err != nil {
			continue // Skip corrupted sessions
		}

		info := SessionInfo{
			ID:        session.ID,
			CreatedAt: session.CreatedAt,
			UpdatedAt: session.UpdatedAt,
			Model:     session.Model,
			MsgCount:  len(session.Messages),
		}

		// Generate preview from first user message or summary
		if session.Summary != "" {
			info.Preview = truncate(session.Summary, 50)
		} else {
			for _, msg := range session.Messages {
				if msg.Role == "user" {
					info.Preview = truncate(msg.Content, 50)
					break
				}
			}
		}

		sessions = append(sessions, info)
	}

	// Sort by UpdatedAt descending (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// GetCurrent returns the current session (from symlink) if it exists
func (m *Manager) GetCurrent() (*Session, error) {
	linkPath := filepath.Join(m.dir, CurrentSessionLink)

	// Read symlink target
	target, err := os.Readlink(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No current session
		}
		return nil, fmt.Errorf("failed to read current session link: %w", err)
	}

	// Extract session ID from target
	id := strings.TrimSuffix(filepath.Base(target), ".json")

	session, err := m.Load(id)
	if err != nil {
		// Current link points to invalid session, remove it
		_ = os.Remove(linkPath)
		return nil, nil
	}

	return session, nil
}

// StartNew creates a new session with a fresh ID
func (m *Manager) StartNew() (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	now := time.Now()
	session := &Session{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  []llm.Message{},
	}

	m.current = session
	return session, nil
}

// Delete removes a session by ID
func (m *Manager) Delete(id string) error {
	path := m.sessionPath(id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session not found: %s", id)
		}
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// If this was the current session, remove the symlink
	linkPath := filepath.Join(m.dir, CurrentSessionLink)
	target, err := os.Readlink(linkPath)
	if err == nil {
		targetID := strings.TrimSuffix(filepath.Base(target), ".json")
		if targetID == id {
			_ = os.Remove(linkPath)
		}
	}

	// Clear current if it matches
	if m.current != nil && m.current.ID == id {
		m.current = nil
	}

	return nil
}

// SetCurrent sets the current session (used when resuming)
func (m *Manager) SetCurrent(session *Session) {
	m.current = session
}

// GetCurrentSession returns the currently active session (in memory)
func (m *Manager) GetCurrentSession() *Session {
	return m.current
}

// sessionPath returns the file path for a session ID
func (m *Manager) sessionPath(id string) string {
	return filepath.Join(m.dir, id+".json")
}

// updateCurrentLink updates the current.json symlink to point to the given session
func (m *Manager) updateCurrentLink(id string) error {
	linkPath := filepath.Join(m.dir, CurrentSessionLink)
	targetPath := id + ".json"

	// Remove existing symlink (ignore error - may not exist)
	_ = os.Remove(linkPath)

	// Create new symlink
	return os.Symlink(targetPath, linkPath)
}

// cleanupOldSessions removes sessions beyond MaxSessions
func (m *Manager) cleanupOldSessions() error {
	sessions, err := m.List()
	if err != nil {
		return err
	}

	if len(sessions) <= MaxSessions {
		return nil
	}

	// Sessions are sorted by UpdatedAt desc, so delete from the end
	for i := MaxSessions; i < len(sessions); i++ {
		path := m.sessionPath(sessions[i].ID)
		_ = os.Remove(path) // Best effort cleanup
	}

	return nil
}

// generateID generates a random session ID
func generateID() (string, error) {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// truncate truncates a string to maxLen, adding "..." if truncated
func truncate(s string, maxLen int) string {
	// Replace newlines with spaces for preview
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// FormatRelativeTime formats a time as a human-readable relative string
func FormatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	default:
		// For older sessions, show the date
		if t.Year() == now.Year() {
			return t.Format("Jan 2")
		}
		return t.Format("Jan 2, 2006")
	}
}
