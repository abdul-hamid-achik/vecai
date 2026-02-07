package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// --- Mock output/input for tests ---

type mockOutput struct {
	mu              sync.Mutex
	activityCalls   []string
	toolCallCalls   []string
	toolResultCalls []string
	errorCalls      []string
}

func (m *mockOutput) StreamText(_ string)                        {}
func (m *mockOutput) StreamThinking(_ string)                    {}
func (m *mockOutput) StreamDone()                                {}
func (m *mockOutput) StreamDoneWithUsage(_, _ int64)             {}
func (m *mockOutput) Text(_ string)                              {}
func (m *mockOutput) TextLn(_ string)                            {}
func (m *mockOutput) Error(_ error)                              {}
func (m *mockOutput) ErrorStr(msg string)                        { m.mu.Lock(); m.errorCalls = append(m.errorCalls, msg); m.mu.Unlock() }
func (m *mockOutput) Warning(_ string)                           {}
func (m *mockOutput) Success(_ string)                           {}
func (m *mockOutput) Info(_ string)                              {}
func (m *mockOutput) Header(_ string)                            {}
func (m *mockOutput) Separator()                                 {}
func (m *mockOutput) Thinking(_ string)                          {}
func (m *mockOutput) ThinkingLn(_ string)                        {}
func (m *mockOutput) ModelInfo(_ string)                         {}
func (m *mockOutput) Done()                                      {}
func (m *mockOutput) PermissionPrompt(_ string, _ tools.PermissionLevel, _ string) {}

func (m *mockOutput) Activity(msg string) {
	m.mu.Lock()
	m.activityCalls = append(m.activityCalls, msg)
	m.mu.Unlock()
}

func (m *mockOutput) ToolCall(name, description string) {
	m.mu.Lock()
	m.toolCallCalls = append(m.toolCallCalls, name+":"+description)
	m.mu.Unlock()
}

func (m *mockOutput) ToolResult(name, result string, isError bool) {
	m.mu.Lock()
	tag := "ok"
	if isError {
		tag = "err"
	}
	m.toolResultCalls = append(m.toolResultCalls, fmt.Sprintf("%s:%s:%s", name, tag, result))
	m.mu.Unlock()
}

type mockInput struct {
	response string
}

func (m *mockInput) ReadLine(_ string) (string, error) { return m.response, nil }
func (m *mockInput) Confirm(_ string, _ bool) (bool, error) { return true, nil }

// --- Mock tools ---

type mockReadTool struct{ name string }

func (t *mockReadTool) Name() string                                                    { return t.name }
func (t *mockReadTool) Description() string                                             { return "mock read tool" }
func (t *mockReadTool) InputSchema() map[string]any                                     { return nil }
func (t *mockReadTool) Execute(_ context.Context, _ map[string]any) (string, error)     { return "read-result", nil }
func (t *mockReadTool) Permission() tools.PermissionLevel                               { return tools.PermissionRead }

type mockWriteTool struct{ name string }

func (t *mockWriteTool) Name() string                                                   { return t.name }
func (t *mockWriteTool) Description() string                                            { return "mock write tool" }
func (t *mockWriteTool) InputSchema() map[string]any                                    { return nil }
func (t *mockWriteTool) Execute(_ context.Context, _ map[string]any) (string, error)    { return "write-result", nil }
func (t *mockWriteTool) Permission() tools.PermissionLevel                              { return tools.PermissionWrite }

// mockSlowTool adds a small delay to help verify concurrent execution.
type mockSlowTool struct {
	name  string
	delay time.Duration
}

func (t *mockSlowTool) Name() string                                                 { return t.name }
func (t *mockSlowTool) Description() string                                          { return "mock slow tool" }
func (t *mockSlowTool) InputSchema() map[string]any                                  { return nil }
func (t *mockSlowTool) Execute(_ context.Context, _ map[string]any) (string, error) {
	time.Sleep(t.delay)
	return "slow-result-" + t.name, nil
}
func (t *mockSlowTool) Permission() tools.PermissionLevel { return tools.PermissionRead }

// --- Helpers ---

