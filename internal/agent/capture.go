package agent

import (
	"context"
	"fmt"
	"strings"
)

// offerCapture prompts the user to save a response to notes
func (a *Agent) offerCapture(ctx context.Context, query, response string) error {
	// Skip if noted is not available
	tool, ok := a.tools.Get("noted_remember")
	if !ok {
		return nil
	}

	// Prompt user
	a.output.TextLn("")
	input, err := a.input.ReadInput("Save to notes? [y/N/e(dit)] ")
	if err != nil {
		return err
	}

	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" || input == "n" || input == "no" {
		return nil
	}

	// Auto-generate tags from query
	tags := generateTags(query)

	content := response
	if input == "e" || input == "edit" {
		// Let user edit the content
		content, err = a.input.ReadMultiLine("Enter content (empty line to finish): ")
		if err != nil {
			return err
		}
	}

	// Save to notes
	_, err = tool.Execute(ctx, map[string]any{
		"content":    content,
		"tags":       tags,
		"importance": 0.6, // Slightly above default for user-saved content
	})
	if err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	a.output.Success("Saved to notes")
	return nil
}

// generateTags creates tags from a query string
func generateTags(query string) []any {
	queryLower := strings.ToLower(query)

	var tags []any

	// Category tags based on keywords
	if strings.Contains(queryLower, "config") || strings.Contains(queryLower, "setting") {
		tags = append(tags, "config")
	}
	if strings.Contains(queryLower, "code") || strings.Contains(queryLower, "function") || strings.Contains(queryLower, "implement") {
		tags = append(tags, "code")
	}
	if strings.Contains(queryLower, "bug") || strings.Contains(queryLower, "fix") || strings.Contains(queryLower, "error") {
		tags = append(tags, "debug")
	}
	if strings.Contains(queryLower, "prefer") || strings.Contains(queryLower, "like") || strings.Contains(queryLower, "want") {
		tags = append(tags, "preference")
	}
	if strings.Contains(queryLower, "remember") || strings.Contains(queryLower, "note") {
		tags = append(tags, "note")
	}

	// Always add a general tag if no specific ones matched
	if len(tags) == 0 {
		tags = append(tags, "general")
	}

	return tags
}

// detectAndRecordCorrection checks if user message looks like a correction and records it
func (a *Agent) detectAndRecordCorrection(userMsg string) {
	if a.memoryLayer == nil {
		return
	}
	patterns := []string{"no,", "wrong", "that's not", "actually", "instead", "should be", "not correct", "incorrect"}
	lower := strings.ToLower(userMsg)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			a.memoryLayer.RecordError("agent_correction", userMsg)
			return
		}
	}
}
