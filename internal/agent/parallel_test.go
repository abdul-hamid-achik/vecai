package agent

import (
	"context"
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

func TestParallelExecutor_AllApproved(t *testing.T) {
	cfg := &config.ToolsConfig{}
	registry := tools.NewRegistry(cfg)
	pe := newParallelExecutor(registry, 2)

	// Use list_files which should be registered
	toolCalls := []llm.ToolCall{
		{Name: "list_files", Input: map[string]any{"path": "."}},
	}

	results := pe.executeParallel(
		context.Background(),
		toolCalls,
		func(name string) (bool, error) { return true, nil },
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error {
		t.Errorf("expected success, got error: %s", results[0].Result)
	}
}

func TestParallelExecutor_PermissionDenied(t *testing.T) {
	cfg := &config.ToolsConfig{}
	registry := tools.NewRegistry(cfg)
	pe := newParallelExecutor(registry, 2)

	toolCalls := []llm.ToolCall{
		{Name: "list_files", Input: map[string]any{"path": "."}},
	}

	results := pe.executeParallel(
		context.Background(),
		toolCalls,
		func(name string) (bool, error) { return false, nil },
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Error {
		t.Error("expected error for denied permission")
	}
	if results[0].Result != "Permission denied" {
		t.Errorf("expected 'Permission denied', got %q", results[0].Result)
	}
}

func TestParallelExecutor_DefaultConcurrency(t *testing.T) {
	pe := newParallelExecutor(nil, 0)
	if pe.maxConcurrency != defaultMaxConcurrency {
		t.Errorf("expected default concurrency %d, got %d", defaultMaxConcurrency, pe.maxConcurrency)
	}
}

func TestParallelExecutor_ContextCancellation(t *testing.T) {
	cfg := &config.ToolsConfig{}
	registry := tools.NewRegistry(cfg)
	pe := newParallelExecutor(registry, 2)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	toolCalls := []llm.ToolCall{
		{Name: "list_files", Input: map[string]any{"path": "."}},
	}

	results := pe.executeParallel(
		ctx,
		toolCalls,
		func(name string) (bool, error) { return true, nil },
		nil,
	)

	// Should still get results (execution happens, context cancellation propagates to tool)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
