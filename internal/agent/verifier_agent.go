package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// VerificationResult represents the result of verifying changes
type VerificationResult struct {
	Passed   bool
	Issues   []VerificationIssue
	Summary  string
	TestsPassed bool
	LintPassed  bool
}

// VerificationIssue represents a single issue found during verification
type VerificationIssue struct {
	Severity    string // "error", "warning", "info"
	File        string
	Line        int
	Description string
	Suggestion  string
}

// VerifierAgent verifies changes made by the executor
type VerifierAgent struct {
	client   llm.LLMClient
	config   *config.Config
	registry *tools.Registry
}

// NewVerifierAgent creates a new verifier agent
func NewVerifierAgent(client llm.LLMClient, cfg *config.Config, registry *tools.Registry) *VerifierAgent {
	return &VerifierAgent{
		client:   client,
		config:   cfg,
		registry: registry,
	}
}

// VerifyChanges verifies the changes made in an execution result
func (v *VerifierAgent) VerifyChanges(ctx context.Context, execution *ExecutionResult, changedFiles []string) (*VerificationResult, error) {
	logDebug("VerifierAgent: verifying changes in %d files", len(changedFiles))

	// Use genius model for thorough verification
	originalModel := v.client.GetModel()
	v.client.SetTier(config.TierGenius)
	defer v.client.SetModel(originalModel)

	result := &VerificationResult{
		Passed: true,
	}

	// Step 1: Run linter
	lintResult := v.runLinter(ctx, changedFiles)
	result.LintPassed = lintResult.Passed
	if !lintResult.Passed {
		result.Passed = false
		result.Issues = append(result.Issues, lintResult.Issues...)
	}

	// Step 2: Run tests
	testResult := v.runTests(ctx, changedFiles)
	result.TestsPassed = testResult.Passed
	if !testResult.Passed {
		result.Passed = false
		result.Issues = append(result.Issues, testResult.Issues...)
	}

	// Step 3: LLM code review
	reviewResult := v.reviewCode(ctx, execution, changedFiles)
	result.Issues = append(result.Issues, reviewResult.Issues...)
	if len(reviewResult.Issues) > 0 {
		for _, issue := range reviewResult.Issues {
			if issue.Severity == "error" {
				result.Passed = false
				break
			}
		}
	}

	// Generate summary
	result.Summary = v.generateSummary(result)

	logDebug("VerifierAgent: verification complete, passed=%v, issues=%d", result.Passed, len(result.Issues))
	return result, nil
}

// QuickVerify does a fast verification without LLM review
func (v *VerifierAgent) QuickVerify(ctx context.Context, changedFiles []string) (*VerificationResult, error) {
	logDebug("VerifierAgent: quick verification of %d files", len(changedFiles))

	result := &VerificationResult{
		Passed: true,
	}

	// Just run linter and tests
	lintResult := v.runLinter(ctx, changedFiles)
	result.LintPassed = lintResult.Passed
	result.Issues = append(result.Issues, lintResult.Issues...)

	testResult := v.runTests(ctx, changedFiles)
	result.TestsPassed = testResult.Passed
	result.Issues = append(result.Issues, testResult.Issues...)

	result.Passed = lintResult.Passed && testResult.Passed
	result.Summary = v.generateSummary(result)

	return result, nil
}

func (v *VerifierAgent) runLinter(ctx context.Context, files []string) *VerificationResult {
	result := &VerificationResult{Passed: true}

	lintTool, ok := v.registry.Get("lint")
	if !ok {
		logDebug("VerifierAgent: lint tool not available")
		return result
	}

	// Determine path to lint
	lintPath := "."
	if len(files) == 1 {
		lintPath = files[0]
	}

	output, err := lintTool.Execute(ctx, map[string]any{
		"path": lintPath,
		"fast": true,
	})
	if err != nil {
		logWarn("VerifierAgent: lint failed: %v", err)
		result.Issues = append(result.Issues, VerificationIssue{
			Severity:    "error",
			Description: fmt.Sprintf("Linter failed to run: %s", err.Error()),
		})
		result.Passed = false
		return result
	}

	// Parse lint output for issues
	if strings.Contains(output, "FAIL") || strings.Contains(output, "Errors") {
		result.Passed = false
		result.Issues = append(result.Issues, VerificationIssue{
			Severity:    "error",
			Description: "Linter found issues",
			Suggestion:  output,
		})
	}

	return result
}

func (v *VerifierAgent) runTests(ctx context.Context, files []string) *VerificationResult {
	result := &VerificationResult{Passed: true}

	testTool, ok := v.registry.Get("test_run")
	if !ok {
		logDebug("VerifierAgent: test_run tool not available")
		return result
	}

	// Run tests for the package
	output, err := testTool.Execute(ctx, map[string]any{
		"path":    "./...",
		"short":   true,
		"timeout": "2m",
	})
	if err != nil {
		logWarn("VerifierAgent: tests failed: %v", err)
		result.Issues = append(result.Issues, VerificationIssue{
			Severity:    "error",
			Description: fmt.Sprintf("Tests failed to run: %s", err.Error()),
		})
		result.Passed = false
		return result
	}

	// Parse test output
	if strings.Contains(output, "FAIL") {
		result.Passed = false
		result.Issues = append(result.Issues, VerificationIssue{
			Severity:    "error",
			Description: "Tests failed",
			Suggestion:  output,
		})
	}

	return result
}

