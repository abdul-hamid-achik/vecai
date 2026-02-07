package agent

import (
	"context"
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/ui"
)

func newTestExecutorAgent(t *testing.T) (*ExecutorAgent, *llm.MockLLMClient) {
	t.Helper()
	mock := llm.NewMockLLMClient()
	cfg := config.DefaultConfig()
	registry := tools.NewRegistry(&cfg.Tools)
	policy := permissions.NewPolicy(permissions.ModeAuto, ui.NewInputHandler(), ui.NewOutputHandler())
	return NewExecutorAgent(mock, cfg, registry, policy), mock
}

// --- Construction ---

func TestExecutorAgent_New(t *testing.T) {
	executor, _ := newTestExecutorAgent(t)
	if executor == nil {
		t.Fatal("NewExecutorAgent() returned nil")
	}
	if executor.client == nil {
		t.Error("executor client should not be nil")
	}
	if executor.config == nil {
		t.Error("executor config should not be nil")
	}
	if executor.registry == nil {
		t.Error("executor registry should not be nil")
	}
	if executor.permissions == nil {
		t.Error("executor permissions should not be nil")
	}
}

// --- ExecuteDirectTask ---

func TestExecutorAgent_ExecuteDirectTask(t *testing.T) {
	tests := []struct {
		name       string
		task       string
		intent     Intent
		response   *llm.Response
		wantOutput string
		wantSucc   bool
	}{
		{
			name:   "simple content response with code intent",
			task:   "write a hello world function",
			intent: IntentCode,
			response: &llm.Response{
				Content:    "Here is the function:\nfunc hello() {}",
				StopReason: "end_turn",
			},
			wantOutput: "Here is the function:\nfunc hello() {}",
			wantSucc:   true,
		},
		{
			name:   "question intent",
			task:   "what does this function do?",
			intent: IntentQuestion,
			response: &llm.Response{
				Content:    "It processes input data.",
				StopReason: "end_turn",
			},
			wantOutput: "It processes input data.",
			wantSucc:   true,
		},
		{
			name:   "debug intent",
			task:   "why is this test failing?",
			intent: IntentDebug,
			response: &llm.Response{
				Content:    "The test expects X but gets Y.",
				StopReason: "end_turn",
			},
			wantOutput: "The test expects X but gets Y.",
			wantSucc:   true,
		},
		{
			name:   "simple intent uses smart tier",
			task:   "do a thing",
			intent: IntentSimple,
			response: &llm.Response{
				Content:    "Done.",
				StopReason: "end_turn",
			},
			wantOutput: "Done.",
			wantSucc:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, mock := newTestExecutorAgent(t)
			mock.ChatFunc = func(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition, _ string) (*llm.Response, error) {
				return tt.response, nil
			}

			result, err := executor.ExecuteDirectTask(context.Background(), tt.task, tt.intent)
			if err != nil {
				t.Fatalf("ExecuteDirectTask() error = %v", err)
			}
			if result.Success != tt.wantSucc {
				t.Errorf("result.Success = %v, want %v", result.Success, tt.wantSucc)
			}
			if result.Output != tt.wantOutput {
				t.Errorf("result.Output = %q, want %q", result.Output, tt.wantOutput)
			}
			if result.StepID != "direct" {
				t.Errorf("result.StepID = %q, want %q", result.StepID, "direct")
			}
		})
	}
}

func TestExecutorAgent_ExecuteDirectTask_LLMError(t *testing.T) {
	executor, mock := newTestExecutorAgent(t)
	mock.ChatFunc = func(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition, _ string) (*llm.Response, error) {
		return nil, context.DeadlineExceeded
	}

	result, err := executor.ExecuteDirectTask(context.Background(), "test", IntentCode)
	if err != nil {
		t.Fatalf("ExecuteDirectTask() should not return an outer error, got %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected result.Error to be set on LLM failure")
	}
	if result.Success {
		t.Error("result.Success should be false on LLM failure")
	}
}

// --- ExecuteStep ---

