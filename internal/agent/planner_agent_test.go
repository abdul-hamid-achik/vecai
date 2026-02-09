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

// --- stripMarkdownFences ---

func TestStripMarkdownFences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "json code fence",
			input: "```json\n{\"goal\": \"test\"}\n```",
			want:  "{\"goal\": \"test\"}",
		},
		{
			name:  "plain code fence",
			input: "```\n{\"goal\": \"test\"}\n```",
			want:  "{\"goal\": \"test\"}",
		},
		{
			name:  "no fences",
			input: "{\"goal\": \"test\"}",
			want:  "{\"goal\": \"test\"}",
		},
		{
			name:  "too short for fences",
			input: "ab",
			want:  "ab",
		},
		{
			name:  "multi-line content in fences",
			input: "```json\n{\n  \"goal\": \"test\",\n  \"steps\": []\n}\n```",
			want:  "{\n  \"goal\": \"test\",\n  \"steps\": []\n}",
		},
		{
			name:  "no closing fence",
			input: "```json\n{\"goal\": \"test\"}",
			want:  "```json\n{\"goal\": \"test\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdownFences(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdownFences() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- parsePlan with markdown fences ---

func TestPlannerAgent_parsePlanWithMarkdownFences(t *testing.T) {
	planner, _ := newTestPlannerAgent(t)

	// JSON wrapped in markdown fences should parse correctly
	input := "```json\n{\"goal\":\"test\",\"summary\":\"s\",\"steps\":[{\"id\":\"s1\",\"description\":\"do it\",\"type\":\"code\"}]}\n```"
	got, err := planner.parsePlan(input)
	if err != nil {
		t.Fatalf("parsePlan() with markdown fences should succeed, got error: %v", err)
	}
	if len(got.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(got.Steps))
	}
	if got.Goal != "test" {
		t.Errorf("Expected goal 'test', got %q", got.Goal)
	}
}

// --- buildFallbackPlan ---

func TestPlannerAgent_buildFallbackPlan(t *testing.T) {
	planner, _ := newTestPlannerAgent(t)

	t.Run("numbered steps extraction", func(t *testing.T) {
		content := "Here is my plan:\n1. Read the codebase\n2. Refactor the logger\n3. Update tests"
		got := planner.buildFallbackPlan("Refactor", content)
		if len(got.Steps) != 3 {
			t.Fatalf("Expected 3 steps from numbered content, got %d", len(got.Steps))
		}
		if got.Steps[0].Description != "Read the codebase" {
			t.Errorf("Step 1 description = %q, want 'Read the codebase'", got.Steps[0].Description)
		}
		if got.Steps[2].Description != "Update tests" {
			t.Errorf("Step 3 description = %q, want 'Update tests'", got.Steps[2].Description)
		}
	})

	t.Run("JSON with summary and steps", func(t *testing.T) {
		content := `{"summary": "Fix the bug", "steps": [{"description": "Find root cause"}, {"description": "Apply patch"}]}`
		got := planner.buildFallbackPlan("Fix bug", content)
		if got.Summary != "Fix the bug" {
			t.Errorf("Summary = %q, want 'Fix the bug'", got.Summary)
		}
		if len(got.Steps) != 2 {
			t.Fatalf("Expected 2 steps, got %d", len(got.Steps))
		}
	})

	t.Run("JSON with string steps", func(t *testing.T) {
		content := `{"summary": "Migrate DB", "steps": ["Create migration", "Run migration", "Verify data"]}`
		got := planner.buildFallbackPlan("DB migration", content)
		if len(got.Steps) != 3 {
			t.Fatalf("Expected 3 steps from string array, got %d", len(got.Steps))
		}
		if got.Steps[0].Description != "Create migration" {
			t.Errorf("Step 1 = %q, want 'Create migration'", got.Steps[0].Description)
		}
	})

	t.Run("plain text fallback", func(t *testing.T) {
		content := "Just do the thing, no structure here."
		got := planner.buildFallbackPlan("Do thing", content)
		if len(got.Steps) != 1 {
			t.Fatalf("Expected 1 fallback step, got %d", len(got.Steps))
		}
		if got.Summary != "Generated from unstructured response" {
			t.Errorf("Summary = %q, want 'Generated from unstructured response'", got.Summary)
		}
	})

	t.Run("JSON in markdown fences with fallback", func(t *testing.T) {
		content := "```json\n{\"summary\": \"Fix it\", \"steps\": [{\"description\": \"Step 1\"}]}\n```"
		got := planner.buildFallbackPlan("Fix", content)
		if got.Summary != "Fix it" {
			t.Errorf("Summary = %q, want 'Fix it'", got.Summary)
		}
		if len(got.Steps) != 1 {
			t.Fatalf("Expected 1 step, got %d", len(got.Steps))
		}
	})
}

