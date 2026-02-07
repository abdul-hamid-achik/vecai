package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

// StructuredPlan represents a complete plan for a task
type StructuredPlan struct {
	Goal        string     `json:"goal"`
	Summary     string     `json:"summary"`
	Steps       []PlanStep `json:"steps"`
	Risks       []string   `json:"risks,omitempty"`
	Assumptions []string   `json:"assumptions,omitempty"`
}

// PlannerAgent creates structured plans for complex tasks
type PlannerAgent struct {
	client   llm.LLMClient
	config   *config.Config
	registry *tools.Registry
}

// NewPlannerAgent creates a new planner agent
func NewPlannerAgent(client llm.LLMClient, cfg *config.Config, registry *tools.Registry) *PlannerAgent {
	return &PlannerAgent{
		client:   client,
		config:   cfg,
		registry: registry,
	}
}

// CreatePlan generates a structured plan for a goal
func (p *PlannerAgent) CreatePlan(ctx context.Context, goal string, codebaseContext string) (*StructuredPlan, error) {
	logDebug("PlannerAgent: creating plan for goal: %s", goal)

	// Use fast model for planning (good at structured thinking)
	originalModel := p.client.GetModel()
	p.client.SetTier(config.TierFast)
	defer p.client.SetModel(originalModel)

	systemPrompt := p.buildSystemPrompt()

	userPrompt := fmt.Sprintf(`Create a detailed plan to accomplish the following goal:

%s

Codebase context (if available):
%s

Respond with a JSON object containing:
- goal: The original goal
- summary: A brief summary of the approach (1-2 sentences)
- steps: An array of steps, each with:
  - id: Unique identifier (e.g., "step1", "step2")
  - description: What to do in this step
  - type: One of "read" (explore code), "code" (write/modify), "test" (run tests), "verify" (check results)
  - files: Array of files this step will touch (if known)
  - dependencies: Array of step IDs that must complete first
- risks: Array of potential risks or challenges
- assumptions: Array of assumptions being made

IMPORTANT: Keep the plan focused and actionable. Each step should be achievable in one LLM turn.`, goal, codebaseContext)

	messages := []llm.Message{
		{Role: "user", Content: userPrompt},
	}

	// Get tool definitions for read-only tools (for exploration)
	readOnlyTools := p.getReadOnlyTools()

	resp, err := p.client.Chat(ctx, messages, readOnlyTools, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("planner failed: %w", err)
	}

	// Parse the plan from response
	plan, err := p.parsePlan(resp.Content)
	if err != nil {
		logWarn("PlannerAgent: failed to parse structured plan, using text plan: %v", err)
		plan = p.buildFallbackPlan(goal, resp.Content)
	}

	logDebug("PlannerAgent: created plan with %d steps", len(plan.Steps))
	return plan, nil
}

// RefinePlan improves an existing plan based on feedback
func (p *PlannerAgent) RefinePlan(ctx context.Context, plan *StructuredPlan, feedback string) (*StructuredPlan, error) {
	logDebug("PlannerAgent: refining plan based on feedback")

	originalModel := p.client.GetModel()
	p.client.SetTier(config.TierFast)
	defer p.client.SetModel(originalModel)

	systemPrompt := p.buildSystemPrompt()

	planJSON, _ := json.MarshalIndent(plan, "", "  ")
	userPrompt := fmt.Sprintf(`Refine the following plan based on the feedback:

Current Plan:
%s

Feedback:
%s

Return the improved plan in the same JSON format.`, string(planJSON), feedback)

	messages := []llm.Message{
		{Role: "user", Content: userPrompt},
	}

	resp, err := p.client.Chat(ctx, messages, nil, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("plan refinement failed: %w", err)
	}

	refinedPlan, err := p.parsePlan(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse refined plan: %w", err)
	}

	return refinedPlan, nil
}

// GetNextStep returns the next executable step (no unmet dependencies)
func (p *PlannerAgent) GetNextStep(plan *StructuredPlan) *PlanStep {
	completedIDs := make(map[string]bool)
	for i := range plan.Steps {
		if plan.Steps[i].Done {
			completedIDs[plan.Steps[i].ID] = true
		}
	}

	for i := range plan.Steps {
		step := &plan.Steps[i]
		if step.Done {
			continue
		}

		// Check if all dependencies are met
		allDepsComplete := true
		for _, dep := range step.Dependencies {
			if !completedIDs[dep] {
				allDepsComplete = false
				break
			}
		}

		if allDepsComplete {
			return step
		}
	}

	return nil // No executable steps
}

