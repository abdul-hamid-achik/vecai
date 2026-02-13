package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// runNoted executes the noted CLI with the given arguments
func runNoted(ctx context.Context, args ...string) ([]byte, error) {
	// Apply a 30-second timeout to prevent runaway noted commands
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "noted", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("noted command failed: %w\nOutput: %s", err, string(output))
	}
	return output, nil
}

// NotedRememberTool stores memories using the noted CLI
type NotedRememberTool struct{}

func (t *NotedRememberTool) Name() string {
	return "noted_remember"
}

func (t *NotedRememberTool) Description() string {
	return "Store a memory with optional importance, tags, and TTL. Use for saving decisions, preferences, context, or important information for later recall."
}

func (t *NotedRememberTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content": map[string]any{
				"type":        "string",
				"description": "The content to remember. Can be any text you want to store for later recall.",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Categorization tags for filtering and organizing memories (e.g., [\"preference\", \"dark-mode\"]).",
			},
			"importance": map[string]any{
				"type":        "number",
				"description": "Importance level from 0.0 to 1.0. Higher importance memories are prioritized in recall. Default is 0.5.",
			},
			"ttl_hours": map[string]any{
				"type":        "integer",
				"description": "Time to live in hours. Memory expires after this duration. 0 means no expiration.",
			},
		},
		"required": []string{"content"},
	}
}

func (t *NotedRememberTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	content, ok := input["content"].(string)
	if !ok || content == "" {
		return "", fmt.Errorf("content is required")
	}

	args := []string{"remember", content}

	// Add tags if provided
	if tags, ok := input["tags"].([]any); ok && len(tags) > 0 {
		tagStrs := make([]string, 0, len(tags))
		for _, tag := range tags {
			if s, ok := tag.(string); ok {
				tagStrs = append(tagStrs, s)
			}
		}
		if len(tagStrs) > 0 {
			args = append(args, "--tags", strings.Join(tagStrs, ","))
		}
	}

	// Add importance if provided
	if importance, ok := input["importance"].(float64); ok {
		args = append(args, "--importance", fmt.Sprintf("%.2f", importance))
	}

	// Add TTL if provided
	if ttl, ok := input["ttl_hours"].(float64); ok && ttl > 0 {
		args = append(args, "--ttl", fmt.Sprintf("%dh", int(ttl)))
	}

	output, err := runNoted(ctx, args...)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func (t *NotedRememberTool) Permission() PermissionLevel {
	return PermissionWrite
}

// NotedRecallTool searches memories using the noted CLI
type NotedRecallTool struct{}

func (t *NotedRecallTool) Name() string {
	return "noted_recall"
}

func (t *NotedRecallTool) Description() string {
	return "Search memories semantically by query. Returns memories ranked by relevance. Use for retrieving stored decisions, preferences, or context."
}

func (t *NotedRecallTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Natural language search query to find relevant memories.",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Filter results to only include memories with these tags.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return. Default is 10.",
			},
			"min_importance": map[string]any{
				"type":        "number",
				"description": "Minimum importance threshold. Only return memories with importance >= this value.",
			},
		},
		"required": []string{"query"},
	}
}

func (t *NotedRecallTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	query, ok := input["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query is required")
	}

	args := []string{"recall", query, "--format", "json"}

	// Add tags filter if provided
	if tags, ok := input["tags"].([]any); ok && len(tags) > 0 {
		tagStrs := make([]string, 0, len(tags))
		for _, tag := range tags {
			if s, ok := tag.(string); ok {
				tagStrs = append(tagStrs, s)
			}
		}
		if len(tagStrs) > 0 {
			args = append(args, "--tags", strings.Join(tagStrs, ","))
		}
	}

	// Add limit if provided
	if limit, ok := input["limit"].(float64); ok && limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", int(limit)))
	}

	// Add min_importance if provided
	if minImp, ok := input["min_importance"].(float64); ok {
		args = append(args, "--min-importance", fmt.Sprintf("%.2f", minImp))
	}

	output, err := runNoted(ctx, args...)
	if err != nil {
		return "", err
	}

	// Try to parse as JSON and format nicely
	var memories []map[string]any
	if err := json.Unmarshal(output, &memories); err == nil {
		var result strings.Builder
		for i, mem := range memories {
			result.WriteString(fmt.Sprintf("Memory %d:\n", i+1))
			if content, ok := mem["content"].(string); ok {
				result.WriteString(fmt.Sprintf("  Content: %s\n", content))
			}
			if tags, ok := mem["tags"].([]any); ok && len(tags) > 0 {
				result.WriteString(fmt.Sprintf("  Tags: %v\n", tags))
			}
			if imp, ok := mem["importance"].(float64); ok {
				result.WriteString(fmt.Sprintf("  Importance: %.2f\n", imp))
			}
			result.WriteString("\n")
		}
		if result.Len() == 0 {
			return "No memories found matching the query.", nil
		}
		return result.String(), nil
	}

	return string(output), nil
}

func (t *NotedRecallTool) Permission() PermissionLevel {
	return PermissionRead
}

// NotedForgetTool deletes memories using the noted CLI
type NotedForgetTool struct{}

func (t *NotedForgetTool) Name() string {
	return "noted_forget"
}

func (t *NotedForgetTool) Description() string {
	return "Delete memories by ID, tags, or age. Use to remove outdated or incorrect stored information."
}

func (t *NotedForgetTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "integer",
				"description": "Delete a specific memory by its ID.",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Delete all memories that have any of these tags.",
			},
			"older_than_hours": map[string]any{
				"type":        "integer",
				"description": "Delete memories older than this many hours.",
			},
			"confirm": map[string]any{
				"type":        "string",
				"description": "Set to 'yes' to confirm bulk deletion (required when deleting by tags or age).",
			},
		},
	}
}

func (t *NotedForgetTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	args := []string{"forget"}

	hasFilter := false

	// Add ID if provided
	if id, ok := input["id"].(float64); ok {
		args = append(args, "--id", fmt.Sprintf("%d", int(id)))
		hasFilter = true
	}

	// Add tags if provided
	if tags, ok := input["tags"].([]any); ok && len(tags) > 0 {
		tagStrs := make([]string, 0, len(tags))
		for _, tag := range tags {
			if s, ok := tag.(string); ok {
				tagStrs = append(tagStrs, s)
			}
		}
		if len(tagStrs) > 0 {
			args = append(args, "--tags", strings.Join(tagStrs, ","))
			hasFilter = true
		}
	}

	// Add older_than_hours if provided
	if hours, ok := input["older_than_hours"].(float64); ok && hours > 0 {
		args = append(args, "--older-than", fmt.Sprintf("%dh", int(hours)))
		hasFilter = true
	}

	if !hasFilter {
		return "", fmt.Errorf("at least one of id, tags, or older_than_hours is required")
	}

	// Add confirmation for bulk deletions
	if confirm, ok := input["confirm"].(string); ok && confirm == "yes" {
		args = append(args, "--confirm", "yes")
	}

	output, err := runNoted(ctx, args...)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func (t *NotedForgetTool) Permission() PermissionLevel {
	return PermissionWrite
}
