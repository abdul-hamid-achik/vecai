package context

import (
	"context"
	"fmt"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/llm"
)

const compactionPrompt = `You are summarizing a conversation to preserve context while reducing token usage. Create a concise summary that captures:

1. Key decisions and conclusions reached
2. Important code changes or files discussed (with specific paths and line numbers if mentioned)
3. Critical technical context (paths, functions, errors, configurations)
4. Current task state and any pending actions
5. User preferences or requirements mentioned

Format your summary as clear bullet points. Preserve critical code snippets verbatim if they are essential for context.
Keep the summary focused and factual - avoid unnecessary elaboration.

%s

CONVERSATION TO SUMMARIZE:
%s`

// CompactRequest contains parameters for compaction
type CompactRequest struct {
	Messages     []llm.Message
	FocusPrompt  string // Optional: "preserve code samples", "keep file paths", etc.
	PreserveLast int    // Keep last N messages verbatim
}

// CompactResult contains the result of compaction
type CompactResult struct {
	Summary          string
	PreservedMsgs    []llm.Message
	OriginalTokens   int
	SummaryTokens    int
	TokensSaved      int
	MessagesSummarized int
}

// Compactor handles conversation compaction using LLM summarization
type Compactor struct {
	llmClient llm.LLMClient
}

// NewCompactor creates a new compactor
func NewCompactor(client llm.LLMClient) *Compactor {
	return &Compactor{
		llmClient: client,
	}
}

// Compact compresses a conversation history into a summary
func (c *Compactor) Compact(ctx context.Context, req CompactRequest) (*CompactResult, error) {
	if len(req.Messages) == 0 {
		return &CompactResult{
			Summary:       "",
			PreservedMsgs: []llm.Message{},
		}, nil
	}

	// Split messages into those to summarize and those to preserve
	preserveCount := min(len(req.Messages), max(0, req.PreserveLast))

	splitPoint := len(req.Messages) - preserveCount
	toSummarize := req.Messages[:splitPoint]
	toPreserve := make([]llm.Message, preserveCount)
	if preserveCount > 0 {
		copy(toPreserve, req.Messages[splitPoint:])
	}

	// If nothing to summarize, just return preserved messages
	if len(toSummarize) == 0 {
		return &CompactResult{
			Summary:          "",
			PreservedMsgs:    toPreserve,
			OriginalTokens:   calculateTokens(req.Messages),
			SummaryTokens:    0,
			TokensSaved:      0,
			MessagesSummarized: 0,
		}, nil
	}

	// Calculate original tokens
	originalTokens := calculateTokens(toSummarize)

	// Format messages for summarization
	conversationText := formatConversationForSummary(toSummarize)

	// Build focus instruction if provided
	focusInstruction := ""
	if req.FocusPrompt != "" {
		focusInstruction = fmt.Sprintf("SPECIAL FOCUS: %s\n", req.FocusPrompt)
	}

	// Create summarization prompt
	prompt := fmt.Sprintf(compactionPrompt, focusInstruction, conversationText)

	// Call LLM for summarization
	response, err := c.llmClient.Chat(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, nil, "You are a helpful assistant that creates concise, accurate summaries.")
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	summary := strings.TrimSpace(response.Content)
	summaryTokens := estimateTokens(summary)
	preservedTokens := calculateTokens(toPreserve)

	return &CompactResult{
		Summary:            summary,
		PreservedMsgs:      toPreserve,
		OriginalTokens:     originalTokens + preservedTokens,
		SummaryTokens:      summaryTokens + preservedTokens,
		TokensSaved:        originalTokens - summaryTokens,
		MessagesSummarized: len(toSummarize),
	}, nil
}

// formatConversationForSummary formats messages into a readable conversation format
func formatConversationForSummary(messages []llm.Message) string {
	var b strings.Builder

	for i, msg := range messages {
		// Capitalize first letter of role
		role := msg.Role
		if len(role) > 0 {
			role = strings.ToUpper(role[:1]) + role[1:]
		}
		content := msg.Content

		// Truncate very long messages for summarization
		if len(content) > 5000 {
			content = content[:5000] + "\n[... truncated for summarization ...]"
		}

		fmt.Fprintf(&b, "[%d] %s:\n%s\n\n", i+1, role, content)
	}

	return b.String()
}

// calculateTokens estimates total tokens for a slice of messages
func calculateTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += estimateTokens(msg.Content)
		total += 10 // Message structure overhead
	}
	return total
}
