package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/debug"
	vecerr "github.com/abdul-hamid-achik/vecai/internal/errors"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// Pipeline orchestrates multi-agent workflows
type Pipeline struct {
	router   *TaskRouter
	planner  *PlannerAgent
	executor *ExecutorAgent
	verifier *VerifierAgent
	config   *config.Config
}

// PipelineConfig contains configuration for the pipeline
type PipelineConfig struct {
	Client      llm.LLMClient
	Config      *config.Config
	Registry    *tools.Registry
	Permissions *permissions.Policy
}

// NewPipeline creates a new multi-agent pipeline.
// Each sub-agent gets its own forked LLM client so tier/model changes
// in one agent do not affect the others.
func NewPipeline(cfg PipelineConfig) *Pipeline {
	return &Pipeline{
		router:   NewTaskRouter(cfg.Client.Fork(), cfg.Config),
		planner:  NewPlannerAgent(cfg.Client.Fork(), cfg.Config, cfg.Registry),
		executor: NewExecutorAgent(cfg.Client.Fork(), cfg.Config, cfg.Registry, cfg.Permissions),
		verifier: NewVerifierAgent(cfg.Client.Fork(), cfg.Config, cfg.Registry),
		config:   cfg.Config,
	}
}

// PipelineResult represents the result of a pipeline execution
type PipelineResult struct {
	Success       bool
	Intent        Intent
	Plan          *StructuredPlan
	Executions    []*ExecutionResult
	Verification  *VerificationResult
	FinalOutput   string
	Errors        []error
}

// Execute runs the appropriate pipeline for a task.
// output is the AgentOutput adapter for displaying progress messages —
// in CLI mode this is a CLIOutput, in TUI mode a TUIOutput.
func (p *Pipeline) Execute(ctx context.Context, task string, output AgentOutput) (*PipelineResult, error) {
	result := &PipelineResult{}

	// Step 1: Classify intent
	result.Intent = p.router.ClassifyIntent(ctx, task)
	logDebug("Pipeline: classified intent as %s", result.Intent)

	// Log intent classification to debug tracer
	truncatedTask := task
	if len(truncatedTask) > 100 {
		truncatedTask = truncatedTask[:100] + "..."
	}
	debug.Event_(debug.EventIntentClassified, map[string]any{
		"query":  truncatedTask,
		"intent": string(result.Intent),
	})

	// Step 2: Route to appropriate flow
	if p.router.ShouldUseMultiAgent(result.Intent) {
		return p.executeMultiAgentFlow(ctx, task, result, output)
	}

	return p.executeSingleAgentFlow(ctx, task, result)
}

// ExecuteWithIntent runs the pipeline with a pre-classified intent,
// skipping the duplicate ClassifyIntent call when the caller already knows the intent.
func (p *Pipeline) ExecuteWithIntent(ctx context.Context, task string, intent Intent, output AgentOutput) (*PipelineResult, error) {
	result := &PipelineResult{
		Intent: intent,
	}

	logDebug("Pipeline: using pre-classified intent %s", intent)

	// Log intent to debug tracer
	truncatedTask := task
	if len(truncatedTask) > 100 {
		truncatedTask = truncatedTask[:100] + "..."
	}
	debug.Event_(debug.EventIntentClassified, map[string]any{
		"query":  truncatedTask,
		"intent": string(intent),
	})

	// Route to appropriate flow
	if p.router.ShouldUseMultiAgent(intent) {
		return p.executeMultiAgentFlow(ctx, task, result, output)
	}

	return p.executeSingleAgentFlow(ctx, task, result)
}