// IsPlanComplete returns true if all steps are done
func (p *PlannerAgent) IsPlanComplete(plan *StructuredPlan) bool {
	for _, step := range plan.Steps {
		if !step.Done {
			return false
		}
	}
	return true
}

// MarkStepDone marks a step as completed
func (p *PlannerAgent) MarkStepDone(plan *StructuredPlan, stepID string) {
	for i := range plan.Steps {
		if plan.Steps[i].ID == stepID {
			plan.Steps[i].Done = true
			break
		}
	}
}

func (p *PlannerAgent) buildSystemPrompt() string {
	return `You are a planning agent that creates structured, actionable plans for software development tasks.

Your role is to:
1. Break down complex goals into discrete, achievable steps
2. Identify dependencies between steps
3. Consider risks and make assumptions explicit
4. Ensure each step is small enough to complete in one turn

Guidelines:
- Each step should do ONE thing well
- "read" steps gather information
- "code" steps write or modify code
- "test" steps verify changes work
- "verify" steps ensure quality
- Dependencies should form a valid DAG (no cycles)
- Be specific about which files will be touched

Always respond with valid JSON.`
}

func (p *PlannerAgent) getReadOnlyTools() []llm.ToolDefinition {
	readOnlyNames := []string{
		"read_file", "list_files", "grep",
		"vecgrep_search", "vecgrep_similar",
		"ast_parse", "lsp_query",
	}

	var defs []llm.ToolDefinition
	for _, name := range readOnlyNames {
		if tool, ok := p.registry.Get(name); ok {
			defs = append(defs, llm.ToolDefinition{
				Name:        tool.Name(),
				Description: tool.Description(),
				InputSchema: tool.InputSchema(),
			})
		}
	}
	return defs
}

func (p *PlannerAgent) parsePlan(content string) (*StructuredPlan, error) {
	// Try to extract JSON from the response
	content = strings.TrimSpace(content)

	// Strip markdown code fences (```json ... ``` or ``` ... ```)
	content = stripMarkdownFences(content)

	// Look for JSON object in the content
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	jsonStr := content[start : end+1]

	var plan StructuredPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate plan
	if len(plan.Steps) == 0 {
		return nil, fmt.Errorf("plan has no steps")
	}

	// Ensure all steps have IDs
	for i := range plan.Steps {
		if plan.Steps[i].ID == "" {
			plan.Steps[i].ID = fmt.Sprintf("step%d", i+1)
		}
		if plan.Steps[i].Type == "" {
			plan.Steps[i].Type = "code"
		}
	}

	return &plan, nil
}

// stripMarkdownFences removes markdown code fences from content.
// Handles ```json\n...\n```, ```\n...\n```, and similar patterns.
func stripMarkdownFences(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 {
		return content
	}

	first := strings.TrimSpace(lines[0])
	last := strings.TrimSpace(lines[len(lines)-1])

	// Check if wrapped in code fences
	if strings.HasPrefix(first, "```") && last == "```" {
		return strings.Join(lines[1:len(lines)-1], "\n")
	}

	return content
}

// buildFallbackPlan creates a plan from an unstructured LLM response.
// It tries to extract meaningful steps rather than dumping raw content.
func (p *PlannerAgent) buildFallbackPlan(goal, content string) *StructuredPlan {
	content = strings.TrimSpace(content)

	// Try lenient JSON parse: the content might be JSON with goal/steps but failed strict parse
	content = stripMarkdownFences(content)
	if looksLikeJSON(content) {
		// Try to extract goal/summary from JSON-like content
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(content), &raw); err == nil {
			if summary, ok := raw["summary"].(string); ok {
				steps := extractStepsFromJSON(raw)
				if len(steps) > 0 {
					return &StructuredPlan{
						Goal:    goal,
						Summary: summary,
						Steps:   steps,
					}
				}
			}
		}
	}

	// Try to extract numbered items as steps (e.g., "1. Do X\n2. Do Y")
	numberedPattern := regexp.MustCompile(`(?m)^\s*(\d+)[.)]\s+(.+)$`)
	matches := numberedPattern.FindAllStringSubmatch(content, -1)
	if len(matches) >= 2 {
		var steps []PlanStep
		for i, m := range matches {
			steps = append(steps, PlanStep{
				ID:          fmt.Sprintf("step%d", i+1),
				Description: strings.TrimSpace(m[2]),
				Type:        "code",
			})
		}
		// Extract a summary from non-numbered content
		summary := extractSummaryFromText(content, numberedPattern)
		return &StructuredPlan{
			Goal:    goal,
			Summary: summary,
			Steps:   steps,
		}
	}

	// Last resort: use content as a single step, but clean it up
	desc := cleanFallbackDescription(content)
	return &StructuredPlan{
		Goal:    goal,
		Summary: "Generated from unstructured response",
		Steps: []PlanStep{
			{
				ID:          "step1",
				Description: desc,
				Type:        "code",
			},
		},
	}
}

