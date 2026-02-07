package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// --- mockErrorTool always returns an error ---

type mockErrorTool struct{ name string }

func (t *mockErrorTool) Name() string                                                { return t.name }
func (t *mockErrorTool) Description() string                                         { return "error tool" }
func (t *mockErrorTool) InputSchema() map[string]any                                 { return nil }
func (t *mockErrorTool) Execute(_ context.Context, _ map[string]any) (string, error) { return "", fmt.Errorf("tool failed") }
func (t *mockErrorTool) Permission() tools.PermissionLevel                           { return tools.PermissionRead }

// --- mockConcurrencyTool tracks concurrent execution ---

type mockConcurrencyTool struct {
	name    string
	maxSeen *atomic.Int32
	current *atomic.Int32
	delay   time.Duration
}

func (t *mockConcurrencyTool) Name() string                { return t.name }
func (t *mockConcurrencyTool) Description() string         { return "concurrency tool" }
func (t *mockConcurrencyTool) InputSchema() map[string]any { return nil }
func (t *mockConcurrencyTool) Execute(_ context.Context, _ map[string]any) (string, error) {
	cur := t.current.Add(1)
	for {
		old := t.maxSeen.Load()
		if cur <= old || t.maxSeen.CompareAndSwap(old, cur) {
			break
		}
	}
	time.Sleep(t.delay)
	t.current.Add(-1)
	return "ok-" + t.name, nil
}
func (t *mockConcurrencyTool) Permission() tools.PermissionLevel { return tools.PermissionRead }

// --- Tests ---

func TestNewParallelExecutor(t *testing.T) {
	registry := tools.NewEmptyRegistry()

	t.Run("default concurrency for zero", func(t *testing.T) {
		pe := newParallelExecutor(registry, 0)
		if pe.maxConcurrency != defaultMaxConcurrency {
			t.Errorf("expected default %d, got %d", defaultMaxConcurrency, pe.maxConcurrency)
		}
	})

	t.Run("default concurrency for negative", func(t *testing.T) {
		pe := newParallelExecutor(registry, -1)
		if pe.maxConcurrency != defaultMaxConcurrency {
			t.Errorf("expected default %d, got %d", defaultMaxConcurrency, pe.maxConcurrency)
		}
	})

	t.Run("custom concurrency", func(t *testing.T) {
		pe := newParallelExecutor(registry, 8)
		if pe.maxConcurrency != 8 {
			t.Errorf("expected 8, got %d", pe.maxConcurrency)
		}
	})

	t.Run("nil registry accepted", func(t *testing.T) {
		pe := newParallelExecutor(nil, 0)
		if pe.maxConcurrency != defaultMaxConcurrency {
			t.Errorf("expected default %d, got %d", defaultMaxConcurrency, pe.maxConcurrency)
		}
	})
}

func TestParallelExecutor_PermissionDenied(t *testing.T) {
	toolA := &mockReadTool{name: "tool_a"}
	toolB := &mockReadTool{name: "tool_b"}
	registry := newMockRegistry(toolA, toolB)

	pe := newParallelExecutor(registry, 4)

	calls := []llm.ToolCall{
		{ID: "c1", Name: "tool_a"},
		{ID: "c2", Name: "tool_b"},
	}

	// Deny tool_b only
	results := pe.executeParallel(context.Background(), calls,
		func(name string) (bool, error) {
			return name == "tool_a", nil
		},
		nil,
	)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// tool_a should succeed
	if results[0].Error {
		t.Errorf("tool_a should have succeeded, got error: %s", results[0].Result)
	}

	// tool_b should be denied
	if !results[1].Error {
		t.Error("tool_b should have been denied")
	}
	if results[1].Result != "Permission denied" {
		t.Errorf("expected 'Permission denied', got '%s'", results[1].Result)
	}
}