func TestExecutorAgent_ExecuteStep(t *testing.T) {
	tests := []struct {
		name       string
		step       *PlanStep
		prevCtx    string
		response   *llm.Response
		wantOutput string
		wantSucc   bool
	}{
		{
			name: "read step with simple response",
			step: &PlanStep{
				ID:          "s1",
				Description: "Read the main file",
				Type:        "read",
			},
			prevCtx: "",
			response: &llm.Response{
				Content:    "The file contains a main function.",
				StopReason: "end_turn",
			},
			wantOutput: "The file contains a main function.",
			wantSucc:   true,
		},
		{
			name: "code step with previous context",
			step: &PlanStep{
				ID:          "s2",
				Description: "Add error handling",
				Type:        "code",
				Files:       []string{"handler.go"},
			},
			prevCtx: "The handler currently panics on nil input.",
			response: &llm.Response{
				Content:    "Added nil check to handler.",
				StopReason: "end_turn",
			},
			wantOutput: "Added nil check to handler.",
			wantSucc:   true,
		},
		{
			name: "test step",
			step: &PlanStep{
				ID:          "s3",
				Description: "Run tests",
				Type:        "test",
			},
			response: &llm.Response{
				Content:    "All tests passed.",
				StopReason: "end_turn",
			},
			wantOutput: "All tests passed.",
			wantSucc:   true,
		},
		{
			name: "verify step",
			step: &PlanStep{
				ID:          "s4",
				Description: "Verify results",
				Type:        "verify",
			},
			response: &llm.Response{
				Content:    "Everything looks good.",
				StopReason: "end_turn",
			},
			wantOutput: "Everything looks good.",
			wantSucc:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, mock := newTestExecutorAgent(t)
			mock.ChatFunc = func(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition, _ string) (*llm.Response, error) {
				return tt.response, nil
			}

			result, err := executor.ExecuteStep(context.Background(), tt.step, tt.prevCtx)
			if err != nil {
				t.Fatalf("ExecuteStep() error = %v", err)
			}
			if result.Success != tt.wantSucc {
				t.Errorf("result.Success = %v, want %v", result.Success, tt.wantSucc)
			}
			if result.Output != tt.wantOutput {
				t.Errorf("result.Output = %q, want %q", result.Output, tt.wantOutput)
			}
			if result.StepID != tt.step.ID {
				t.Errorf("result.StepID = %q, want %q", result.StepID, tt.step.ID)
			}
		})
	}
}

func TestExecutorAgent_ExecuteStep_LLMError(t *testing.T) {
	executor, mock := newTestExecutorAgent(t)
	mock.ChatFunc = func(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition, _ string) (*llm.Response, error) {
		return nil, context.DeadlineExceeded
	}

	step := &PlanStep{ID: "err", Description: "will fail", Type: "code"}
	result, err := executor.ExecuteStep(context.Background(), step, "")
	if err != nil {
		t.Fatalf("ExecuteStep() should not return outer error, got %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected result.Error to be set")
	}
	if result.Success {
		t.Error("result.Success should be false on LLM failure")
	}
}

// --- buildSystemPrompt ---

func TestExecutorAgent_buildSystemPrompt(t *testing.T) {
	executor, _ := newTestExecutorAgent(t)

	tests := []struct {
		stepType string
		contains string
	}{
		{"read", "read"},
		{"code", "code"},
		{"test", "test"},
		{"verify", "verify"},
	}

	for _, tt := range tests {
		t.Run(tt.stepType, func(t *testing.T) {
			prompt := executor.buildSystemPrompt(tt.stepType)
			if prompt == "" {
				t.Error("buildSystemPrompt() returned empty string")
			}
			// The prompt includes the step type
			if !containsStr(prompt, tt.contains) {
				t.Errorf("buildSystemPrompt(%q) should contain %q", tt.stepType, tt.contains)
			}
		})
	}
}

// --- buildSystemPromptForIntent ---

func TestExecutorAgent_buildSystemPromptForIntent(t *testing.T) {
	executor, _ := newTestExecutorAgent(t)

	tests := []struct {
		intent   Intent
		contains string
	}{
		{IntentCode, "code"},
		{IntentQuestion, "questions"},
		{IntentDebug, "debugging"},
		{IntentSimple, "helpful"},
	}

	for _, tt := range tests {
		t.Run(string(tt.intent), func(t *testing.T) {
			prompt := executor.buildSystemPromptForIntent(tt.intent)
			if prompt == "" {
				t.Error("buildSystemPromptForIntent() returned empty string")
			}
			if !containsStr(prompt, tt.contains) {
				t.Errorf("prompt for intent %q should contain %q, got: %s", tt.intent, tt.contains, prompt)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(len(s) >= len(substr)) &&
		(s == substr || len(s) > len(substr) && searchIgnoreCase(s, substr))
}

func searchIgnoreCase(s, substr string) bool {
	sl := len(s)
	subl := len(substr)
	for i := 0; i <= sl-subl; i++ {
		match := true
		for j := 0; j < subl; j++ {
			sc := s[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