func newTestToolExecutor(registry *tools.Registry, mode permissions.Mode) *ToolExecutor {
	policy := permissions.NewPolicy(mode, nil, nil)
	return NewToolExecutor(registry, policy, nil, false)
}

// --- canParallelize tests ---

func TestCanParallelize(t *testing.T) {
	readToolA := &mockReadTool{name: "tool_a"}
	readToolB := &mockReadTool{name: "tool_b"}
	writeTool := &mockWriteTool{name: "writer"}

	registry := newMockRegistry(readToolA, readToolB, writeTool)

	t.Run("false for zero calls", func(t *testing.T) {
		te := newTestToolExecutor(registry, permissions.ModeAuto)
		if te.canParallelize(nil) {
			t.Error("expected false for zero calls")
		}
	})

	t.Run("false for one call", func(t *testing.T) {
		te := newTestToolExecutor(registry, permissions.ModeAuto)
		calls := []llm.ToolCall{{Name: "tool_a"}}
		if te.canParallelize(calls) {
			t.Error("expected false for single call")
		}
	})

	t.Run("false for ModeAsk", func(t *testing.T) {
		te := newTestToolExecutor(registry, permissions.ModeAsk)
		calls := []llm.ToolCall{{Name: "tool_a"}, {Name: "tool_b"}}
		if te.canParallelize(calls) {
			t.Error("expected false for ModeAsk")
		}
	})

	t.Run("false when write tool present", func(t *testing.T) {
		te := newTestToolExecutor(registry, permissions.ModeAuto)
		calls := []llm.ToolCall{{Name: "tool_a"}, {Name: "writer"}}
		if te.canParallelize(calls) {
			t.Error("expected false when write tool present")
		}
	})

	t.Run("false for unknown tool", func(t *testing.T) {
		te := newTestToolExecutor(registry, permissions.ModeAuto)
		calls := []llm.ToolCall{{Name: "tool_a"}, {Name: "nonexistent"}}
		if te.canParallelize(calls) {
			t.Error("expected false for unknown tool")
		}
	})

	t.Run("true for 2+ read-only in ModeAuto", func(t *testing.T) {
		te := newTestToolExecutor(registry, permissions.ModeAuto)
		calls := []llm.ToolCall{{Name: "tool_a"}, {Name: "tool_b"}}
		if !te.canParallelize(calls) {
			t.Error("expected true for 2 read-only tools in ModeAuto")
		}
	})
}

// --- ExecuteToolCalls tests ---

func TestExecuteToolCalls(t *testing.T) {
	readToolA := &mockReadTool{name: "tool_a"}
	readToolB := &mockReadTool{name: "tool_b"}
	writeTool := &mockWriteTool{name: "writer"}

	t.Run("parallel path shows activity message", func(t *testing.T) {
		registry := newMockRegistry(readToolA, readToolB)
		te := newTestToolExecutor(registry, permissions.ModeAuto)
		output := &mockOutput{}
		input := &mockInput{}

		calls := []llm.ToolCall{
			{ID: "c1", Name: "tool_a", Input: map[string]any{}},
			{ID: "c2", Name: "tool_b", Input: map[string]any{}},
		}

		results := te.ExecuteToolCalls(context.Background(), calls, output, input)

		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		// The parallel path calls Activity(...)
		if len(output.activityCalls) == 0 {
			t.Error("expected Activity call for parallel path, got none")
		}
	})

	t.Run("sequential path for write tool", func(t *testing.T) {
		registry := newMockRegistry(readToolA, writeTool)
		te := newTestToolExecutor(registry, permissions.ModeAuto)
		output := &mockOutput{}
		input := &mockInput{}

		calls := []llm.ToolCall{
			{ID: "c1", Name: "tool_a", Input: map[string]any{}},
			{ID: "c2", Name: "writer", Input: map[string]any{}},
		}

		results := te.ExecuteToolCalls(context.Background(), calls, output, input)

		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		// Sequential path does NOT call Activity
		if len(output.activityCalls) != 0 {
			t.Errorf("expected no Activity calls for sequential path, got %d", len(output.activityCalls))
		}
	})

	t.Run("unknown tool returns error result", func(t *testing.T) {
		registry := newMockRegistry(readToolA)
		te := newTestToolExecutor(registry, permissions.ModeAuto)
		output := &mockOutput{}
		input := &mockInput{}

		calls := []llm.ToolCall{
			{ID: "c1", Name: "nonexistent", Input: map[string]any{}},
		}

		results := te.ExecuteToolCalls(context.Background(), calls, output, input)

		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if !results[0].Error {
			t.Error("expected error for unknown tool")
		}
		if results[0].ToolCallID != "c1" {
			t.Errorf("expected ToolCallID 'c1', got '%s'", results[0].ToolCallID)
		}
	})
}