func TestParallelExecutor_AllApproved(t *testing.T) {
	toolA := &mockSlowTool{name: "slow_a", delay: 10 * time.Millisecond}
	toolB := &mockSlowTool{name: "slow_b", delay: 10 * time.Millisecond}
	registry := newMockRegistry(toolA, toolB)

	pe := newParallelExecutor(registry, 4)

	calls := []llm.ToolCall{
		{ID: "c1", Name: "slow_a"},
		{ID: "c2", Name: "slow_b"},
	}

	start := time.Now()
	results := pe.executeParallel(context.Background(), calls,
		func(_ string) (bool, error) { return true, nil },
		nil,
	)
	elapsed := time.Since(start)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for i, r := range results {
		if r.Error {
			t.Errorf("results[%d] should not be error: %s", i, r.Result)
		}
	}

	// If truly parallel, should take roughly 1x delay, not 2x
	if elapsed > 50*time.Millisecond {
		t.Errorf("execution took %v, expected parallel execution to be faster", elapsed)
	}
}

func TestParallelExecutor_ResultOrdering(t *testing.T) {
	// slow_a takes longer but is first in list
	toolA := &mockSlowTool{name: "slow_a", delay: 30 * time.Millisecond}
	toolB := &mockSlowTool{name: "slow_b", delay: 5 * time.Millisecond}
	registry := newMockRegistry(toolA, toolB)

	pe := newParallelExecutor(registry, 4)

	calls := []llm.ToolCall{
		{ID: "c1", Name: "slow_a"},
		{ID: "c2", Name: "slow_b"},
	}

	results := pe.executeParallel(context.Background(), calls,
		func(_ string) (bool, error) { return true, nil },
		nil,
	)

	// Even though slow_b finishes first, results should match call order
	if results[0].Name != "slow_a" {
		t.Errorf("expected results[0].Name = 'slow_a', got '%s'", results[0].Name)
	}
	if results[1].Name != "slow_b" {
		t.Errorf("expected results[1].Name = 'slow_b', got '%s'", results[1].Name)
	}

	// Verify result content matches the tool
	if results[0].Result != "slow-result-slow_a" {
		t.Errorf("expected results[0].Result = 'slow-result-slow_a', got '%s'", results[0].Result)
	}
	if results[1].Result != "slow-result-slow_b" {
		t.Errorf("expected results[1].Result = 'slow-result-slow_b', got '%s'", results[1].Result)
	}
}

func TestParallelExecutor_ErrorFromTool(t *testing.T) {
	goodTool := &mockReadTool{name: "good"}
	badTool := &mockErrorTool{name: "bad"}
	registry := newMockRegistry(goodTool, badTool)

	pe := newParallelExecutor(registry, 4)

	calls := []llm.ToolCall{
		{ID: "c1", Name: "good"},
		{ID: "c2", Name: "bad"},
	}

	results := pe.executeParallel(context.Background(), calls,
		func(_ string) (bool, error) { return true, nil },
		nil,
	)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// good tool succeeds
	if results[0].Error {
		t.Errorf("good tool should have succeeded, got: %s", results[0].Result)
	}

	// bad tool returns error
	if !results[1].Error {
		t.Error("bad tool should have error flag set")
	}
	if results[1].Result != "Error: tool failed" {
		t.Errorf("expected 'Error: tool failed', got '%s'", results[1].Result)
	}
}

func TestParallelExecutor_OnResultCallback(t *testing.T) {
	toolA := &mockReadTool{name: "tool_a"}
	toolB := &mockReadTool{name: "tool_b"}
	registry := newMockRegistry(toolA, toolB)

	pe := newParallelExecutor(registry, 4)

	calls := []llm.ToolCall{
		{ID: "c1", Name: "tool_a"},
		{ID: "c2", Name: "tool_b"},
	}

	var mu sync.Mutex
	var callbackNames []string
	var callbackErrors []error

	results := pe.executeParallel(context.Background(), calls,
		func(_ string) (bool, error) { return true, nil },
		func(name, result string, err error) {
			mu.Lock()
			defer mu.Unlock()
			callbackNames = append(callbackNames, name)
			callbackErrors = append(callbackErrors, err)
		},
	)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if len(callbackNames) != 2 {
		t.Errorf("expected 2 callback invocations, got %d", len(callbackNames))
	}

	// Both callbacks should have nil errors
	for i, err := range callbackErrors {
		if err != nil {
			t.Errorf("callback[%d] unexpected error: %v", i, err)
		}
	}
}