// executeMultiAgentFlow handles complex tasks with planning
func (p *Pipeline) executeMultiAgentFlow(ctx context.Context, task string, result *PipelineResult, output AgentOutput) (*PipelineResult, error) {
	logDebug("Pipeline: using multi-agent flow")

	// Step 1: Create plan
	output.Info("Creating plan...")
	plan, err := p.planner.CreatePlan(ctx, task, "")
	if err != nil {
		debug.Error("planning", err, map[string]any{"task": task})
		result.Errors = append(result.Errors, fmt.Errorf("planning failed: %w", err))
		return result, nil
	}
	result.Plan = plan

	// Log plan creation to debug tracer
	debug.PlanCreated(plan.Goal, len(plan.Steps))

	// Show plan (use Glamour-rendered plan block if supported, else plain text)
	planText := p.planner.FormatPlan(plan)
	if ps, ok := output.(PlanSupport); ok {
		ps.Plan(planText)
	} else {
		output.TextLn(planText)
	}

	// Gate: require user approval before executing the plan.
	// In TUI mode, we must send PermissionPrompt first to put the TUI into
	// StatePermission before blocking on ReadLine (which waits on resultChan).
	// Without PermissionPrompt, the TUI never shows the approval dialog and deadlocks.
	if inp, ok := output.(AgentInput); ok {
		output.PermissionPrompt("plan", tools.PermissionWrite, "Execute this plan? (y to confirm, n to cancel)")
		response, err := inp.ReadLine("")
		if err != nil {
			output.Info("Plan execution cancelled")
			return result, nil
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" && response != "a" {
			output.Info("Plan execution cancelled")
			return result, nil
		}
	}

	// Step 2: Execute steps
	var previousContext string
	maxRetries := p.config.Agent.MaxRetries

	for !p.planner.IsPlanComplete(plan) {
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, fmt.Errorf("pipeline cancelled: %w", ctx.Err()))
			return result, nil
		default:
		}

		step := p.planner.GetNextStep(plan)
		if step == nil {
			result.Errors = append(result.Errors, fmt.Errorf("no executable steps found but plan not complete"))
			break
		}

		// Log step start
		debug.StepStart(step.ID, step.Description)

		output.Info(fmt.Sprintf("Executing: %s", cleanStepDescription(step.Description)))

		// Try step with retries
		var execResult *ExecutionResult
		var lastErr error

		for attempt := 0; attempt < maxRetries; attempt++ {
			select {
			case <-ctx.Done():
				result.Errors = append(result.Errors, fmt.Errorf("pipeline cancelled during retry: %w", ctx.Err()))
				return result, nil
			default:
			}

			if attempt > 0 {
				output.Info(fmt.Sprintf("Retrying step (attempt %d/%d)...", attempt+1, maxRetries))
			}

			execResult, lastErr = p.executor.ExecuteStep(ctx, step, previousContext)
			if lastErr == nil && execResult.Success {
				break
			}

			if execResult != nil && execResult.Error != nil {
				lastErr = execResult.Error
			}
		}

		if execResult != nil {
			result.Executions = append(result.Executions, execResult)

			if execResult.Success {
				p.planner.MarkStepDone(plan, step.ID)
				debug.StepComplete(step.ID, true, nil)
				output.Success(fmt.Sprintf("Completed: %s", cleanStepDescription(step.Description)))
				// Update plan display with new checkmarks
				if ps, ok := output.(PlanSupport); ok {
					ps.PlanUpdate(p.planner.FormatPlan(plan))
				}
				previousContext += fmt.Sprintf("\n--- Step %s completed ---\n%s\n", step.ID, execResult.Output)
			} else {
				debug.StepComplete(step.ID, false, lastErr)
				result.Errors = append(result.Errors, vecerr.PipelineStepFailed(step.ID, lastErr))
				// Continue to next step or break based on error severity
				if lastErr != nil {
					break
				}
			}
		}
	}

	// Step 3: Verify if enabled
	if p.config.Agent.VerificationEnabled && len(result.Executions) > 0 {
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, fmt.Errorf("pipeline cancelled before verification: %w", ctx.Err()))
			return result, nil
		default:
		}

		output.Info("Verifying changes...")
		changedFiles := p.extractChangedFiles(result.Executions)
		verification, err := p.verifier.VerifyChanges(ctx, result.Executions[len(result.Executions)-1], changedFiles)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("verification failed: %w", err))
		} else {
			result.Verification = verification
			output.TextLn(p.verifier.FormatResult(verification))
		}
	}

	result.Success = len(result.Errors) == 0 && p.planner.IsPlanComplete(plan)
	result.FinalOutput = p.generateFinalOutput(result)

	return result, nil
}

// executeSingleAgentFlow handles simple tasks directly
func (p *Pipeline) executeSingleAgentFlow(ctx context.Context, task string, result *PipelineResult) (*PipelineResult, error) {
	logDebug("Pipeline: using single-agent flow")

	execResult, err := p.executor.ExecuteDirectTask(ctx, task, result.Intent)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result, nil
	}

	result.Executions = append(result.Executions, execResult)
	result.Success = execResult.Success
	result.FinalOutput = execResult.Output

	// Quick verification for code changes
	if result.Intent == IntentCode && p.config.Agent.VerificationEnabled {
		changedFiles := p.extractChangedFiles(result.Executions)
		if len(changedFiles) > 0 {
			verification, _ := p.verifier.QuickVerify(ctx, changedFiles)
			result.Verification = verification
		}
	}

	return result, nil
}

// extractChangedFiles extracts file paths from tool calls
func (p *Pipeline) extractChangedFiles(executions []*ExecutionResult) []string {
	fileSet := make(map[string]bool)

	for _, exec := range executions {
		for _, tc := range exec.ToolCalls {
			// Check for file write/edit tools
			if tc.Tool == "write_file" || tc.Tool == "edit_file" {
				if path, ok := tc.Input["path"].(string); ok {
					fileSet[path] = true
				}
			}
		}
	}

	files := make([]string, 0, len(fileSet))
	for f := range fileSet {
		files = append(files, f)
	}
	return files
}

// generateFinalOutput creates a summary of the pipeline execution
func (p *Pipeline) generateFinalOutput(result *PipelineResult) string {
	var output string

	if result.Plan != nil {
		completed := 0
		for _, step := range result.Plan.Steps {
			if step.Done {
				completed++
			}
		}
		output += fmt.Sprintf("Plan: %d/%d steps completed\n", completed, len(result.Plan.Steps))
	}

	if result.Verification != nil {
		output += fmt.Sprintf("Verification: %s\n", result.Verification.Summary)
	}

	if len(result.Errors) > 0 {
		output += fmt.Sprintf("Errors: %d\n", len(result.Errors))
	}

	return output
}

// GetRouter returns the task router for external use
func (p *Pipeline) GetRouter() *TaskRouter {
	return p.router
}

// GetPlanner returns the planner agent for external use
func (p *Pipeline) GetPlanner() *PlannerAgent {
	return p.planner
}

// GetExecutor returns the executor agent for external use
func (p *Pipeline) GetExecutor() *ExecutorAgent {
	return p.executor
}

// GetVerifier returns the verifier agent for external use
func (p *Pipeline) GetVerifier() *VerifierAgent {
	return p.verifier
}
