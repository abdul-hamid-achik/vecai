package context

import (
	"sync"

	"github.com/abdul-hamid-achik/vecai/internal/llm"
)

const (
	// DefaultContextWindow is the default context window size for Claude models (200K tokens)
	DefaultContextWindow = 200000
)

// ContextConfig holds configuration for context management
type ContextConfig struct {
	AutoCompactThreshold float64 `yaml:"auto_compact_threshold"` // Default: 0.95
	WarnThreshold        float64 `yaml:"warn_threshold"`         // Default: 0.80
	PreserveLast         int     `yaml:"preserve_last"`          // Default: 4
	EnableAutoCompact    bool    `yaml:"enable_auto_compact"`    // Default: true
	ContextWindow        int     `yaml:"context_window"`         // Default: 200000
}

// DefaultContextConfig returns the default context configuration
func DefaultContextConfig() ContextConfig {
	return ContextConfig{
		AutoCompactThreshold: 0.95,
		WarnThreshold:        0.80,
		PreserveLast:         4,
		EnableAutoCompact:    true,
		ContextWindow:        DefaultContextWindow,
	}
}

// ContextStats contains statistics about context usage
type ContextStats struct {
	UsedTokens      int
	ContextWindow   int
	UsagePercent    float64
	MessageCount    int
	NeedsCompaction bool
	NeedsWarning    bool
}

// MessageBreakdown contains token counts by message type
type MessageBreakdown struct {
	SystemPrompt  int
	UserMessages  int
	AssistantMsgs int
	ToolResults   int
	Total         int
}

// ContextManager manages conversation context and token usage
type ContextManager struct {
	mu sync.RWMutex

	messages      []llm.Message
	systemPrompt  string
	contextWindow int

	// Thresholds
	compactThreshold  float64
	warnThreshold     float64
	preserveLast      int
	enableAutoCompact bool

	// Cached stats
	cachedTokens int
	statsDirty   bool

	// Session persistence callback
	onSave func([]llm.Message)
}

// NewContextManager creates a new context manager
func NewContextManager(systemPrompt string, cfg ContextConfig) *ContextManager {
	contextWindow := cfg.ContextWindow
	if contextWindow <= 0 {
		contextWindow = DefaultContextWindow
	}

	return &ContextManager{
		messages:          []llm.Message{},
		systemPrompt:      systemPrompt,
		contextWindow:     contextWindow,
		compactThreshold:  cfg.AutoCompactThreshold,
		warnThreshold:     cfg.WarnThreshold,
		preserveLast:      cfg.PreserveLast,
		enableAutoCompact: cfg.EnableAutoCompact,
		statsDirty:        true,
	}
}

// SetOnSave sets a callback function that is called after each message is added
func (cm *ContextManager) SetOnSave(fn func([]llm.Message)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onSave = fn
}

// AddMessage adds a message to the conversation
func (cm *ContextManager) AddMessage(msg llm.Message) {
	cm.mu.Lock()
	cm.messages = append(cm.messages, msg)
	cm.statsDirty = true

	// Copy messages and callback ref while holding lock
	var msgsCopy []llm.Message
	var onSave func([]llm.Message)
	if cm.onSave != nil {
		msgsCopy = make([]llm.Message, len(cm.messages))
		copy(msgsCopy, cm.messages)
		onSave = cm.onSave
	}
	cm.mu.Unlock()

	// Call callback outside of lock to avoid deadlocks
	if onSave != nil {
		onSave(msgsCopy)
	}
}

// RestoreMessages restores messages from a saved session
func (cm *ContextManager) RestoreMessages(messages []llm.Message) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.messages = make([]llm.Message, len(messages))
	copy(cm.messages, messages)
	cm.statsDirty = true
}

// GetMessages returns all messages
func (cm *ContextManager) GetMessages() []llm.Message {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]llm.Message, len(cm.messages))
	copy(result, cm.messages)
	return result
}

// GetMessageCount returns the number of messages
func (cm *ContextManager) GetMessageCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.messages)
}

// GetStats returns current context statistics
func (cm *ContextManager) GetStats() ContextStats {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.statsDirty {
		cm.cachedTokens = cm.calculateTotalTokens()
		cm.statsDirty = false
	}

	usagePercent := float64(cm.cachedTokens) / float64(cm.contextWindow)

	return ContextStats{
		UsedTokens:      cm.cachedTokens,
		ContextWindow:   cm.contextWindow,
		UsagePercent:    usagePercent,
		MessageCount:    len(cm.messages),
		NeedsCompaction: usagePercent >= cm.compactThreshold,
		NeedsWarning:    usagePercent >= cm.warnThreshold && usagePercent < cm.compactThreshold,
	}
}