func (v *VerifierAgent) reviewCode(ctx context.Context, execution *ExecutionResult, changedFiles []string) *VerificationResult {
	result := &VerificationResult{Passed: true}

	// Build context about what was changed
	var sb strings.Builder
	sb.WriteString("Review the following changes:\n\n")

	// Add execution output
	if execution.Output != "" {
		sb.WriteString("## Execution Output\n")
		sb.WriteString(execution.Output)
		sb.WriteString("\n\n")
	}

	// Add tool calls
	if len(execution.ToolCalls) > 0 {
		sb.WriteString("## Tool Calls\n")
		for _, tc := range execution.ToolCalls {
			sb.WriteString(fmt.Sprintf("- %s: ", tc.Tool))
			if tc.Error != nil {
				sb.WriteString(fmt.Sprintf("ERROR: %s", tc.Error.Error()))
			} else {
				// Truncate long outputs
				output := tc.Output
				if len(output) > 500 {
					output = output[:500] + "..."
				}
				sb.WriteString(output)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Ask LLM to review
	systemPrompt := `You are a code review assistant. Review the changes and identify any issues.

For each issue found, respond in this format:
ISSUE: [error|warning|info] - Description of the issue
SUGGESTION: How to fix it

If no issues are found, respond with:
LGTM - Changes look good

Be thorough but focus on actual problems, not style preferences.`

	messages := []llm.Message{
		{Role: "user", Content: sb.String()},
	}

	resp, err := v.client.Chat(ctx, messages, nil, systemPrompt)
	if err != nil {
		logWarn("VerifierAgent: code review failed: %v", err)
		return result
	}

	// Parse issues from response
	lines := strings.Split(resp.Content, "\n")
	var currentIssue *VerificationIssue

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "ISSUE:") {
			if currentIssue != nil {
				result.Issues = append(result.Issues, *currentIssue)
			}
			currentIssue = &VerificationIssue{}

			// Parse severity and description
			rest := strings.TrimPrefix(line, "ISSUE:")
			rest = strings.TrimSpace(rest)

			if strings.HasPrefix(rest, "[error]") {
				currentIssue.Severity = "error"
				currentIssue.Description = strings.TrimSpace(strings.TrimPrefix(rest, "[error]"))
				currentIssue.Description = strings.TrimPrefix(currentIssue.Description, "- ")
			} else if strings.HasPrefix(rest, "[warning]") {
				currentIssue.Severity = "warning"
				currentIssue.Description = strings.TrimSpace(strings.TrimPrefix(rest, "[warning]"))
				currentIssue.Description = strings.TrimPrefix(currentIssue.Description, "- ")
			} else {
				currentIssue.Severity = "info"
				currentIssue.Description = strings.TrimPrefix(rest, "[info]")
				currentIssue.Description = strings.TrimSpace(strings.TrimPrefix(currentIssue.Description, "- "))
			}
		} else if strings.HasPrefix(line, "SUGGESTION:") && currentIssue != nil {
			currentIssue.Suggestion = strings.TrimSpace(strings.TrimPrefix(line, "SUGGESTION:"))
		}
	}

	if currentIssue != nil {
		result.Issues = append(result.Issues, *currentIssue)
	}

	return result
}

func (v *VerifierAgent) generateSummary(result *VerificationResult) string {
	var parts []string

	if result.LintPassed {
		parts = append(parts, "Lint: PASS")
	} else {
		parts = append(parts, "Lint: FAIL")
	}

	if result.TestsPassed {
		parts = append(parts, "Tests: PASS")
	} else {
		parts = append(parts, "Tests: FAIL")
	}

	errorCount := 0
	warningCount := 0
	for _, issue := range result.Issues {
		switch issue.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		}
	}

	if errorCount > 0 {
		parts = append(parts, fmt.Sprintf("Errors: %d", errorCount))
	}
	if warningCount > 0 {
		parts = append(parts, fmt.Sprintf("Warnings: %d", warningCount))
	}

	status := "PASSED"
	if !result.Passed {
		status = "FAILED"
	}

	return fmt.Sprintf("[%s] %s", status, strings.Join(parts, " | "))
}

// FormatResult returns a human-readable representation of the verification result
func (v *VerifierAgent) FormatResult(result *VerificationResult) string {
	var sb strings.Builder

	sb.WriteString("## Verification Result\n\n")
	sb.WriteString(fmt.Sprintf("**Status:** %s\n\n", result.Summary))

	if len(result.Issues) > 0 {
		sb.WriteString("### Issues Found\n")
		for _, issue := range result.Issues {
			icon := "info"
			switch issue.Severity {
			case "error":
				icon = "error"
			case "warning":
				icon = "warning"
			}

			sb.WriteString(fmt.Sprintf("\n**[%s]** %s\n", icon, issue.Description))
			if issue.File != "" {
				sb.WriteString(fmt.Sprintf("  File: %s", issue.File))
				if issue.Line > 0 {
					sb.WriteString(fmt.Sprintf(":%d", issue.Line))
				}
				sb.WriteString("\n")
			}
			if issue.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("  Suggestion: %s\n", issue.Suggestion))
			}
		}
	} else {
		sb.WriteString("No issues found.\n")
	}

	return sb.String()
}
