package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

func newTestPlannerAgent(t *testing.T) (*PlannerAgent, *llm.MockLLMClient) {
	t.Helper()
	mock := llm.NewMockLLMClient()
	cfg := config.DefaultConfig()
	registry := tools.NewRegistry(&cfg.Tools)
	return NewPlannerAgent(mock, cfg, registry), mock
}

// --- FormatPlan ---

func TestPlannerAgent_FormatPlan(t *testing.T) {
	planner, _ := newTestPlannerAgent(t)

	tests := []struct {
		name     string
		plan     *StructuredPlan
		contains []string
	}{
		{
			name: "plan with steps",
			plan: &StructuredPlan{
				Goal:    "Refactor logging",
				Summary: "Move to structured logging",
				Steps: []PlanStep{
					{ID: "s1", Description: "Read current logger", Type: "read", Done: false},
					{ID: "s2", Description: "Replace fmt calls", Type: "code", Done: true, Files: []string{"main.go"}},
				},
				Risks:       []string{"Breaking existing callers"},
				Assumptions: []string{"Tests exist for logger"},
			},
			contains: []string{
				"## Plan: Refactor logging",
				"**Summary:** Move to structured logging",
				"[ ] **Read current logger** (read)",
				"[x] **Replace fmt calls** (code)",
				"Files: main.go",
				"### Risks",
				"Breaking existing callers",
				"### Assumptions",
				"Tests exist for logger",
			},
		},
		{
			name: "empty plan",
			plan: &StructuredPlan{
				Goal:    "Empty",
				Summary: "Nothing to do",
				Steps:   []PlanStep{},
			},
			contains: []string{
				"## Plan: Empty",
				"**Summary:** Nothing to do",
				"### Steps",
			},
		},
		{
			name: "step with dependencies",
			plan: &StructuredPlan{
				Goal:    "Dep test",
				Summary: "Test deps",
				Steps: []PlanStep{
					{ID: "a", Description: "First", Type: "read"},
					{ID: "b", Description: "Second", Type: "code", Dependencies: []string{"a"}},
				},
			},
			contains: []string{
				"Depends on: a",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := planner.FormatPlan(tt.plan)
			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("FormatPlan() output missing %q\ngot:\n%s", want, output)
				}
			}
		})
	}
}

// --- IsPlanComplete ---

func TestPlannerAgent_IsPlanComplete(t *testing.T) {
	planner, _ := newTestPlannerAgent(t)

	tests := []struct {
		name string
		plan *StructuredPlan
		want bool
	}{
		{
			name: "all steps done",
			plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "1", Done: true},
					{ID: "2", Done: true},
				},
			},
			want: true,
		},
		{
			name: "no steps done",
			plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "1", Done: false},
					{ID: "2", Done: false},
				},
			},
			want: false,
		},
		{
			name: "partial completion",
			plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "1", Done: true},
					{ID: "2", Done: false},
				},
			},
			want: false,
		},
		{
			name: "empty plan is complete",
			plan: &StructuredPlan{Steps: []PlanStep{}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := planner.IsPlanComplete(tt.plan)
			if got != tt.want {
				t.Errorf("IsPlanComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- GetNextStep ---

func TestPlannerAgent_GetNextStep(t *testing.T) {
	planner, _ := newTestPlannerAgent(t)

	tests := []struct {
		name   string
		plan   *StructuredPlan
		wantID string // empty string means nil
	}{
		{
			name: "returns first undone step",
			plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "s1", Done: true},
					{ID: "s2", Done: false},
					{ID: "s3", Done: false},
				},
			},
			wantID: "s2",
		},
		{
			name: "returns nil when all done",
			plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "s1", Done: true},
					{ID: "s2", Done: true},
				},
			},
			wantID: "",
		},
		{
			name: "respects dependencies",
			plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "s1", Done: false},
					{ID: "s2", Done: false, Dependencies: []string{"s1"}},
				},
			},
			wantID: "s1",
		},
		{
			name: "skips step with unmet dependencies",
			plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "s1", Done: false, Dependencies: []string{"s3"}},
					{ID: "s2", Done: false},
					{ID: "s3", Done: false},
				},
			},
			wantID: "s2",
		},
		{
			name:   "empty plan returns nil",
			plan:   &StructuredPlan{Steps: []PlanStep{}},
			wantID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := planner.GetNextStep(tt.plan)
			if tt.wantID == "" {
				if got != nil {
					t.Errorf("GetNextStep() = %v, want nil", got.ID)
				}
			} else {
				if got == nil {
					t.Fatalf("GetNextStep() = nil, want step %q", tt.wantID)
				}
				if got.ID != tt.wantID {
					t.Errorf("GetNextStep().ID = %q, want %q", got.ID, tt.wantID)
				}
			}
		})
	}
}

// --- MarkStepDone ---