// --- stripThinkTags ---

func TestStripThinkTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no think tags",
			input: `{"goal":"test","steps":[]}`,
			want:  `{"goal":"test","steps":[]}`,
		},
		{
			name:  "think tags before JSON",
			input: "<think>\nLet me analyze this...\n</think>\n{\"goal\":\"test\",\"steps\":[]}",
			want:  `{"goal":"test","steps":[]}`,
		},
		{
			name:  "think tags wrapping everything",
			input: "<think>reasoning here</think>Here is the plan",
			want:  "Here is the plan",
		},
		{
			name:  "multiple think blocks",
			input: "<think>first</think>middle<think>second</think>end",
			want:  "middleend",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripThinkTags(tt.input)
			if got != tt.want {
				t.Errorf("stripThinkTags() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- extractJSONSubstring ---

func TestExtractJSONSubstring(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "pure JSON",
			input: `{"goal":"test"}`,
			want:  `{"goal":"test"}`,
		},
		{
			name:  "JSON with trailing text",
			input: `{"goal":"test","steps":[]} Let me know if you need changes!`,
			want:  `{"goal":"test","steps":[]}`,
		},
		{
			name:  "JSON with leading text",
			input: `Here is the plan: {"goal":"test","steps":[]}`,
			want:  `{"goal":"test","steps":[]}`,
		},
		{
			name:  "JSON with both leading and trailing text",
			input: `Sure! {"goal":"test","steps":[]} Hope that helps.`,
			want:  `{"goal":"test","steps":[]}`,
		},
		{
			name:  "no JSON",
			input: "no json here at all",
			want:  "",
		},
		{
			name:  "only opening brace",
			input: "{ but no closing",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONSubstring(tt.input)
			if got != tt.want {
				t.Errorf("extractJSONSubstring() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- buildFallbackPlan with trailing text (the screenshot bug) ---

func TestPlannerAgent_buildFallbackPlan_JSONWithTrailingText(t *testing.T) {
	planner, _ := newTestPlannerAgent(t)

	t.Run("JSON with trailing text should extract steps", func(t *testing.T) {
		content := `{"summary": "Fix the bug", "steps": [{"description": "Find root cause"}, {"description": "Apply patch"}]} Let me know if you want me to refine this plan!`
		got := planner.buildFallbackPlan("Fix bug", content)
		if got.Summary != "Fix the bug" {
			t.Errorf("Summary = %q, want 'Fix the bug'", got.Summary)
		}
		if len(got.Steps) != 2 {
			t.Fatalf("Expected 2 steps, got %d", len(got.Steps))
		}
	})

	t.Run("think tags followed by JSON", func(t *testing.T) {
		content := "<think>\nLet me think about this...\n</think>\n{\"summary\": \"Refactor\", \"steps\": [{\"description\": \"Extract method\"}]}"
		got := planner.buildFallbackPlan("Refactor", content)
		if got.Summary != "Refactor" {
			t.Errorf("Summary = %q, want 'Refactor'", got.Summary)
		}
		if len(got.Steps) != 1 {
			t.Fatalf("Expected 1 step, got %d", len(got.Steps))
		}
	})

	t.Run("think tags + JSON + trailing text", func(t *testing.T) {
		content := "<think>reasoning</think>\n{\"summary\": \"Plan\", \"steps\": [{\"description\": \"Do it\"}]}\nHope this helps!"
		got := planner.buildFallbackPlan("Test", content)
		if len(got.Steps) != 1 {
			t.Fatalf("Expected 1 step, got %d", len(got.Steps))
		}
		if got.Steps[0].Description != "Do it" {
			t.Errorf("Step description = %q, want 'Do it'", got.Steps[0].Description)
		}
	})
}

// --- parsePlan with think tags ---

func TestPlannerAgent_parsePlan_WithThinkTags(t *testing.T) {
	planner, _ := newTestPlannerAgent(t)

	input := "<think>\nAnalyzing the task...\n</think>\n{\"goal\":\"test\",\"summary\":\"s\",\"steps\":[{\"id\":\"s1\",\"description\":\"do it\",\"type\":\"code\"}]}"
	got, err := planner.parsePlan(input)
	if err != nil {
		t.Fatalf("parsePlan() with think tags should succeed, got error: %v", err)
	}
	if len(got.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(got.Steps))
	}
	if got.Goal != "test" {
		t.Errorf("Expected goal 'test', got %q", got.Goal)
	}
}

// --- truncateDescription ---

func TestTruncateDescription(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 200, "short"},
		{strings.Repeat("a", 200), 200, strings.Repeat("a", 200)},
		{strings.Repeat("a", 201), 200, strings.Repeat("a", 197) + "..."},
	}
	for _, tt := range tests {
		got := truncateDescription(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateDescription(%d chars, %d) = %d chars, want %d chars",
				len(tt.input), tt.maxLen, len(got), len(tt.want))
		}
	}
}

// --- FormatPlan with long step descriptions ---

func TestPlannerAgent_FormatPlan_LongDescriptionTruncation(t *testing.T) {
	planner, _ := newTestPlannerAgent(t)

	longDesc := strings.Repeat("x", 300)
	plan := &StructuredPlan{
		Goal:    "Test truncation",
		Summary: "Check that long descriptions are capped",
		Steps: []PlanStep{
			{ID: "s1", Description: longDesc, Type: "code"},
		},
	}

	output := planner.FormatPlan(plan)
	// The description in the formatted output should be truncated
	if strings.Contains(output, longDesc) {
		t.Error("FormatPlan should truncate long step descriptions")
	}
	if !strings.Contains(output, "...") {
		t.Error("Truncated description should end with '...'")
	}
}

// --- looksLikeJSON ---

func TestLooksLikeJSON(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`{"key": "value"}`, true},
		{`  {"key": "value"}  `, true},
		{`not json`, false},
		{`[1,2,3]`, false},
		{`{broken`, false},
		{``, false},
	}
	for _, tt := range tests {
		got := looksLikeJSON(tt.input)
		if got != tt.want {
			t.Errorf("looksLikeJSON(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --- cleanFallbackDescription ---

func TestCleanFallbackDescription(t *testing.T) {
	t.Run("extracts from JSON with summary", func(t *testing.T) {
		input := `{"summary": "Fix the bug", "extra": "data"}`
		got := cleanFallbackDescription(input)
		if got != "Fix the bug" {
			t.Errorf("cleanFallbackDescription() = %q, want 'Fix the bug'", got)
		}
	})

	t.Run("extracts from JSON with goal", func(t *testing.T) {
		input := `{"goal": "Refactor auth"}`
		got := cleanFallbackDescription(input)
		if got != "Refactor auth" {
			t.Errorf("cleanFallbackDescription() = %q, want 'Refactor auth'", got)
		}
	})

	t.Run("plain text truncated", func(t *testing.T) {
		input := strings.Repeat("x", 300)
		got := cleanFallbackDescription(input)
		if len([]rune(got)) > 200 {
			t.Errorf("cleanFallbackDescription should truncate to 200 runes, got %d", len([]rune(got)))
		}
	})

	t.Run("non-JSON returned as-is", func(t *testing.T) {
		input := "Just a plain description"
		got := cleanFallbackDescription(input)
		if got != input {
			t.Errorf("cleanFallbackDescription() = %q, want %q", got, input)
		}
	})
}
