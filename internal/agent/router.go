package agent

import (
	"context"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/debug"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
)

// Intent represents the classified user intent
type Intent string

const (
	IntentPlan     Intent = "plan"     // Complex goals needing breakdown
	IntentCode     Intent = "code"     // Code writing/modification
	IntentReview   Intent = "review"   // Code quality review
	IntentQuestion Intent = "question" // Understanding code
	IntentDebug    Intent = "debug"    // Debugging issues
	IntentSimple   Intent = "simple"   // Simple tasks that don't need multi-agent
)

// TaskRouter classifies user intent to route to appropriate agent
type TaskRouter struct {
	client llm.LLMClient
	config *config.Config
}

// NewTaskRouter creates a new task router
func NewTaskRouter(client llm.LLMClient, cfg *config.Config) *TaskRouter {
	return &TaskRouter{
		client: client,
		config: cfg,
	}
}

// ClassifyIntent determines the appropriate intent for a user query
func (r *TaskRouter) ClassifyIntent(ctx context.Context, query string) Intent {
	// First try fast keyword-based classification
	intent := r.classifyByKeywords(query)
	if intent != "" {
		logDebug("Router: classified by keywords as %s", intent)
		debug.IntentClassified(query, string(intent), "keywords")
		return intent
	}

	// For ambiguous cases, use the LLM (fast model)
	intent = r.classifyByLLM(ctx, query)
	logDebug("Router: classified by LLM as %s", intent)
	debug.IntentClassified(query, string(intent), "llm")
	return intent
}

// classifyByKeywords uses simple pattern matching for common cases
func (r *TaskRouter) classifyByKeywords(query string) Intent {
	lower := strings.ToLower(query)

	// Plan indicators (complex multi-step tasks)
	planKeywords := []string{
		"implement", "create", "build", "develop", "design",
		"refactor", "restructure", "migrate", "convert",
		"feature", "system", "architecture",
	}
	for _, kw := range planKeywords {
		if strings.Contains(lower, kw) && len(query) > 50 {
			return IntentPlan
		}
	}

	// Code writing indicators
	codeKeywords := []string{
		"write", "add", "modify", "change", "update",
		"function", "method", "class", "struct", "type",
		"fix bug", "fix the", "patch",
	}
	for _, kw := range codeKeywords {
		if strings.Contains(lower, kw) {
			// If it's a simple single-file change, use code
			if !containsMultipleFileIndicators(lower) {
				return IntentCode
			}
			return IntentPlan
		}
	}

	// Review indicators
	reviewKeywords := []string{
		"review", "audit", "check", "analyze code",
		"code quality", "best practices", "security",
		"performance", "optimize",
	}
	for _, kw := range reviewKeywords {
		if strings.Contains(lower, kw) {
			return IntentReview
		}
	}

	// Debug indicators
	debugKeywords := []string{
		"debug", "why", "not working", "error", "crash",
		"failing", "broken", "issue", "problem",
		"doesn't work", "doesn't compile",
	}
	for _, kw := range debugKeywords {
		if strings.Contains(lower, kw) {
			return IntentDebug
		}
	}

	// Question indicators
	questionKeywords := []string{
		"what is", "what does", "how does", "where is",
		"explain", "describe", "show me", "find",
		"understand", "documentation", "help me understand",
		"?",
	}
	for _, kw := range questionKeywords {
		if strings.Contains(lower, kw) {
			return IntentQuestion
		}
	}

	return "" // Ambiguous, needs LLM classification
}

// containsMultipleFileIndicators checks if query suggests multi-file changes
func containsMultipleFileIndicators(query string) bool {
	indicators := []string{
		"multiple files", "several files", "across the",
		"throughout", "all files", "entire codebase",
		"refactor", "migrate", "restructure",
	}
	for _, ind := range indicators {
		if strings.Contains(query, ind) {
			return true
		}
	}
	return false
}

// classifyByLLM uses the LLM to classify ambiguous queries
func (r *TaskRouter) classifyByLLM(ctx context.Context, query string) Intent {
	// Switch to fast model for classification
	originalModel := r.client.GetModel()
	r.client.SetTier(config.TierFast)
	defer r.client.SetModel(originalModel)

	systemPrompt := `You are a task classifier. Classify the user's intent into exactly one category:

- plan: Complex tasks requiring multiple steps, features, or architectural changes
- code: Writing or modifying code in a single file or small scope
- review: Analyzing code quality, security, or performance
- question: Asking about how code works or finding information
- debug: Investigating errors, bugs, or unexpected behavior
- simple: Very simple tasks like greetings, thanks, or basic info

Respond with ONLY the category name, nothing else.`

	messages := []llm.Message{
		{Role: "user", Content: query},
	}

	resp, err := r.client.Chat(ctx, messages, nil, systemPrompt)
	if err != nil {
		logWarn("Router: LLM classification failed: %v, defaulting to simple", err)
		return IntentSimple
	}

	// Parse response
	content := strings.TrimSpace(strings.ToLower(resp.Content))
	switch content {
	case "plan":
		return IntentPlan
	case "code":
		return IntentCode
	case "review":
		return IntentReview
	case "question":
		return IntentQuestion
	case "debug":
		return IntentDebug
	default:
		return IntentSimple
	}
}

// ShouldUseMultiAgent determines if a task should use multi-agent flow
func (r *TaskRouter) ShouldUseMultiAgent(intent Intent) bool {
	switch intent {
	case IntentPlan, IntentReview:
		return true
	case IntentCode:
		// Code tasks might benefit from verification but not full multi-agent
		return false
	default:
		return false
	}
}

// GetRecommendedTier returns the recommended model tier for an intent
func (r *TaskRouter) GetRecommendedTier(intent Intent) config.ModelTier {
	switch intent {
	case IntentPlan, IntentReview:
		return config.TierGenius // Complex reasoning
	case IntentCode:
		return config.TierSmart // Code-focused
	case IntentDebug:
		return config.TierSmart // Needs good reasoning
	case IntentQuestion:
		return config.TierFast // Quick answers
	default:
		return config.TierFast
	}
}
