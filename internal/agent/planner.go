package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/llm"
)

const plannerSystemPrompt = `You are a planning assistant. Your job is to help break down complex goals into actionable steps.

When creating a plan:
1. First, use tools to understand the current codebase state
2. Identify questions that need user input
3. Create clear, ordered steps

Your plans should be:
- Specific and actionable
- Ordered by dependency (what needs to happen first)
- Include verification steps

Output your plan in this format:

## Questions (if any)
1. Question about requirement/approach?
2. Another clarifying question?

## Plan
1. **Step name**: Description of what to do
2. **Step name**: Description of what to do
...

## Risks
- Any potential issues or things to watch out for`

// Planner handles plan creation and execution
type Planner struct {
	agent *Agent
}

// NewPlanner creates a new planner
func NewPlanner(agent *Agent) *Planner {
	return &Planner{agent: agent}
}

// Execute runs the full planning flow
func (p *Planner) Execute(goal string) error {
	ctx := context.Background()

	p.agent.output.Header("Plan Mode")
	p.agent.output.TextLn("Goal: " + goal)
	p.agent.output.Separator()

	// Phase 1: Gather context
	p.agent.output.Info("Gathering context...")
	plan, err := p.createPlan(ctx, goal)
	if err != nil {
		return fmt.Errorf("failed to create plan: %w", err)
	}

	// Phase 2: Show plan and ask questions
	p.agent.output.TextLn(plan.Content)

	if len(plan.Questions) > 0 {
		p.agent.output.Header("Questions")
		answers, err := p.runQuestionnaire(plan.Questions)
		if err != nil {
			return fmt.Errorf("questionnaire failed: %w", err)
		}

		// Update plan with answers
		plan, err = p.refinePlan(ctx, goal, plan, answers)
		if err != nil {
			return fmt.Errorf("failed to refine plan: %w", err)
		}

		p.agent.output.Header("Updated Plan")
		p.agent.output.TextLn(plan.Content)
	}

	// Phase 3: Confirm and execute
	confirmed, err := p.agent.input.Confirm("Execute this plan?", false)
	if err != nil {
		return err
	}

	if !confirmed {
		p.agent.output.Info("Plan cancelled")
		return nil
	}

	// Phase 4: Execute steps
	return p.executePlan(ctx, plan)
}

// Plan represents a generated plan
type Plan struct {
	Content   string
	Questions []string
	Steps     []PlanStep
}

// PlanStep represents a single step in the plan
type PlanStep struct {
	Name        string
	Description string
	Status      string // pending, in_progress, completed, failed
}

// createPlan generates an initial plan
func (p *Planner) createPlan(ctx context.Context, goal string) (*Plan, error) {
	// Create a separate conversation for planning
	messages := []llm.Message{
		{
			Role:    "user",
			Content: fmt.Sprintf("I want to: %s\n\nPlease analyze the codebase and create a plan. Use the available tools to understand the current state.", goal),
		},
	}

	// Get tool definitions
	toolDefs := p.agent.getToolDefinitions()

	// Run the planning loop
	const maxIterations = 10
	var lastContent string

	for i := 0; i < maxIterations; i++ {
		resp, err := p.agent.llm.Chat(ctx, messages, toolDefs, plannerSystemPrompt)
		if err != nil {
			return nil, err
		}

		lastContent = resp.Content

		// If no tool calls, we have the plan
		if len(resp.ToolCalls) == 0 {
			break
		}

		// Execute tool calls
		results := p.agent.executeToolCalls(ctx, resp.ToolCalls)

		// Add results to conversation
		var resultContent strings.Builder
		for _, result := range results {
			resultContent.WriteString(fmt.Sprintf("Tool %s result:\n%s\n\n", result.Name, result.Result))
		}

		messages = append(messages,
			llm.Message{Role: "assistant", Content: resp.Content},
			llm.Message{Role: "user", Content: resultContent.String()},
		)
	}

	// Parse the plan
	plan := &Plan{
		Content: lastContent,
	}

	// Extract questions
	plan.Questions = extractQuestions(lastContent)

	// Extract steps
	plan.Steps = extractSteps(lastContent)

	return plan, nil
}

// runQuestionnaire asks the user questions and collects answers
func (p *Planner) runQuestionnaire(questions []string) (map[string]string, error) {
	answers := make(map[string]string)

	for i, q := range questions {
		p.agent.output.TextLn(fmt.Sprintf("%d. %s", i+1, q))
		answer, err := p.agent.input.ReadLine("   Answer: ")
		if err != nil {
			return nil, err
		}
		answers[q] = answer
	}

	return answers, nil
}

// refinePlan updates the plan based on user answers
func (p *Planner) refinePlan(ctx context.Context, goal string, originalPlan *Plan, answers map[string]string) (*Plan, error) {
	// Build context with answers
	var answerText strings.Builder
	answerText.WriteString("User answers to questions:\n")
	for q, a := range answers {
		answerText.WriteString(fmt.Sprintf("Q: %s\nA: %s\n\n", q, a))
	}

	messages := []llm.Message{
		{
			Role:    "user",
			Content: fmt.Sprintf("Original goal: %s\n\nOriginal plan:\n%s\n\n%s\n\nPlease update the plan based on these answers.", goal, originalPlan.Content, answerText.String()),
		},
	}

	resp, err := p.agent.llm.Chat(ctx, messages, nil, plannerSystemPrompt)
	if err != nil {
		return nil, err
	}

	return &Plan{
		Content:   resp.Content,
		Questions: []string{}, // No more questions
		Steps:     extractSteps(resp.Content),
	}, nil
}

// executePlan executes the plan steps
func (p *Planner) executePlan(ctx context.Context, plan *Plan) error {
	p.agent.output.Header("Executing Plan")

	// Use the main agent to execute
	prompt := fmt.Sprintf(`Execute the following plan step by step. After each step, report progress.

%s

Begin execution.`, plan.Content)

	return p.agent.Run(prompt)
}

// extractQuestions extracts questions from plan content
func extractQuestions(content string) []string {
	var questions []string

	lines := strings.Split(content, "\n")
	inQuestions := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(strings.ToLower(line), "## questions") {
			inQuestions = true
			continue
		}

		if strings.HasPrefix(line, "## ") {
			inQuestions = false
			continue
		}

		if inQuestions && line != "" {
			// Remove number prefix if present
			if len(line) > 2 && line[0] >= '0' && line[0] <= '9' && line[1] == '.' {
				line = strings.TrimSpace(line[2:])
			}
			if line != "" && strings.HasSuffix(line, "?") {
				questions = append(questions, line)
			}
		}
	}

	return questions
}

// extractSteps extracts plan steps from content
func extractSteps(content string) []PlanStep {
	var steps []PlanStep

	lines := strings.Split(content, "\n")
	inPlan := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(strings.ToLower(line), "## plan") {
			inPlan = true
			continue
		}

		if strings.HasPrefix(line, "## ") {
			inPlan = false
			continue
		}

		if inPlan && line != "" {
			// Check for numbered step
			if len(line) > 2 && line[0] >= '0' && line[0] <= '9' && line[1] == '.' {
				stepText := strings.TrimSpace(line[2:])
				// Try to extract name and description
				parts := strings.SplitN(stepText, ":", 2)
				step := PlanStep{Status: "pending"}
				if len(parts) == 2 {
					step.Name = strings.TrimSpace(strings.Trim(parts[0], "*"))
					step.Description = strings.TrimSpace(parts[1])
				} else {
					step.Name = stepText
					step.Description = stepText
				}
				steps = append(steps, step)
			}
		}
	}

	return steps
}
