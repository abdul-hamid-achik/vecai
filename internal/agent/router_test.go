package agent

import (
	"context"
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
)

func newTestRouter(t *testing.T) (*TaskRouter, *llm.MockLLMClient) {
	t.Helper()
	mock := llm.NewMockLLMClient()
	cfg := config.DefaultConfig()
	return NewTaskRouter(mock, cfg), mock
}

func TestNewTaskRouter(t *testing.T) {
	router, _ := newTestRouter(t)
	if router == nil {
		t.Fatal("expected non-nil router")
	}
	if router.client == nil {
		t.Error("router client should not be nil")
	}
	if router.config == nil {
		t.Error("router config should not be nil")
	}
}

func TestClassifyByKeywords(t *testing.T) {
	router, _ := newTestRouter(t)

	tests := []struct {
		name  string
		query string
		want  Intent
	}{
		// Plan intent
		{
			name:  "plan: implement a feature",
			query: "implement a new authentication system",
			want:  IntentPlan,
		},
		{
			name:  "plan: architecture design",
			query: "design the architecture for our microservices",
			want:  IntentPlan,
		},
		{
			name:  "plan: refactor keyword",
			query: "refactor the database layer",
			want:  IntentPlan,
		},
		{
			name:  "plan: migrate keyword",
			query: "migrate from REST to GraphQL",
			want:  IntentPlan,
		},

		// Code intent
		{
			name:  "code: write a function",
			query: "write a function to parse JSON",
			want:  IntentCode,
		},
		{
			name:  "code: add a method",
			query: "add a method to the User struct",
			want:  IntentCode,
		},
		{
			name:  "code: fix bug",
			query: "fix bug in the login handler",
			want:  IntentCode,
		},
		{
			name:  "code: modify and update",
			query: "modify the config and update the handler",
			want:  IntentCode,
		},

		// Review intent
		{
			name:  "review: review code",
			query: "review this code for best practices",
			want:  IntentReview,
		},
		{
			name:  "review: audit security",
			query: "audit the code for security issues",
			want:  IntentReview,
		},
		{
			name:  "review: analyze code quality",
			query: "analyze code quality and performance",
			want:  IntentReview,
		},

		// Debug intent
		{
			name:  "debug: not working",
			query: "the login page is not working",
			want:  IntentDebug,
		},
		{
			name:  "debug: error and crash",
			query: "getting an error crash when starting the server",
			want:  IntentDebug,
		},
		{
			name:  "debug: doesn't compile",
			query: "the code doesn't compile",
			want:  IntentDebug,
		},

		// Question intent
		{
			name:  "question: explain with question mark",
			query: "explain the middleware chain?",
			want:  IntentQuestion,
		},
		{
			name:  "question: what does with understand",
			query: "help me understand what does this code do",
			want:  IntentQuestion,
		},
		{
			name:  "question: where is plus find",
			query: "where is the config? find it for me",
			want:  IntentQuestion,
		},

		// Ambiguous / below threshold should return empty
		{
			name:  "ambiguous: single weak word",
			query: "hello",
			want:  "",
		},
		{
			name:  "ambiguous: generic request",
			query: "thanks for your help",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := router.classifyByKeywords(tt.query)
			if got != tt.want {
				t.Errorf("classifyByKeywords(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestClassifyByKeywords_CaseInsensitive(t *testing.T) {
	router, _ := newTestRouter(t)

	queries := []string{
		"IMPLEMENT a new feature",
		"Implement A New Feature",
		"implement a new feature",
	}

	for _, q := range queries {
		got := router.classifyByKeywords(q)
		if got != IntentPlan {
			t.Errorf("classifyByKeywords(%q) = %q, want %q (case insensitivity)", q, got, IntentPlan)
		}
	}
}

func TestClassifyByKeywords_MultiFileShiftsToPlan(t *testing.T) {
	router, _ := newTestRouter(t)

	// A code query mentioning multiple files should shift to plan
	query := "write new handlers across the entire codebase"
	got := router.classifyByKeywords(query)
	if got != IntentPlan {
		t.Errorf("classifyByKeywords(%q) = %q, want %q (multi-file shift to plan)", query, got, IntentPlan)
	}
}

func TestClassifyByKeywords_LongQueryPlanBonus(t *testing.T) {
	router, _ := newTestRouter(t)

	// A long query with a plan keyword should still classify as plan due to complexity bonus
	query := "I want to create a feature that handles user authentication across multiple services including OAuth support and session management with proper error handling"
	got := router.classifyByKeywords(query)
	if got != IntentPlan {
		t.Errorf("classifyByKeywords(%q) = %q, want %q (long query plan bonus)", query, got, IntentPlan)
	}
}

func TestClassifyIntent_FallsBackToLLM(t *testing.T) {
	router, mock := newTestRouter(t)

	// Mock the LLM to return "debug"
	mock.ChatFunc = func(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition, systemPrompt string) (*llm.Response, error) {
		return &llm.Response{Content: "debug"}, nil
	}

	// "hi" has no strong keywords, so it should fall through to LLM
	got := router.ClassifyIntent(context.Background(), "hi")
	if got != IntentDebug {
		t.Errorf("ClassifyIntent(\"hi\") = %q, want %q (LLM fallback)", got, IntentDebug)
	}

	if len(mock.ChatCalls) == 0 {
		t.Error("expected LLM Chat to be called for ambiguous query")
	}
}

func TestClassifyIntent_KeywordsSkipsLLM(t *testing.T) {
	router, mock := newTestRouter(t)

	mock.ChatFunc = func(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition, systemPrompt string) (*llm.Response, error) {
		t.Error("LLM should not be called when keywords match")
		return &llm.Response{Content: "simple"}, nil
	}

	got := router.ClassifyIntent(context.Background(), "explain the middleware chain")
	if got != IntentQuestion {
		t.Errorf("ClassifyIntent = %q, want %q", got, IntentQuestion)
	}
}

func TestClassifyByLLM_ParsesResponse(t *testing.T) {
	router, mock := newTestRouter(t)

	tests := []struct {
		llmResp string
		want    Intent
	}{
		{"plan", IntentPlan},
		{"code", IntentCode},
		{"review", IntentReview},
		{"question", IntentQuestion},
		{"debug", IntentDebug},
		{"  Plan  ", IntentPlan},    // whitespace and case
		{"unknown", IntentSimple},   // unrecognized defaults to simple
		{"", IntentSimple},          // empty defaults to simple
	}

	for _, tt := range tests {
		t.Run("llm_returns_"+tt.llmResp, func(t *testing.T) {
			mock.ChatFunc = func(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition, systemPrompt string) (*llm.Response, error) {
				return &llm.Response{Content: tt.llmResp}, nil
			}

			got := router.classifyByLLM(context.Background(), "test query")
			if got != tt.want {
				t.Errorf("classifyByLLM with response %q = %q, want %q", tt.llmResp, got, tt.want)
			}
		})
	}
}

func TestShouldUseMultiAgent(t *testing.T) {
	router, _ := newTestRouter(t)

	tests := []struct {
		intent Intent
		want   bool
	}{
		{IntentPlan, true},
		{IntentReview, true},
		{IntentCode, false},
		{IntentDebug, false},
		{IntentQuestion, false},
		{IntentSimple, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.intent), func(t *testing.T) {
			got := router.ShouldUseMultiAgent(tt.intent)
			if got != tt.want {
				t.Errorf("ShouldUseMultiAgent(%q) = %v, want %v", tt.intent, got, tt.want)
			}
		})
	}
}

func TestGetRecommendedTier(t *testing.T) {
	router, _ := newTestRouter(t)

	tests := []struct {
		intent Intent
		want   config.ModelTier
	}{
		{IntentPlan, config.TierGenius},
		{IntentReview, config.TierGenius},
		{IntentCode, config.TierSmart},
		{IntentDebug, config.TierSmart},
		{IntentQuestion, config.TierFast},
		{IntentSimple, config.TierFast},
	}

	for _, tt := range tests {
		t.Run(string(tt.intent), func(t *testing.T) {
			got := router.GetRecommendedTier(tt.intent)
			if got != tt.want {
				t.Errorf("GetRecommendedTier(%q) = %q, want %q", tt.intent, got, tt.want)
			}
		})
	}
}

func TestContainsMultipleFileIndicators(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"update multiple files in the project", true},
		{"refactor the codebase", true},
		{"migrate to new API", true},
		{"restructure the code", true},
		{"change across the entire codebase", true},
		{"update a single function", false},
		{"hello world", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := containsMultipleFileIndicators(tt.query)
			if got != tt.want {
				t.Errorf("containsMultipleFileIndicators(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}
