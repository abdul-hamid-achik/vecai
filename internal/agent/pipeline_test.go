package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/ui"
)

func newTestPipeline(t *testing.T) (*Pipeline, *llm.MockLLMClient) {
	t.Helper()

	mock := llm.NewMockLLMClient()
	cfg := config.DefaultConfig()
	cfg.Memory.Enabled = false

	registry := tools.NewRegistry(&cfg.Tools)
	policy := permissions.NewPolicy(permissions.ModeAuto, ui.NewInputHandler(), ui.NewOutputHandler())

	p := NewPipeline(PipelineConfig{
		Client:      mock,
		Config:      cfg,
		Registry:    registry,
		Permissions: policy,
	})

	return p, mock
}

func TestNewPipeline(t *testing.T) {
	p, _ := newTestPipeline(t)
	if p == nil {
		t.Fatal("expected non-nil pipeline")
	}
	if p.router == nil {
		t.Error("pipeline router should not be nil")
	}
	if p.planner == nil {
		t.Error("pipeline planner should not be nil")
	}
	if p.executor == nil {
		t.Error("pipeline executor should not be nil")
	}
	if p.verifier == nil {
		t.Error("pipeline verifier should not be nil")
	}
	// Pipeline no longer has an output field; output is passed per-Execute call.
	if p.config == nil {
		t.Error("pipeline config should not be nil")
	}
}

func TestPipeline_GetRouter(t *testing.T) {
	p, _ := newTestPipeline(t)
	if p.GetRouter() == nil {
		t.Error("GetRouter() should not return nil")
	}
}

func TestPipeline_GetPlanner(t *testing.T) {
	p, _ := newTestPipeline(t)
	if p.GetPlanner() == nil {
		t.Error("GetPlanner() should not return nil")
	}
}

func TestPipeline_GetExecutor(t *testing.T) {
	p, _ := newTestPipeline(t)
	if p.GetExecutor() == nil {
		t.Error("GetExecutor() should not return nil")
	}
}

func TestPipeline_GetVerifier(t *testing.T) {
	p, _ := newTestPipeline(t)
	if p.GetVerifier() == nil {
		t.Error("GetVerifier() should not return nil")
	}
}

func TestExtractChangedFiles(t *testing.T) {
	p, _ := newTestPipeline(t)

	tests := []struct {
		name       string
		executions []*ExecutionResult
		wantFiles  map[string]bool
	}{
		{
			name:       "empty executions",
			executions: []*ExecutionResult{},
			wantFiles:  map[string]bool{},
		},
		{
			name: "no file tool calls",
			executions: []*ExecutionResult{
				{
					ToolCalls: []ToolCallResult{
						{Tool: "grep", Input: map[string]any{"pattern": "foo"}},
					},
				},
			},
			wantFiles: map[string]bool{},
		},
		{
			name: "write_file tool call",
			executions: []*ExecutionResult{
				{
					ToolCalls: []ToolCallResult{
						{Tool: "write_file", Input: map[string]any{"path": "main.go"}},
					},
				},
			},
			wantFiles: map[string]bool{"main.go": true},
		},
		{
			name: "edit_file tool call",
			executions: []*ExecutionResult{
				{
					ToolCalls: []ToolCallResult{
						{Tool: "edit_file", Input: map[string]any{"path": "config.go"}},
					},
				},
			},
			wantFiles: map[string]bool{"config.go": true},
		},
		{
			name: "multiple executions with dedup",
			executions: []*ExecutionResult{
				{
					ToolCalls: []ToolCallResult{
						{Tool: "write_file", Input: map[string]any{"path": "main.go"}},
						{Tool: "edit_file", Input: map[string]any{"path": "util.go"}},
					},
				},
				{
					ToolCalls: []ToolCallResult{
						{Tool: "write_file", Input: map[string]any{"path": "main.go"}},
						{Tool: "edit_file", Input: map[string]any{"path": "test.go"}},
					},
				},
			},
			wantFiles: map[string]bool{
				"main.go": true,
				"util.go": true,
				"test.go": true,
			},
		},
		{
			name: "tool call without path key",
			executions: []*ExecutionResult{
				{
					ToolCalls: []ToolCallResult{
						{Tool: "write_file", Input: map[string]any{"content": "data"}},
					},
				},
			},
			wantFiles: map[string]bool{},
		},
		{
			name: "mixed tool calls",
			executions: []*ExecutionResult{
				{
					ToolCalls: []ToolCallResult{
						{Tool: "read_file", Input: map[string]any{"path": "readme.md"}},
						{Tool: "write_file", Input: map[string]any{"path": "output.txt"}},
						{Tool: "list_files", Input: map[string]any{"path": "."}},
					},
				},
			},
			wantFiles: map[string]bool{"output.txt": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.extractChangedFiles(tt.executions)

			// Build a set from the result for easy comparison
			gotSet := make(map[string]bool)
			for _, f := range got {
				gotSet[f] = true
			}

			if len(gotSet) != len(tt.wantFiles) {
				t.Errorf("extractChangedFiles() returned %d files, want %d", len(gotSet), len(tt.wantFiles))
			}
			for f := range tt.wantFiles {
				if !gotSet[f] {
					t.Errorf("extractChangedFiles() missing expected file %q", f)
				}
			}
		})
	}
}

