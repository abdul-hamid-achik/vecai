package tools

import (
	"testing"
)

func TestNewToolSelector(t *testing.T) {
	registry := NewRegistry(nil)
	selector := NewToolSelector(registry)

	if selector == nil {
		t.Fatal("expected non-nil selector")
	}
}

func TestToolSelector_SelectTools_CoreOnly(t *testing.T) {
	registry := NewRegistry(nil)
	selector := NewToolSelector(registry)

	// Query with no special keywords should only return core tools
	tools := selector.SelectTools("explain this code")

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	// Should have core tools
	for _, core := range CoreTools {
		if !toolNames[core] {
			t.Errorf("expected core tool %s to be included", core)
		}
	}

	// Should NOT have git tools
	for _, git := range GitTools {
		if toolNames[git] {
			t.Errorf("expected git tool %s to NOT be included for non-git query", git)
		}
	}
}

func TestToolSelector_SelectTools_GitKeywords(t *testing.T) {
	registry := NewRegistry(nil)
	selector := NewToolSelector(registry)

	gitQueries := []string{
		"show git history",
		"what changed in the last commit",
		"show me the diff",
		"which branch is this",
		"who modified this file (blame)",
	}

	for _, query := range gitQueries {
		t.Run(query, func(t *testing.T) {
			tools := selector.SelectTools(query)

			toolNames := make(map[string]bool)
			for _, tool := range tools {
				toolNames[tool.Name] = true
			}

			// Should have at least some git tools
			hasGitTool := false
			for _, git := range GitTools {
				if toolNames[git] {
					hasGitTool = true
					break
				}
			}

			if !hasGitTool {
				t.Errorf("expected git tools for query %q", query)
			}
		})
	}
}

func TestToolSelector_SelectTools_WriteKeywords(t *testing.T) {
	registry := NewRegistry(nil)
	selector := NewToolSelector(registry)

	writeQueries := []string{
		"write a new function",
		"edit this file",
		"modify the configuration",
		"fix this bug",
		"implement the feature",
	}

	for _, query := range writeQueries {
		t.Run(query, func(t *testing.T) {
			tools := selector.SelectTools(query)

			toolNames := make(map[string]bool)
			for _, tool := range tools {
				toolNames[tool.Name] = true
			}

			// Should have write tools
			hasWriteTool := false
			for _, write := range WriteTools {
				if toolNames[write] {
					hasWriteTool = true
					break
				}
			}

			if !hasWriteTool {
				t.Errorf("expected write tools for query %q", query)
			}
		})
	}
}

func TestToolSelector_SelectTools_ExecuteKeywords(t *testing.T) {
	registry := NewRegistry(nil)
	selector := NewToolSelector(registry)

	executeQueries := []string{
		"run the tests",
		"execute the build",
		"npm install dependencies",
		"make the project",
	}

	for _, query := range executeQueries {
		t.Run(query, func(t *testing.T) {
			tools := selector.SelectTools(query)

			toolNames := make(map[string]bool)
			for _, tool := range tools {
				toolNames[tool.Name] = true
			}

			// Should have execute tools
			if !toolNames["bash"] {
				t.Errorf("expected bash tool for query %q", query)
			}
		})
	}
}

func TestToolSelector_SelectTools_CaseInsensitive(t *testing.T) {
	registry := NewRegistry(nil)
	selector := NewToolSelector(registry)

	// Test case insensitivity
	tools1 := selector.SelectTools("show GIT history")
	tools2 := selector.SelectTools("show git history")

	if len(tools1) != len(tools2) {
		t.Errorf("expected same tools regardless of case: %d vs %d", len(tools1), len(tools2))
	}
}

func TestToolSelector_GetAllToolDefinitions(t *testing.T) {
	registry := NewRegistry(nil)
	selector := NewToolSelector(registry)

	allTools := selector.GetAllToolDefinitions()
	selectedTools := selector.SelectTools("explain code")

	// All tools should be >= selected tools
	if len(allTools) < len(selectedTools) {
		t.Errorf("GetAllToolDefinitions should return more tools than SelectTools")
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		text     string
		keywords []string
		expected bool
	}{
		{"show git history", []string{"git", "commit"}, true},
		{"explain this code", []string{"git", "commit"}, false},
		{"run the tests", []string{"run", "execute"}, true},
		{"", []string{"git"}, false},
		{"git", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if got := containsAny(tt.text, tt.keywords); got != tt.expected {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.text, tt.keywords, got, tt.expected)
			}
		})
	}
}

func TestNewAnalysisRegistry(t *testing.T) {
	registry := NewAnalysisRegistry(nil)

	tools := registry.List()

	// Should have read-only tools
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name()] = true
	}

	// Should have core read tools
	expectedTools := []string{"vecgrep_search", "read_file", "list_files", "grep"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("expected analysis registry to have %s", name)
		}
	}

	// Should NOT have write/execute tools
	excludedTools := []string{"write_file", "edit_file", "bash"}
	for _, name := range excludedTools {
		if toolNames[name] {
			t.Errorf("expected analysis registry to NOT have %s", name)
		}
	}
}