func TestPlannerAgent_MarkStepDone(t *testing.T) {
	planner, _ := newTestPlannerAgent(t)

	tests := []struct {
		name   string
		plan   *StructuredPlan
		stepID string
		check  func(t *testing.T, plan *StructuredPlan)
	}{
		{
			name: "marks correct step",
			plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "s1", Done: false},
					{ID: "s2", Done: false},
				},
			},
			stepID: "s1",
			check: func(t *testing.T, plan *StructuredPlan) {
				if !plan.Steps[0].Done {
					t.Error("step s1 should be marked done")
				}
				if plan.Steps[1].Done {
					t.Error("step s2 should remain not done")
				}
			},
		},
		{
			name: "no-op for missing ID",
			plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "s1", Done: false},
				},
			},
			stepID: "nonexistent",
			check: func(t *testing.T, plan *StructuredPlan) {
				if plan.Steps[0].Done {
					t.Error("step s1 should remain not done")
				}
			},
		},
		{
			name: "idempotent on already-done step",
			plan: &StructuredPlan{
				Steps: []PlanStep{
					{ID: "s1", Done: true},
				},
			},
			stepID: "s1",
			check: func(t *testing.T, plan *StructuredPlan) {
				if !plan.Steps[0].Done {
					t.Error("step s1 should still be done")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planner.MarkStepDone(tt.plan, tt.stepID)
			tt.check(t, tt.plan)
		})
	}
}

// --- CreatePlan with MockLLMClient ---

func TestPlannerAgent_CreatePlan(t *testing.T) {
	planner, mock := newTestPlannerAgent(t)

	t.Run("parses valid JSON plan", func(t *testing.T) {
		plan := StructuredPlan{
			Goal:    "Add logging",
			Summary: "Integrate structured logging",
			Steps: []PlanStep{
				{ID: "step1", Description: "Read current code", Type: "read"},
				{ID: "step2", Description: "Add logger", Type: "code", Dependencies: []string{"step1"}},
			},
			Risks:       []string{"Breakage"},
			Assumptions: []string{"Go 1.21+"},
		}
		planJSON, _ := json.Marshal(plan)

		mock.ChatFunc = func(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition, _ string) (*llm.Response, error) {
			return &llm.Response{Content: string(planJSON)}, nil
		}

		got, err := planner.CreatePlan(context.Background(), "Add logging", "")
		if err != nil {
			t.Fatalf("CreatePlan() error = %v", err)
		}
		if got.Goal != "Add logging" {
			t.Errorf("plan goal = %q, want %q", got.Goal, "Add logging")
		}
		if len(got.Steps) != 2 {
			t.Errorf("plan steps count = %d, want 2", len(got.Steps))
		}
		if got.Steps[0].Type != "read" {
			t.Errorf("step1 type = %q, want %q", got.Steps[0].Type, "read")
		}
	})

	t.Run("falls back to text plan on invalid JSON", func(t *testing.T) {
		mock.ChatFunc = func(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition, _ string) (*llm.Response, error) {
			return &llm.Response{Content: "Just do the thing, no JSON here."}, nil
		}

		got, err := planner.CreatePlan(context.Background(), "Do something", "")
		if err != nil {
			t.Fatalf("CreatePlan() error = %v", err)
		}
		if len(got.Steps) != 1 {
			t.Fatalf("fallback plan should have 1 step, got %d", len(got.Steps))
		}
		if got.Steps[0].Type != "code" {
			t.Errorf("fallback step type = %q, want %q", got.Steps[0].Type, "code")
		}
	})

	t.Run("returns error on LLM failure", func(t *testing.T) {
		mock.ChatFunc = func(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition, _ string) (*llm.Response, error) {
			return nil, context.DeadlineExceeded
		}

		_, err := planner.CreatePlan(context.Background(), "Fail", "")
		if err == nil {
			t.Fatal("expected error from CreatePlan when LLM fails")
		}
		if !strings.Contains(err.Error(), "planner failed") {
			t.Errorf("error = %q, want it to contain 'planner failed'", err.Error())
		}
	})
}

// --- parsePlan ---

func TestPlannerAgent_parsePlan(t *testing.T) {
	planner, _ := newTestPlannerAgent(t)

	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantSteps int
	}{
		{
			name:      "valid JSON",
			input:     `{"goal":"test","summary":"s","steps":[{"id":"s1","description":"do it","type":"code"}]}`,
			wantSteps: 1,
		},
		{
			name:      "JSON embedded in text",
			input:     `Here is the plan: {"goal":"test","summary":"s","steps":[{"id":"s1","description":"do it","type":"code"}]} end`,
			wantSteps: 1,
		},
		{
			name:    "no JSON",
			input:   "no json here",
			wantErr: true,
		},
		{
			name:    "empty steps",
			input:   `{"goal":"test","summary":"s","steps":[]}`,
			wantErr: true,
		},
		{
			name:      "assigns default IDs and types",
			input:     `{"goal":"test","summary":"s","steps":[{"description":"do it"}]}`,
			wantSteps: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := planner.parsePlan(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePlan() error = %v", err)
			}
			if len(got.Steps) != tt.wantSteps {
				t.Errorf("parsePlan() steps = %d, want %d", len(got.Steps), tt.wantSteps)
			}
		})
	}
}