// GetBreakdown returns token counts by message type
func (cm *ContextManager) GetBreakdown() MessageBreakdown {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	breakdown := MessageBreakdown{
		SystemPrompt: estimateTokens(cm.systemPrompt),
	}

	for _, msg := range cm.messages {
		tokens := estimateTokens(msg.Content)
		switch msg.Role {
		case "user":
			// Check if this is a tool result message (legacy format)
			if isToolResultMessage(msg.Content) {
				breakdown.ToolResults += tokens
			} else {
				breakdown.UserMessages += tokens
			}
		case "assistant":
			breakdown.AssistantMsgs += tokens
		case "tool":
			breakdown.ToolResults += tokens
		}
	}

	breakdown.Total = breakdown.SystemPrompt + breakdown.UserMessages +
		breakdown.AssistantMsgs + breakdown.ToolResults

	return breakdown
}

// ReplaceWithSummary replaces the conversation history with a summary
func (cm *ContextManager) ReplaceWithSummary(summary string, preserveRecent []llm.Message) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Create new message list with summary as first message
	newMessages := []llm.Message{
		{
			Role:    "user",
			Content: "[Previous conversation summary]\n" + summary,
		},
		{
			Role:    "assistant",
			Content: "I understand. I have the context from our previous conversation. How can I help you continue?",
		},
	}

	// Add preserved recent messages
	newMessages = append(newMessages, preserveRecent...)

	cm.messages = newMessages
	cm.statsDirty = true
}

// Clear clears all messages
func (cm *ContextManager) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.messages = []llm.Message{}
	cm.cachedTokens = 0
	cm.statsDirty = true
}

// ShouldCompact returns true if the context needs compaction
func (cm *ContextManager) ShouldCompact() bool {
	stats := cm.GetStats()
	return stats.NeedsCompaction && cm.enableAutoCompact
}

// ShouldWarn returns true if a warning should be shown
func (cm *ContextManager) ShouldWarn() bool {
	stats := cm.GetStats()
	return stats.NeedsWarning
}

// GetPreserveLast returns the number of messages to preserve during compaction
func (cm *ContextManager) GetPreserveLast() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.preserveLast
}

// GetRecentMessages returns the last N messages for preservation
func (cm *ContextManager) GetRecentMessages(n int) []llm.Message {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if n >= len(cm.messages) {
		result := make([]llm.Message, len(cm.messages))
		copy(result, cm.messages)
		return result
	}

	result := make([]llm.Message, n)
	copy(result, cm.messages[len(cm.messages)-n:])
	return result
}

// GetOlderMessages returns messages excluding the last N (for summarization)
func (cm *ContextManager) GetOlderMessages(excludeLast int) []llm.Message {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if excludeLast >= len(cm.messages) {
		return []llm.Message{}
	}

	count := len(cm.messages) - excludeLast
	result := make([]llm.Message, count)
	copy(result, cm.messages[:count])
	return result
}

// SetContextWindow updates the context window size
func (cm *ContextManager) SetContextWindow(size int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.contextWindow = size
	cm.statsDirty = true
}

// calculateTotalTokens estimates total tokens for all content
func (cm *ContextManager) calculateTotalTokens() int {
	total := estimateTokens(cm.systemPrompt)
	for _, msg := range cm.messages {
		total += estimateTokens(msg.Content)
		// Add overhead for message structure
		total += 10
	}
	return total
}

// estimateTokens provides a content-aware token estimate for text.
// Uses different chars-per-token ratios for code vs prose.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Content-aware estimation
	ratio := estimateCharsPerToken(text)
	base := int(float64(len(text)) / ratio)
	// Add 10% buffer instead of 20% for more accuracy
	return base + (base / 10)
}

// estimateCharsPerToken returns the estimated characters per token based on content type.
// Code has more symbols/punctuation = fewer chars per token.
// Prose has more regular words = more chars per token.
func estimateCharsPerToken(text string) float64 {
	punctCount := 0
	for _, r := range text {
		if (r >= '!' && r <= '/') || (r >= ':' && r <= '@') ||
			(r >= '[' && r <= '`') || (r >= '{' && r <= '~') {
			punctCount++
		}
	}
	punctRatio := float64(punctCount) / float64(max(len(text), 1))

	// High punctuation (>15%) suggests code
	if punctRatio > 0.15 {
		return 3.2 // Code: ~3.2 chars per token
	}
	return 4.0 // Prose: ~4.0 chars per token
}

// isToolResultMessage checks if a message content looks like tool results
func isToolResultMessage(content string) bool {
	return len(content) > 5 && content[:5] == "Tool "
}
