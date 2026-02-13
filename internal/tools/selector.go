package tools

import (
	"strings"
)

// ToolSelector provides on-demand tool loading based on query content
// This reduces token usage by only including relevant tools
type ToolSelector struct {
	registry *Registry
}

// NewToolSelector creates a new tool selector
func NewToolSelector(registry *Registry) *ToolSelector {
	return &ToolSelector{registry: registry}
}

// ToolCategory represents a group of related tools
type ToolCategory string

const (
	CategoryCore     ToolCategory = "core"     // Always included: vecgrep, read_file, list_files, grep
	CategoryGit      ToolCategory = "git"      // Git tools: gpeek_*
	CategoryWrite    ToolCategory = "write"    // Write tools: write_file, edit_file
	CategoryExecute  ToolCategory = "execute"  // Execute tools: bash
	CategoryWeb      ToolCategory = "web"      // Web tools: web_search
	CategoryDev      ToolCategory = "dev"      // Dev tools: ast_parse, lsp_query, lint, test_run
	CategoryMemory   ToolCategory = "memory"   // Memory tools: noted_remember, noted_recall, noted_forget
)

// CoreTools are always included in tool selection
var CoreTools = []string{
	"vecgrep_search",
	"vecgrep_similar",
	"vecgrep_status",
	"vecgrep_index",
	"vecgrep_clean",
	"vecgrep_delete",
	"vecgrep_init",
	"read_file",
	"list_files",
	"grep",
}

// GitTools are included when query mentions git-related concepts
var GitTools = []string{
	"gpeek_status",
	"gpeek_diff",
	"gpeek_log",
	"gpeek_summary",
	"gpeek_blame",
	"gpeek_branches",
	"gpeek_stashes",
	"gpeek_tags",
	"gpeek_changes_between",
	"gpeek_conflict_check",
}

// WriteTools are included when query mentions writing/editing
var WriteTools = []string{
	"write_file",
	"edit_file",
}

// ExecuteTools are included when query mentions running/executing
var ExecuteTools = []string{
	"bash",
}

// WebTools are included when query mentions web/search
var WebTools = []string{
	"web_search",
}

// DevTools are included when query mentions development tasks
var DevTools = []string{
	"ast_parse",
	"lsp_query",
	"lint",
	"test_run",
}

// MemoryTools are included when query mentions memory/remember
var MemoryTools = []string{
	"noted_remember",
	"noted_recall",
	"noted_forget",
}

// gitKeywords trigger inclusion of git tools
var gitKeywords = []string{
	"git", "commit", "branch", "diff", "merge", "rebase", "stash",
	"blame", "history", "log", "tag", "changes", "conflict",
}

// writeKeywords trigger inclusion of write tools
var writeKeywords = []string{
	"write", "edit", "modify", "change", "update", "create", "add",
	"fix", "implement", "refactor",
}

// executeKeywords trigger inclusion of execute tools
var executeKeywords = []string{
	"run", "execute", "test", "build", "install", "npm", "go run",
	"make", "shell", "command", "script",
}

// webKeywords trigger inclusion of web tools
var webKeywords = []string{
	"search", "web", "internet", "online", "latest", "documentation",
	"api reference",
}

// devKeywords trigger inclusion of dev tools
var devKeywords = []string{
	"parse", "ast", "lint", "linter", "lsp", "symbol", "definition",
	"type check", "analyze", "test",
}

// memoryKeywords trigger inclusion of memory tools
var memoryKeywords = []string{
	"remember", "recall", "forget", "memory", "note", "store",
}

// SelectTools returns tool definitions based on query content
// This implements smart tool selection to reduce token usage
func (ts *ToolSelector) SelectTools(query string) []ToolDefinition {
	query = strings.ToLower(query)
	categories := ts.detectCategories(query)

	toolNames := make(map[string]bool)

	// Always include core tools
	for _, name := range CoreTools {
		toolNames[name] = true
	}

	// Add category-specific tools
	for _, cat := range categories {
		switch cat {
		case CategoryGit:
			for _, name := range GitTools {
				toolNames[name] = true
			}
		case CategoryWrite:
			for _, name := range WriteTools {
				toolNames[name] = true
			}
		case CategoryExecute:
			for _, name := range ExecuteTools {
				toolNames[name] = true
			}
		case CategoryWeb:
			for _, name := range WebTools {
				toolNames[name] = true
			}
		case CategoryDev:
			for _, name := range DevTools {
				toolNames[name] = true
			}
		case CategoryMemory:
			for _, name := range MemoryTools {
				toolNames[name] = true
			}
		}
	}

	// Build definitions from registry
	var defs []ToolDefinition
	for name := range toolNames {
		if tool, ok := ts.registry.Get(name); ok {
			defs = append(defs, ToolDefinition{
				Name:        tool.Name(),
				Description: tool.Description(),
				InputSchema: tool.InputSchema(),
			})
		}
	}

	return defs
}

// detectCategories analyzes query and returns relevant tool categories
func (ts *ToolSelector) detectCategories(query string) []ToolCategory {
	var categories []ToolCategory

	if containsAny(query, gitKeywords) {
		categories = append(categories, CategoryGit)
	}

	if containsAny(query, writeKeywords) {
		categories = append(categories, CategoryWrite)
	}

	if containsAny(query, executeKeywords) {
		categories = append(categories, CategoryExecute)
	}

	if containsAny(query, webKeywords) {
		categories = append(categories, CategoryWeb)
	}

	if containsAny(query, devKeywords) {
		categories = append(categories, CategoryDev)
	}

	if containsAny(query, memoryKeywords) {
		categories = append(categories, CategoryMemory)
	}

	return categories
}

// containsAny checks if text contains any of the keywords
func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// GetAllToolDefinitions returns all tool definitions (bypasses selection)
func (ts *ToolSelector) GetAllToolDefinitions() []ToolDefinition {
	return ts.registry.GetDefinitions()
}