func TestGenerateFinalOutput(t *testing.T) {
	p, _ := newTestPipeline(t)

	t.Run("empty result", func(t *testing.T) {
		result := &PipelineResult{}
		output := p.generateFinalOutput(result)
		if output != "" {
			t.Errorf("expected empty output for empty result, got %q", output)
		}
	})

	t.Run("with plan progress", func(t *testing.T) {
		result := &PipelineResult{
			Plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "step1", Done: true},
					{ID: "step2", Done: false},
					{ID: "step3", Done: true},
				},
			},
		}
		output := p.generateFinalOutput(result)
		if !contains(output, "2/3 steps completed") {
			t.Errorf("expected plan progress in output, got %q", output)
		}
	})

	t.Run("with verification", func(t *testing.T) {
		result := &PipelineResult{
			Verification: &VerificationResult{
				Summary: "All checks passed",
			},
		}
		output := p.generateFinalOutput(result)
		if !contains(output, "All checks passed") {
			t.Errorf("expected verification summary in output, got %q", output)
		}
	})

	t.Run("with errors", func(t *testing.T) {
		result := &PipelineResult{
			Errors: []error{
				fmt.Errorf("step1 failed"),
				fmt.Errorf("step2 failed"),
			},
		}
		output := p.generateFinalOutput(result)
		if !contains(output, "Errors: 2") {
			t.Errorf("expected error count in output, got %q", output)
		}
	})

	t.Run("combined output", func(t *testing.T) {
		result := &PipelineResult{
			Plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "step1", Done: true},
				},
			},
			Verification: &VerificationResult{
				Summary: "OK",
			},
			Errors: []error{fmt.Errorf("oops")},
		}
		output := p.generateFinalOutput(result)
		if !contains(output, "1/1 steps completed") {
			t.Errorf("expected plan info in combined output, got %q", output)
		}
		if !contains(output, "Verification: OK") {
			t.Errorf("expected verification in combined output, got %q", output)
		}
		if !contains(output, "Errors: 1") {
			t.Errorf("expected errors in combined output, got %q", output)
		}
	})
}

func TestPipeline_ExecuteContextCancellation(t *testing.T) {
	p, mock := newTestPipeline(t)

	// Set up the mock to return a response that would normally trigger single-agent flow
	mock.ChatFunc = func(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition, systemPrompt string) (*llm.Response, error) {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return &llm.Response{Content: "mock response"}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	testOutput := &CLIOutput{Out: ui.NewOutputHandler(), In: ui.NewInputHandler()}
	result, err := p.Execute(ctx, "explain how routing works", testOutput)
	// The pipeline should handle the cancellation gracefully.
	// Depending on where cancellation hits, we may get an error in result or returned.
	// We just verify it does not panic.
	if err != nil && result == nil {
		// This is acceptable - the error was returned directly
		return
	}
	if result != nil && len(result.Errors) > 0 {
		// Also acceptable - errors captured in result
		return
	}
	// If we get here with a nil error and no result errors, the task completed before
	// cancellation took effect, which is also acceptable for a fast mock.
}

func TestPipelineResult_InitialState(t *testing.T) {
	result := &PipelineResult{}
	if result.Success {
		t.Error("new PipelineResult should not be successful by default")
	}
	if result.Plan != nil {
		t.Error("new PipelineResult should have nil plan")
	}
	if result.Verification != nil {
		t.Error("new PipelineResult should have nil verification")
	}
	if len(result.Executions) != 0 {
		t.Error("new PipelineResult should have no executions")
	}
	if len(result.Errors) != 0 {
		t.Error("new PipelineResult should have no errors")
	}
}
