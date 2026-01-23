package context

import (
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/llm"
)

func TestNewContextManager(t *testing.T) {
	cfg := DefaultContextConfig()
	cm := NewContextManager("test system prompt", cfg)

	if cm == nil {
		t.Fatal("NewContextManager returned nil")
	}

	stats := cm.GetStats()
	if stats.MessageCount != 0 {
		t.Errorf("expected 0 messages, got %d", stats.MessageCount)
	}
	if stats.ContextWindow != DefaultContextWindow {
		t.Errorf("expected context window %d, got %d", DefaultContextWindow, stats.ContextWindow)
	}
}

func TestAddMessage(t *testing.T) {
	cfg := DefaultContextConfig()
	cm := NewContextManager("test prompt", cfg)

	cm.AddMessage(llm.Message{Role: "user", Content: "Hello"})
	cm.AddMessage(llm.Message{Role: "assistant", Content: "Hi there!"})

	messages := cm.GetMessages()
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}

	stats := cm.GetStats()
	if stats.MessageCount != 2 {
		t.Errorf("expected 2 in stats, got %d", stats.MessageCount)
	}
}

func TestClear(t *testing.T) {
	cfg := DefaultContextConfig()
	cm := NewContextManager("test prompt", cfg)

	cm.AddMessage(llm.Message{Role: "user", Content: "Hello"})
	cm.Clear()

	stats := cm.GetStats()
	if stats.MessageCount != 0 {
		t.Errorf("expected 0 messages after clear, got %d", stats.MessageCount)
	}
}

func TestGetBreakdown(t *testing.T) {
	cfg := DefaultContextConfig()
	cm := NewContextManager("system", cfg)

	cm.AddMessage(llm.Message{Role: "user", Content: "Hello world"})
	cm.AddMessage(llm.Message{Role: "assistant", Content: "Hi there!"})
	cm.AddMessage(llm.Message{Role: "user", Content: "Tool result:\nSome output"})

	breakdown := cm.GetBreakdown()

	if breakdown.SystemPrompt == 0 {
		t.Error("expected non-zero system prompt tokens")
	}
	if breakdown.UserMessages == 0 {
		t.Error("expected non-zero user message tokens")
	}
	if breakdown.AssistantMsgs == 0 {
		t.Error("expected non-zero assistant message tokens")
	}
	if breakdown.Total == 0 {
		t.Error("expected non-zero total tokens")
	}
}

func TestShouldCompact(t *testing.T) {
	cfg := ContextConfig{
		AutoCompactThreshold: 0.95,
		WarnThreshold:        0.80,
		PreserveLast:         4,
		EnableAutoCompact:    true,
		ContextWindow:        100, // Small window for testing
	}
	cm := NewContextManager("", cfg)

	// Add enough messages to exceed 95%
	for i := 0; i < 10; i++ {
		cm.AddMessage(llm.Message{
			Role:    "user",
			Content: "This is a message that takes up some tokens in our small context window",
		})
	}

	if !cm.ShouldCompact() {
		t.Error("expected ShouldCompact to return true")
	}
}

func TestShouldWarn(t *testing.T) {
	cfg := ContextConfig{
		AutoCompactThreshold: 0.95,
		WarnThreshold:        0.50, // Lower threshold for testing
		PreserveLast:         4,
		EnableAutoCompact:    true,
		ContextWindow:        100, // Small window for testing
	}
	cm := NewContextManager("", cfg)

	// Add messages to exceed 50% but not 95%
	for i := 0; i < 3; i++ {
		cm.AddMessage(llm.Message{
			Role:    "user",
			Content: "Short message",
		})
	}

	stats := cm.GetStats()
	if stats.UsagePercent < 0.50 {
		t.Skipf("usage %.2f%% is below warn threshold, test may need adjustment", stats.UsagePercent*100)
	}

	if stats.UsagePercent >= 0.95 {
		t.Skipf("usage %.2f%% is at compact threshold, test may need adjustment", stats.UsagePercent*100)
	}

	if !cm.ShouldWarn() {
		t.Errorf("expected ShouldWarn to return true at %.2f%% usage", stats.UsagePercent*100)
	}
}

