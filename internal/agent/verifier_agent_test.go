package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

func newTestVerifierAgent(t *testing.T) (*VerifierAgent, *llm.MockLLMClient) {
	t.Helper()
	mock := llm.NewMockLLMClient()
	cfg := config.DefaultConfig()
	registry := tools.NewRegistry(&cfg.Tools)
	return NewVerifierAgent(mock, cfg, registry), mock
}

// --- FormatResult ---

func TestVerifierAgent_FormatResult(t *testing.T) {
	verifier, _ := newTestVerifierAgent(t)

	tests := []struct {
		name     string
		result   *VerificationResult
		contains []string
	}{
		{
			name: "passed with no issues",
			result: &VerificationResult{
				Passed:  true,
				Summary: "[PASSED] Lint: PASS | Tests: PASS",
			},
			contains: []string{
				"## Verification Result",
				"[PASSED] Lint: PASS | Tests: PASS",
				"No issues found.",
			},
		},
		{
			name: "failed with error issues",
			result: &VerificationResult{
				Passed:  false,
				Summary: "[FAILED] Lint: FAIL | Tests: FAIL | Errors: 2",
				Issues: []VerificationIssue{
					{
						Severity:    "error",
						Description: "unused variable",
						File:        "main.go",
						Line:        42,
						Suggestion:  "Remove the variable",
					},
					{
						Severity:    "error",
						Description: "test failure in TestFoo",
						Suggestion:  "Fix assertion",
					},
				},
			},
			contains: []string{
				"## Verification Result",
				"[FAILED]",
				"[error]",
				"unused variable",
				"File: main.go:42",
				"Suggestion: Remove the variable",
				"test failure in TestFoo",
			},
		},
		{
			name: "warning issues",
			result: &VerificationResult{
				Passed:  true,
				Summary: "[PASSED] Lint: PASS | Tests: PASS | Warnings: 1",
				Issues: []VerificationIssue{
					{
						Severity:    "warning",
						Description: "long function",
						File:        "handler.go",
					},
				},
			},
			contains: []string{
				"[warning]",
				"long function",
				"File: handler.go",
			},
		},
		{
			name: "info issues",
			result: &VerificationResult{
				Passed:  true,
				Summary: "[PASSED] Lint: PASS | Tests: PASS",
				Issues: []VerificationIssue{
					{
						Severity:    "info",
						Description: "consider adding docs",
					},
				},
			},
			contains: []string{
				"[info]",
				"consider adding docs",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := verifier.FormatResult(tt.result)
			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("FormatResult() missing %q\ngot:\n%s", want, output)
				}
			}
		})
	}
}

// --- generateSummary ---

func TestVerifierAgent_generateSummary(t *testing.T) {
	verifier, _ := newTestVerifierAgent(t)

	tests := []struct {
		name     string
		result   *VerificationResult
		contains []string
	}{
		{
			name: "all passed",
			result: &VerificationResult{
				Passed:      true,
				LintPassed:  true,
				TestsPassed: true,
			},
			contains: []string{"[PASSED]", "Lint: PASS", "Tests: PASS"},
		},
		{
			name: "lint failed",
			result: &VerificationResult{
				Passed:      false,
				LintPassed:  false,
				TestsPassed: true,
			},
			contains: []string{"[FAILED]", "Lint: FAIL", "Tests: PASS"},
		},
		{
			name: "tests failed",
			result: &VerificationResult{
				Passed:      false,
				LintPassed:  true,
				TestsPassed: false,
			},
			contains: []string{"[FAILED]", "Lint: PASS", "Tests: FAIL"},
		},
		{
			name: "with error and warning counts",
			result: &VerificationResult{
				Passed:      false,
				LintPassed:  false,
				TestsPassed: false,
				Issues: []VerificationIssue{
					{Severity: "error"},
					{Severity: "error"},
					{Severity: "warning"},
				},
			},
			contains: []string{"Errors: 2", "Warnings: 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := verifier.generateSummary(tt.result)
			for _, want := range tt.contains {
				if !strings.Contains(summary, want) {
					t.Errorf("generateSummary() missing %q, got %q", want, summary)
				}
			}
		})
	}
}

// --- QuickVerify ---

func newTestVerifierAgentNoTools(t *testing.T) (*VerifierAgent, *llm.MockLLMClient) {
	t.Helper()
	mock := llm.NewMockLLMClient()
	cfg := config.DefaultConfig()
	// Use an empty registry so lint/test tools don't run real commands
	registry := &tools.Registry{}
	return NewVerifierAgent(mock, cfg, registry), mock
}

