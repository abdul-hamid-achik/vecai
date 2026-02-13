package agent

import (
	"context"
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/debug"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/tui"
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

// keywordScore pairs a keyword with a weight for intent classification
type keywordScore struct {
	keyword string
	weight  float64
}

// intentKeywords maps each intent to weighted keywords for scoring
var intentKeywords = map[Intent][]keywordScore{
	IntentPlan: {
		{"implement", 2.0}, {"create", 1.5}, {"build", 1.5}, {"develop", 1.5}, {"design", 2.0},
		{"refactor", 2.0}, {"restructure", 2.0}, {"migrate", 2.0}, {"convert", 1.5},
		{"feature", 1.5}, {"system", 1.0}, {"architecture", 2.0},
	},
	IntentCode: {
		{"write", 2.0}, {"add", 1.5}, {"modify", 1.5}, {"change", 1.5}, {"update", 1.5},
		{"function", 1.0}, {"method", 1.0}, {"class", 1.0}, {"struct", 1.0}, {"type", 0.5},
		{"fix bug", 2.0}, {"fix the", 1.5}, {"patch", 1.5},
	},
	IntentReview: {
		{"review", 2.0}, {"audit", 2.0}, {"check", 1.0}, {"analyze code", 2.0},
		{"code quality", 2.0}, {"best practices", 1.5}, {"security", 1.5},
		{"performance", 1.5}, {"optimize", 1.5},
	},
	IntentDebug: {
		{"debug", 2.0}, {"why", 1.0}, {"not working", 2.0}, {"error", 1.5}, {"crash", 2.0},
		{"failing", 1.5}, {"broken", 1.5}, {"issue", 1.0}, {"problem", 1.0},
		{"doesn't work", 2.0}, {"doesn't compile", 2.0},
	},
	IntentQuestion: {
		{"what is", 1.5}, {"what does", 1.5}, {"how does", 1.5}, {"where is", 1.5},
		{"explain", 2.0}, {"describe", 1.5}, {"show me", 1.0}, {"find", 1.0},
		{"understand", 1.5}, {"documentation", 1.5}, {"help me understand", 2.0},
		{"?", 1.0},
	},
}

// classifyByKeywords uses weighted keyword scoring for intent classification
func (r *TaskRouter) classifyByKeywords(query string) Intent {
	lower := strings.ToLower(query)
	const scoreThreshold = 2.0

	// Score each intent
	scores := make(map[Intent]float64)
	for intent, keywords := range intentKeywords {
		var total float64
		for _, ks := range keywords {
			if strings.Contains(lower, ks.keyword) {
				total += ks.weight
			}
		}
		scores[intent] = total
	}

	// Add complexity bonus to IntentPlan for longer queries (word count > 10)
	wordCount := len(strings.Fields(query))
	if wordCount > 10 {
		scores[IntentPlan] += 1.5
	}

	// For code intent with multi-file indicators, shift score to plan
	if scores[IntentCode] > 0 && containsMultipleFileIndicators(lower) {
		scores[IntentPlan] += scores[IntentCode]
		scores[IntentCode] = 0
	}

	// Find the highest scoring intent
	var bestIntent Intent
	var bestScore float64
	for intent, score := range scores {
		if score > bestScore {
			bestScore = score
			bestIntent = intent
		}
	}

	// Only return if score exceeds threshold
	if bestScore >= scoreThreshold {
		return bestIntent
	}

	return "" // Falls through to LLM classification
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

	// Strip <think>...</think> tags that some models wrap reasoning in
	content := stripThinkTags(resp.Content)
	content = strings.TrimSpace(strings.ToLower(content))
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

// GetRecommendedMode returns the recommended agent mode for an intent.
// Returns the mode and whether a switch should happen (false for IntentSimple).
func (r *TaskRouter) GetRecommendedMode(intent Intent) (tui.AgentMode, bool) {
	switch intent {
	case IntentQuestion:
		return tui.ModeAsk, true
	case IntentPlan, IntentReview:
		return tui.ModePlan, true
	case IntentCode, IntentDebug:
		return tui.ModeBuild, true
	default:
		return 0, false // IntentSimple: don't switch
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
