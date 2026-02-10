package context

import (
	"fmt"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/llm"
)

// DefaultPreserveRecent is the number of recent tool results to keep unmasked.
const DefaultPreserveRecent = 4

// MaskOldToolResults replaces old tool result content with a short summary
// to reduce context usage while preserving ToolCallID linkage for Ollama's
// tool protocol. The last `preserveRecent` tool results are kept verbatim.
func MaskOldToolResults(messages []llm.Message, preserveRecent int) []llm.Message {
	if preserveRecent <= 0 {
		preserveRecent = DefaultPreserveRecent
	}

	// Find indices of all tool result messages (role="tool")
	var toolIndices []int
	for i, msg := range messages {
		if msg.Role == "tool" {
			toolIndices = append(toolIndices, i)
		}
	}

	// Nothing to mask if we have fewer tool results than the preserve threshold
	if len(toolIndices) <= preserveRecent {
		return messages
	}

	// Indices to mask: all except the last `preserveRecent`
	maskUpTo := len(toolIndices) - preserveRecent
	maskSet := make(map[int]bool, maskUpTo)
	for _, idx := range toolIndices[:maskUpTo] {
		maskSet[idx] = true
	}

	// Create a new slice with masked messages
	result := make([]llm.Message, len(messages))
	for i, msg := range messages {
		if maskSet[i] {
			result[i] = llm.Message{
				Role:       msg.Role,
				Content:    maskContent(msg.Content),
				ToolCallID: msg.ToolCallID, // Preserve linkage
			}
		} else {
			result[i] = msg
		}
	}

	return result
}

// maskContent creates a short masked summary of tool output.
func maskContent(content string) string {
	lines := strings.Count(content, "\n") + 1
	preview := content
	if idx := strings.IndexByte(content, '\n'); idx > 0 {
		preview = content[:idx]
	}
	if len(preview) > 80 {
		preview = preview[:77] + "..."
	}
	return fmt.Sprintf("[Masked: %d lines, preview: %s]", lines, preview)
}