// looksLikeJSON returns true if the content appears to be JSON
func looksLikeJSON(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}

// extractStepsFromJSON tries to pull steps from a loosely-typed JSON map
func extractStepsFromJSON(raw map[string]interface{}) []PlanStep {
	stepsRaw, ok := raw["steps"].([]interface{})
	if !ok {
		return nil
	}
	var steps []PlanStep
	for i, s := range stepsRaw {
		switch v := s.(type) {
		case map[string]interface{}:
			desc, _ := v["description"].(string)
			if desc == "" {
				desc, _ = v["name"].(string)
			}
			if desc == "" {
				continue
			}
			stepType, _ := v["type"].(string)
			if stepType == "" {
				stepType = "code"
			}
			steps = append(steps, PlanStep{
				ID:          fmt.Sprintf("step%d", i+1),
				Description: desc,
				Type:        stepType,
			})
		case string:
			steps = append(steps, PlanStep{
				ID:          fmt.Sprintf("step%d", i+1),
				Description: v,
				Type:        "code",
			})
		}
	}
	return steps
}

// extractSummaryFromText extracts non-numbered text as a summary
func extractSummaryFromText(content string, numberedPattern *regexp.Regexp) string {
	cleaned := numberedPattern.ReplaceAllString(content, "")
	cleaned = strings.TrimSpace(cleaned)
	lines := strings.Split(cleaned, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && len(line) > 10 {
			return truncateDescription(line, 200)
		}
	}
	return "Plan extracted from text response"
}

// cleanFallbackDescription cleans up raw content for use as a step description
func cleanFallbackDescription(content string) string {
	// If it looks like JSON, try to extract just the meaningful text
	if looksLikeJSON(content) {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(content), &raw); err == nil {
			// Try common fields
			for _, key := range []string{"summary", "goal", "description", "plan"} {
				if v, ok := raw[key].(string); ok && v != "" {
					return truncateDescription(v, 200)
				}
			}
		}
	}
	return truncateDescription(content, 200)
}

// truncateDescription truncates a description to maxLen runes, adding ellipsis
func truncateDescription(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen-3]) + "..."
}

// FormatPlan returns a human-readable representation of the plan
func (p *PlannerAgent) FormatPlan(plan *StructuredPlan) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Plan: %s\n\n", plan.Goal))
	sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", plan.Summary))

	sb.WriteString("### Steps\n")
	for i, step := range plan.Steps {
		status := "[ ]"
		if step.Done {
			status = "[x]"
		}
		desc := truncateDescription(step.Description, 200)
		sb.WriteString(fmt.Sprintf("%d. %s **%s** (%s)\n", i+1, status, desc, step.Type))
		if len(step.Files) > 0 {
			sb.WriteString(fmt.Sprintf("   Files: %s\n", strings.Join(step.Files, ", ")))
		}
		if len(step.Dependencies) > 0 {
			sb.WriteString(fmt.Sprintf("   Depends on: %s\n", strings.Join(step.Dependencies, ", ")))
		}
	}

	if len(plan.Risks) > 0 {
		sb.WriteString("\n### Risks\n")
		for _, risk := range plan.Risks {
			sb.WriteString(fmt.Sprintf("- %s\n", risk))
		}
	}

	if len(plan.Assumptions) > 0 {
		sb.WriteString("\n### Assumptions\n")
		for _, assumption := range plan.Assumptions {
			sb.WriteString(fmt.Sprintf("- %s\n", assumption))
		}
	}

	return sb.String()
}
