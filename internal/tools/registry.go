package tools

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// PermissionLevel defines the level of permission required for a tool
type PermissionLevel int

const (
	PermissionRead    PermissionLevel = 0 // Read-only operations
	PermissionWrite   PermissionLevel = 1 // File modifications
	PermissionExecute PermissionLevel = 2 // Shell execution
)

func (p PermissionLevel) String() string {
	switch p {
	case PermissionRead:
		return "read"
	case PermissionWrite:
		return "write"
	case PermissionExecute:
		return "execute"
	default:
		return "unknown"
	}
}

// Tool defines the interface all tools must implement
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	Execute(ctx context.Context, input map[string]any) (string, error)
	Permission() PermissionLevel
}

// Registry manages available tools
type Registry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry with default tools
func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]Tool),
	}

	// Register default tools
	r.Register(&VecgrepSearchTool{})
	r.Register(&VecgrepSimilarTool{})
	r.Register(&VecgrepStatusTool{})
	r.Register(&ReadFileTool{})
	r.Register(&WriteFileTool{})
	r.Register(&EditFileTool{})
	r.Register(&ListFilesTool{})
	r.Register(&BashTool{})
	r.Register(&GrepTool{})

	// Register gpeek tools
	r.Register(&GpeekStatusTool{})
	r.Register(&GpeekDiffTool{})
	r.Register(&GpeekLogTool{})
	r.Register(&GpeekSummaryTool{})
	r.Register(&GpeekBlameTool{})
	r.Register(&GpeekBranchesTool{})
	r.Register(&GpeekStashesTool{})
	r.Register(&GpeekTagsTool{})
	r.Register(&GpeekChangesBetweenTool{})
	r.Register(&GpeekConflictCheckTool{})

	// Register web search tool (conditionally if API key available)
	if os.Getenv("TAVILY_API_KEY") != "" {
		r.Register(NewWebSearchTool())
	}

	return r
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// Get returns a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Execute runs a tool by name with the given input
func (r *Registry) Execute(ctx context.Context, name string, input map[string]any) (string, error) {
	tool, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Execute(ctx, input)
}

// GetDefinitions returns tool definitions for the LLM
func (r *Registry) GetDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return defs
}

// ToolDefinition is used to pass tool info to the LLM
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
}