// --- executeParallel tests ---

func TestExecuteParallel(t *testing.T) {
	t.Run("results in order with ToolCallIDs set", func(t *testing.T) {
		toolA := &mockReadTool{name: "tool_a"}
		toolB := &mockReadTool{name: "tool_b"}
		registry := newMockRegistry(toolA, toolB)
		te := newTestToolExecutor(registry, permissions.ModeAuto)
		output := &mockOutput{}

		calls := []llm.ToolCall{
			{ID: "id-1", Name: "tool_a", Input: map[string]any{}},
			{ID: "id-2", Name: "tool_b", Input: map[string]any{}},
		}

		results := te.executeParallel(context.Background(), calls, output)

		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		// Verify order: results[0] should be from tool_a, results[1] from tool_b
		if results[0].Name != "tool_a" {
			t.Errorf("expected results[0].Name = 'tool_a', got '%s'", results[0].Name)
		}
		if results[1].Name != "tool_b" {
			t.Errorf("expected results[1].Name = 'tool_b', got '%s'", results[1].Name)
		}

		// ToolCallIDs set from original calls
		if results[0].ToolCallID != "id-1" {
			t.Errorf("expected results[0].ToolCallID = 'id-1', got '%s'", results[0].ToolCallID)
		}
		if results[1].ToolCallID != "id-2" {
			t.Errorf("expected results[1].ToolCallID = 'id-2', got '%s'", results[1].ToolCallID)
		}
	})

	t.Run("results not marked as error on success", func(t *testing.T) {
		toolA := &mockReadTool{name: "tool_a"}
		toolB := &mockReadTool{name: "tool_b"}
		registry := newMockRegistry(toolA, toolB)
		te := newTestToolExecutor(registry, permissions.ModeAuto)
		output := &mockOutput{}

		calls := []llm.ToolCall{
			{ID: "id-1", Name: "tool_a", Input: map[string]any{}},
			{ID: "id-2", Name: "tool_b", Input: map[string]any{}},
		}

		results := te.executeParallel(context.Background(), calls, output)

		for i, r := range results {
			if r.Error {
				t.Errorf("results[%d] should not have error flag", i)
			}
		}
	})

	t.Run("output shows tool calls and results", func(t *testing.T) {
		toolA := &mockReadTool{name: "tool_a"}
		toolB := &mockReadTool{name: "tool_b"}
		registry := newMockRegistry(toolA, toolB)
		te := newTestToolExecutor(registry, permissions.ModeAuto)
		output := &mockOutput{}

		calls := []llm.ToolCall{
			{ID: "id-1", Name: "tool_a", Input: map[string]any{}},
			{ID: "id-2", Name: "tool_b", Input: map[string]any{}},
		}

		te.executeParallel(context.Background(), calls, output)

		// Should have 2 ToolCall calls and 2 ToolResult calls
		if len(output.toolCallCalls) != 2 {
			t.Errorf("expected 2 ToolCall calls, got %d", len(output.toolCallCalls))
		}
		if len(output.toolResultCalls) != 2 {
			t.Errorf("expected 2 ToolResult calls, got %d", len(output.toolResultCalls))
		}
	})
}

// --- Helper: create a registry with only the given tools ---

func newMockRegistry(mockTools ...tools.Tool) *tools.Registry {
	r := tools.NewEmptyRegistry()
	for _, t := range mockTools {
		r.Register(t)
	}
	return r
}
