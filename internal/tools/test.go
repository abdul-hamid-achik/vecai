package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TestRunnerTool runs go test and parses the results
type TestRunnerTool struct{}

func (t *TestRunnerTool) Name() string {
	return "test_run"
}

func (t *TestRunnerTool) Description() string {
	return "Run Go tests and return results. Can run tests for a specific file, package, or the entire project. Supports running specific tests by name pattern."
}

func (t *TestRunnerTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to test: package path, directory, or '.' for current package. Use './...' for all packages. Default: '.'",
				"default":     ".",
			},
			"run": map[string]any{
				"type":        "string",
				"description": "Regular expression to select tests to run (passed to -run flag). Example: 'TestFoo' or 'Test.*Config'",
			},
			"verbose": map[string]any{
				"type":        "boolean",
				"description": "Show verbose output including all test names. Default: false",
				"default":     false,
			},
			"short": map[string]any{
				"type":        "boolean",
				"description": "Run in short mode (-short flag), skipping long-running tests. Default: false",
				"default":     false,
			},
			"timeout": map[string]any{
				"type":        "string",
				"description": "Test timeout (e.g., '30s', '5m'). Default: '5m'",
				"default":     "5m",
			},
			"cover": map[string]any{
				"type":        "boolean",
				"description": "Show coverage information. Default: false",
				"default":     false,
			},
		},
	}
}

func (t *TestRunnerTool) Permission() PermissionLevel {
	return PermissionExecute
}

// TestEvent represents a single test event from go test -json
type TestEvent struct {
	Time    string  `json:"Time"`
	Action  string  `json:"Action"` // run, pause, cont, pass, bench, fail, output, skip
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

// TestResult holds the parsed test results
type TestResult struct {
	Package  string
	Test     string
	Passed   bool
	Skipped  bool
	Failed   bool
	Duration float64
	Output   []string
}

func (t *TestRunnerTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	// Get path
	path := "."
	if p, ok := input["path"].(string); ok && p != "" {
		path = p
	}

	// Get options
	runPattern := ""
	if r, ok := input["run"].(string); ok {
		runPattern = r
	}

	verbose := false
	if v, ok := input["verbose"].(bool); ok {
		verbose = v
	}

	short := false
	if s, ok := input["short"].(bool); ok {
		short = s
	}

	timeout := "5m"
	if to, ok := input["timeout"].(string); ok && to != "" {
		timeout = to
	}

	cover := false
	if c, ok := input["cover"].(bool); ok {
		cover = c
	}

	// Resolve path
	absPath := path
	if !strings.HasPrefix(path, "./") && path != "./..." {
		var err error
		absPath, err = filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}
	}

	// Build command
	args := []string{"test", "-json"}
	if verbose {
		args = append(args, "-v")
	}
	if short {
		args = append(args, "-short")
	}
	if runPattern != "" {
		args = append(args, "-run", runPattern)
	}
	if cover {
		args = append(args, "-cover")
	}
	args = append(args, "-timeout", timeout)
	args = append(args, absPath)

	// Parse timeout for context
	ctxTimeout, err := time.ParseDuration(timeout)
	if err != nil {
		ctxTimeout = 5 * time.Minute
	}
	// Add buffer for go test overhead
	ctx, cancel := context.WithTimeout(ctx, ctxTimeout+30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run test (don't check error - failing tests return exit code 1)
	_ = cmd.Run()

	// Check for compilation errors
	if stderr.Len() > 0 {
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "build failed") || strings.Contains(stderrStr, "cannot find") {
			return "", fmt.Errorf("build error:\n%s", stderrStr)
		}
	}

	// Parse JSON output
	if stdout.Len() == 0 {
		if stderr.Len() > 0 {
			return fmt.Sprintf("Test output:\n%s", stderr.String()), nil
		}
		return "No test output.", nil
	}

	return t.parseTestOutput(stdout.String(), verbose)
}

func (t *TestRunnerTool) parseTestOutput(output string, verbose bool) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))

	results := make(map[string]*TestResult) // key: package/test
	var packageOrder []string               // maintain order
	var coverage []string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event TestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Not JSON, might be coverage or other output
			if strings.Contains(line, "coverage:") {
				coverage = append(coverage, line)
			}
			continue
		}

		// Track by package/test
		key := event.Package
		if event.Test != "" {
			key = event.Package + "/" + event.Test
		}

		if _, exists := results[key]; !exists {
			results[key] = &TestResult{
				Package: event.Package,
				Test:    event.Test,
			}
			if event.Test == "" {
				packageOrder = append(packageOrder, key)
			}
		}

		result := results[key]

		switch event.Action {
		case "pass":
			result.Passed = true
			result.Duration = event.Elapsed
		case "fail":
			result.Failed = true
			result.Duration = event.Elapsed
		case "skip":
			result.Skipped = true
		case "output":
			if verbose || event.Test == "" || result.Failed {
				result.Output = append(result.Output, event.Output)
			}
		}
	}

	// Format output
	return t.formatResults(results, packageOrder, coverage, verbose), nil
}