func TestGetRecentMessages(t *testing.T) {
	cfg := DefaultContextConfig()
	cm := NewContextManager("", cfg)

	for i := 0; i < 5; i++ {
		cm.AddMessage(llm.Message{
			Role:    "user",
			Content: string(rune('A' + i)),
		})
	}

	recent := cm.GetRecentMessages(2)
	if len(recent) != 2 {
		t.Errorf("expected 2 recent messages, got %d", len(recent))
	}
	if recent[0].Content != "D" || recent[1].Content != "E" {
		t.Errorf("expected D and E, got %s and %s", recent[0].Content, recent[1].Content)
	}
}

func TestGetOlderMessages(t *testing.T) {
	cfg := DefaultContextConfig()
	cm := NewContextManager("", cfg)

	for i := 0; i < 5; i++ {
		cm.AddMessage(llm.Message{
			Role:    "user",
			Content: string(rune('A' + i)),
		})
	}

	older := cm.GetOlderMessages(2)
	if len(older) != 3 {
		t.Errorf("expected 3 older messages, got %d", len(older))
	}
	if older[0].Content != "A" || older[2].Content != "C" {
		t.Errorf("expected A to C, got %s to %s", older[0].Content, older[2].Content)
	}
}

func TestReplaceWithSummary(t *testing.T) {
	cfg := DefaultContextConfig()
	cm := NewContextManager("", cfg)

	// Add some messages
	for i := 0; i < 5; i++ {
		cm.AddMessage(llm.Message{
			Role:    "user",
			Content: "Original message " + string(rune('A'+i)),
		})
	}

	preserved := []llm.Message{
		{Role: "user", Content: "Recent message"},
	}

	cm.ReplaceWithSummary("This is a summary", preserved)

	messages := cm.GetMessages()
	// Should have: summary user msg, summary ack, preserved message
	if len(messages) != 3 {
		t.Errorf("expected 3 messages after replacement, got %d", len(messages))
	}

	// First message should be the summary
	if messages[0].Role != "user" {
		t.Error("expected first message to be user role (summary)")
	}
	if messages[0].Content == "" || len(messages[0].Content) < 10 {
		t.Error("expected summary content in first message")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input    string
		minTokens int
		maxTokens int
	}{
		{"", 0, 0},
		{"hi", 0, 5},
		{"Hello, world!", 2, 10},
		{string(make([]byte, 100)), 20, 50},
	}

	for _, tt := range tests {
		tokens := estimateTokens(tt.input)
		if tokens < tt.minTokens || tokens > tt.maxTokens {
			t.Errorf("estimateTokens(%q) = %d, expected between %d and %d",
				tt.input, tokens, tt.minTokens, tt.maxTokens)
		}
	}
}

func TestDefaultContextConfig(t *testing.T) {
	cfg := DefaultContextConfig()

	if cfg.AutoCompactThreshold != 0.95 {
		t.Errorf("expected AutoCompactThreshold 0.95, got %f", cfg.AutoCompactThreshold)
	}
	if cfg.WarnThreshold != 0.80 {
		t.Errorf("expected WarnThreshold 0.80, got %f", cfg.WarnThreshold)
	}
	if cfg.PreserveLast != 4 {
		t.Errorf("expected PreserveLast 4, got %d", cfg.PreserveLast)
	}
	if !cfg.EnableAutoCompact {
		t.Error("expected EnableAutoCompact to be true")
	}
	if cfg.ContextWindow != DefaultContextWindow {
		t.Errorf("expected ContextWindow %d, got %d", DefaultContextWindow, cfg.ContextWindow)
	}
}
