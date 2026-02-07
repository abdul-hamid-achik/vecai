package agent

import (
	"context"
	"fmt"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/debug"
	vecerr "github.com/abdul-hamid-achik/vecai/internal/errors"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/ui"
)

// Pipeline orchestrates multi-agent workflows
type Pipeline struct {
	router   *TaskRouter
	planner  *PlannerAgent
	executor *ExecutorAgent
	verifier *VerifierAgent
	output   ui.OutputHandler
	config   *config.Config
}

// PipelineConfig contains configuration for the pipeline
type PipelineConfig struct {
	Client      llm.LLMClient
	Config      *config.Config
	Registry    *tools.Registry
	Permissions *permissions.Policy
	Output      ui.OutputHandler
}

// NewPipeline creates a new multi-agent pipeline
func NewPipeline(cfg PipelineConfig) *Pipeline {
	return &Pipeline{
		router:   NewTaskRouter(cfg.Client, cfg.Config),
		planner:  NewPlannerAgent(cfg.Client, cfg.Config, cfg.Registry),
		executor: NewExecutorAgent(cfg.Client, cfg.Config, cfg.Registry, cfg.Permissions),
		verifier: NewVerifierAgent(cfg.Client, cfg.Config, cfg.Registry),
		output:   cfg.Output,
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

// Execute runs the appropriate pipeline for a task
func (p *Pipeline) Execute(ctx context.Context, task string) (*PipelineResult, error) {
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
		return p.executeMultiAgentFlow(ctx, task, result)
	}

	return p.executeSingleAgentFlow(ctx, task, result)
}

// executeMultiAgentFlow handles complex tasks with planning
func (p *Pipeline) executeMultiAgentFlow(ctx context.Context, task string, result *PipelineResult) (*PipelineResult, error) {
	logDebug("Pipeline: using multi-agent flow")

	// Step 1: Create plan
	p.output.Info("Creating plan...")
	plan, err := p.planner.CreatePlan(ctx, task, "")
	if err != nil {
		debug.Error("planning", err, map[string]any{"task": task})
		result.Errors = append(result.Errors, fmt.Errorf("planning failed: %w", err))
		return result, nil
	}
	result.Plan = plan

	// Log plan creation to debug tracer
	debug.PlanCreated(plan.Goal, len(plan.Steps))

	// Show plan
	p.output.TextLn(p.planner.FormatPlan(plan))

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

		p.output.Info(fmt.Sprintf("Executing: %s", step.Description))

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
				p.output.Info(fmt.Sprintf("Retrying step (attempt %d/%d)...", attempt+1, maxRetries))
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

		p.output.Info("Verifying changes...")
		changedFiles := p.extractChangedFiles(result.Executions)
		verification, err := p.verifier.VerifyChanges(ctx, result.Executions[len(result.Executions)-1], changedFiles)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("verification failed: %w", err))
		} else {
			result.Verification = verification
			p.output.TextLn(p.verifier.FormatResult(verification))
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