func (t *TestRunnerTool) formatResults(results map[string]*TestResult, packageOrder []string, coverage []string, verbose bool) string {
	var sb strings.Builder

	// Count totals
	var passed, failed, skipped int
	for _, r := range results {
		if r.Test != "" { // Only count actual tests, not packages
			if r.Passed {
				passed++
			} else if r.Failed {
				failed++
			} else if r.Skipped {
				skipped++
			}
		}
	}

	// Summary header
	status := "PASS"
	if failed > 0 {
		status = "FAIL"
	}
	sb.WriteString(fmt.Sprintf("## Test Results: %s\n", status))
	sb.WriteString(fmt.Sprintf("Passed: %d | Failed: %d | Skipped: %d\n\n", passed, failed, skipped))

	// Show failures first (always)
	var failures []*TestResult
	for _, r := range results {
		if r.Failed && r.Test != "" {
			failures = append(failures, r)
		}
	}

	if len(failures) > 0 {
		sb.WriteString("### Failed Tests\n")
		for _, r := range failures {
			sb.WriteString(fmt.Sprintf("\n**%s/%s** (%.2fs)\n", r.Package, r.Test, r.Duration))
			if len(r.Output) > 0 {
				sb.WriteString("```\n")
				for _, line := range r.Output {
					// Trim common test output prefixes
					line = strings.TrimPrefix(line, "    ")
					sb.WriteString(line)
				}
				sb.WriteString("```\n")
			}
		}
		sb.WriteString("\n")
	}

	// Show all tests if verbose
	if verbose && passed > 0 {
		sb.WriteString("### Passed Tests\n")
		for _, r := range results {
			if r.Passed && r.Test != "" {
				sb.WriteString(fmt.Sprintf("  %s/%s (%.2fs)\n", r.Package, r.Test, r.Duration))
			}
		}
		sb.WriteString("\n")
	}

	// Show skipped tests
	if skipped > 0 {
		sb.WriteString("### Skipped Tests\n")
		for _, r := range results {
			if r.Skipped && r.Test != "" {
				sb.WriteString(fmt.Sprintf("  %s/%s\n", r.Package, r.Test))
			}
		}
		sb.WriteString("\n")
	}

	// Show coverage
	if len(coverage) > 0 {
		sb.WriteString("### Coverage\n")
		for _, c := range coverage {
			sb.WriteString(fmt.Sprintf("  %s\n", strings.TrimSpace(c)))
		}
		sb.WriteString("\n")
	}

	// Package summary
	sb.WriteString("### Packages\n")
	for _, key := range packageOrder {
		r := results[key]
		if r.Test == "" {
			status := "ok"
			if r.Failed {
				status = "FAIL"
			} else if r.Skipped {
				status = "skip"
			}
			sb.WriteString(fmt.Sprintf("  %s %s (%.2fs)\n", status, r.Package, r.Duration))
		}
	}

	return sb.String()
}