func TestVerifierAgent_QuickVerify(t *testing.T) {
	tests := []struct {
		name         string
		changedFiles []string
		wantPassed   bool
	}{
		{
			name:         "no tools registered passes by default",
			changedFiles: []string{"main.go"},
			wantPassed:   true,
		},
		{
			name:         "empty file list",
			changedFiles: []string{},
			wantPassed:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier, _ := newTestVerifierAgentNoTools(t)
			result, err := verifier.QuickVerify(context.Background(), tt.changedFiles)
			if err != nil {
				t.Fatalf("QuickVerify() error = %v", err)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("result.Passed = %v, want %v", result.Passed, tt.wantPassed)
			}
			if result.Summary == "" {
				t.Error("result.Summary should not be empty")
			}
		})
	}
}

// --- VerifyChanges ---

func TestVerifierAgent_VerifyChanges(t *testing.T) {
	tests := []struct {
		name         string
		execution    *ExecutionResult
		changedFiles []string
		llmResponse  string
		wantPassed   bool
	}{
		{
			name: "LGTM response passes",
			execution: &ExecutionResult{
				StepID:  "s1",
				Success: true,
				Output:  "Added error handling.",
			},
			changedFiles: []string{"handler.go"},
			llmResponse:  "LGTM - Changes look good",
			wantPassed:   true,
		},
		{
			name: "error issue fails",
			execution: &ExecutionResult{
				StepID:  "s1",
				Success: true,
				Output:  "Made changes.",
			},
			changedFiles: []string{"main.go"},
			llmResponse:  "ISSUE: [error] - Missing error handling\nSUGGESTION: Add error check",
			wantPassed:   false,
		},
		{
			name: "warning issue still passes",
			execution: &ExecutionResult{
				StepID:  "s1",
				Success: true,
				Output:  "Refactored code.",
			},
			changedFiles: []string{"util.go"},
			llmResponse:  "ISSUE: [warning] - Long function\nSUGGESTION: Consider splitting",
			wantPassed:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use no-tools verifier so lint/test don't run real commands
			verifier, mock := newTestVerifierAgentNoTools(t)
			mock.ChatFunc = func(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition, _ string) (*llm.Response, error) {
				return &llm.Response{Content: tt.llmResponse}, nil
			}

			result, err := verifier.VerifyChanges(context.Background(), tt.execution, tt.changedFiles)
			if err != nil {
				t.Fatalf("VerifyChanges() error = %v", err)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("result.Passed = %v, want %v", result.Passed, tt.wantPassed)
			}
			if result.Summary == "" {
				t.Error("result.Summary should not be empty")
			}
		})
	}
}

func TestVerifierAgent_VerifyChanges_WithToolCalls(t *testing.T) {
	// Use no-tools verifier so lint/test don't run real commands
	verifier, mock := newTestVerifierAgentNoTools(t)
	mock.ChatFunc = func(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition, _ string) (*llm.Response, error) {
		return &llm.Response{Content: "LGTM - Changes look good"}, nil
	}

	execution := &ExecutionResult{
		StepID:  "s1",
		Success: true,
		Output:  "Updated files.",
		ToolCalls: []ToolCallResult{
			{Tool: "write_file", Input: map[string]any{"path": "main.go"}, Output: "ok"},
			{Tool: "read_file", Input: map[string]any{"path": "util.go"}, Output: "contents..."},
		},
	}

	result, err := verifier.VerifyChanges(context.Background(), execution, []string{"main.go"})
	if err != nil {
		t.Fatalf("VerifyChanges() error = %v", err)
	}
	if !result.Passed {
		t.Error("expected LGTM to pass verification")
	}
}

// --- reviewCode parsing ---

func TestVerifierAgent_reviewCode(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		wantIssues int
	}{
		{
			name:       "LGTM",
			response:   "LGTM - Changes look good",
			wantIssues: 0,
		},
		{
			name:       "single error",
			response:   "ISSUE: [error] - Bad code\nSUGGESTION: Fix it",
			wantIssues: 1,
		},
		{
			name:       "multiple issues",
			response:   "ISSUE: [error] - Problem one\nSUGGESTION: Fix one\nISSUE: [warning] - Problem two\nSUGGESTION: Fix two",
			wantIssues: 2,
		},
		{
			name:       "info issue without severity bracket",
			response:   "ISSUE: Missing docs\nSUGGESTION: Add docs",
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier, mock := newTestVerifierAgentNoTools(t)
			mock.ChatFunc = func(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition, _ string) (*llm.Response, error) {
				return &llm.Response{Content: tt.response}, nil
			}

			execution := &ExecutionResult{Output: "test output"}
			result := verifier.reviewCode(context.Background(), execution, []string{"test.go"})
			if len(result.Issues) != tt.wantIssues {
				t.Errorf("reviewCode() returned %d issues, want %d", len(result.Issues), tt.wantIssues)
			}
		})
	}
}
