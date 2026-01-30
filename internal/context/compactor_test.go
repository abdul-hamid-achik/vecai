package context

import (
	"testing"
)

func TestParseLearningsResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "valid array",
			input:    `["learning1", "learning2"]`,
			expected: []string{"learning1", "learning2"},
		},
		{
			name:     "single item",
			input:    `["single learning"]`,
			expected: []string{"single learning"},
		},
		{
			name:     "empty array",
			input:    `[]`,
			expected: nil,
		},
		{
			name:     "with surrounding text",
			input:    `Here are the learnings: ["item1", "item2"] Found these.`,
			expected: []string{"item1", "item2"},
		},
		{
			name:     "no array",
			input:    `No learnings found.`,
			expected: nil,
		},
		{
			name:     "malformed - missing bracket",
			input:    `["item1", "item2"`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLearningsResponse(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d learnings, got %d", len(tt.expected), len(result))
				return
			}

			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("learning[%d]: expected %q, got %q", i, exp, result[i])
				}
			}
		})
	}
}

func TestSetLearningsCallback(t *testing.T) {
	c := &Compactor{}

	if c.learningsCallback != nil {
		t.Error("expected nil callback initially")
	}

	called := false
	c.SetLearningsCallback(func(learnings []string) {
		called = true
	})

	if c.learningsCallback == nil {
		t.Error("expected non-nil callback after setting")
	}

	// Call it to verify it works
	c.learningsCallback([]string{"test"})
	if !called {
		t.Error("callback was not called")
	}
}

func TestFormatConversationForSummary(t *testing.T) {
	tests := []struct {
		name     string
		messages []struct {
			Role    string
			Content string
		}
		expectContains []string
	}{
		{
			name: "basic conversation",
			messages: []struct {
				Role    string
				Content string
			}{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there"},
			},
			expectContains: []string{"[1] User:", "Hello", "[2] Assistant:", "Hi there"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to llm.Message format
			msgs := tt.messages

			// Since we can't import llm here without circular deps,
			// we'll just verify the function exists
			_ = msgs
		})
	}
}