func TestParallelExecutor_OnResultCallbackWithError(t *testing.T) {
	goodTool := &mockReadTool{name: "good"}
	badTool := &mockErrorTool{name: "bad"}
	registry := newMockRegistry(goodTool, badTool)

	pe := newParallelExecutor(registry, 4)

	calls := []llm.ToolCall{
		{ID: "c1", Name: "good"},
		{ID: "c2", Name: "bad"},
	}

	var mu sync.Mutex
	callbackErrs := make(map[string]error)

	pe.executeParallel(context.Background(), calls,
		func(_ string) (bool, error) { return true, nil },
		func(name, result string, err error) {
			mu.Lock()
			defer mu.Unlock()
			callbackErrs[name] = err
		},
	)

	if callbackErrs["good"] != nil {
		t.Error("good tool callback should have nil error")
	}
	if callbackErrs["bad"] == nil {
		t.Error("bad tool callback should have non-nil error")
	}
}

func TestParallelExecutor_SemaphoreRespectsMaxConcurrency(t *testing.T) {
	maxSeen := &atomic.Int32{}
	current := &atomic.Int32{}

	var mockTools []tools.Tool
	for i := 0; i < 8; i++ {
		mockTools = append(mockTools, &mockConcurrencyTool{
			name:    fmt.Sprintf("ct_%d", i),
			maxSeen: maxSeen,
			current: current,
			delay:   20 * time.Millisecond,
		})
	}
	registry := newMockRegistry(mockTools...)

	maxConcurrency := 2
	pe := newParallelExecutor(registry, maxConcurrency)

	var calls []llm.ToolCall
	for i := 0; i < 8; i++ {
		calls = append(calls, llm.ToolCall{
			ID:   fmt.Sprintf("c%d", i),
			Name: fmt.Sprintf("ct_%d", i),
		})
	}

	results := pe.executeParallel(context.Background(), calls,
		func(_ string) (bool, error) { return true, nil },
		nil,
	)

	if len(results) != 8 {
		t.Fatalf("expected 8 results, got %d", len(results))
	}

	for i, r := range results {
		if r.Error {
			t.Errorf("results[%d] unexpected error: %s", i, r.Result)
		}
	}

	observed := int(maxSeen.Load())
	if observed > maxConcurrency {
		t.Errorf("max concurrent executions was %d, but maxConcurrency is %d", observed, maxConcurrency)
	}
}

func TestParallelExecutor_ContextCancellation(t *testing.T) {
	toolA := &mockReadTool{name: "tool_a"}
	registry := newMockRegistry(toolA)

	pe := newParallelExecutor(registry, 2)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	calls := []llm.ToolCall{
		{ID: "c1", Name: "tool_a"},
	}

	results := pe.executeParallel(ctx, calls,
		func(_ string) (bool, error) { return true, nil },
		nil,
	)

	// Should still get results (execution happens, context cancellation propagates to tool)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestParallelExecutor_AllDenied(t *testing.T) {
	toolA := &mockReadTool{name: "tool_a"}
	toolB := &mockReadTool{name: "tool_b"}
	registry := newMockRegistry(toolA, toolB)

	pe := newParallelExecutor(registry, 4)

	calls := []llm.ToolCall{
		{ID: "c1", Name: "tool_a"},
		{ID: "c2", Name: "tool_b"},
	}

	results := pe.executeParallel(context.Background(), calls,
		func(_ string) (bool, error) { return false, nil },
		nil,
	)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for i, r := range results {
		if !r.Error {
			t.Errorf("results[%d] should be an error (denied)", i)
		}
		if r.Result != "Permission denied" {
			t.Errorf("results[%d] expected 'Permission denied', got '%s'", i, r.Result)
		}
	}
}
