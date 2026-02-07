package agent

import (
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/config"
)

func TestNewTierSelector(t *testing.T) {
	ts := NewTierSelector()
	if ts == nil {
		t.Fatal("expected non-nil TierSelector")
	}
}

func TestSelectTier_ComplexTriggers(t *testing.T) {
	ts := NewTierSelector()

	tests := []struct {
		name  string
		query string
	}{
		{"review", "review the authentication module"},
		{"analyze", "analyze the performance bottleneck"},
		{"architecture", "describe the architecture"},
		{"refactor", "refactor the database layer"},
		{"redesign", "redesign the API endpoints"},
		{"optimize", "optimize the query performance"},
		{"explain how", "explain how the middleware works"},
		{"why does", "why does the test fail"},
		{"what's wrong with", "what's wrong with this code"},
		{"debug", "debug the login issue"},
		{"fix the bug", "fix the bug in the parser"},
		{"implement", "implement a caching layer"},
		{"design", "design a new data model"},
		{"plan", "plan the migration strategy"},
		{"security", "check for security vulnerabilities"},
		{"performance", "improve performance of the API"},
		{"compare", "compare the two approaches"},
		{"tradeoff", "discuss the tradeoff between speed and memory"},
		{"trade-off", "what is the trade-off here"},
		{"best practice", "follow best practice for error handling"},
		{"code review", "do a code review"},
		{"comprehensive", "provide a comprehensive analysis"},
		{"thorough", "give a thorough explanation"},
		{"detailed analysis", "perform a detailed analysis"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ts.SelectTier(tt.query, config.TierSmart)
			if got != config.TierGenius {
				t.Errorf("SelectTier(%q) = %q, want %q", tt.query, got, config.TierGenius)
			}
		})
	}
}

func TestSelectTier_SimpleTriggers(t *testing.T) {
	ts := NewTierSelector()

	tests := []struct {
		name  string
		query string
	}{
		{"where is", "where is the main function"},
		{"find", "find the config file"},
		{"show me", "show me the error handler"},
		{"list", "list all the routes"},
		{"what file", "what file has the database code"},
		{"which file", "which file contains the router"},
		{"locate", "locate the test files"},
		{"search for", "search for the function signature"},
		{"look for", "look for the import statement"},
		{"open", "open the readme"},
		{"read", "read the configuration"},
		{"get", "get the API endpoint definition"},
		{"what is the path", "what is the path to the config"},
		{"where can i find", "where can i find the templates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ts.SelectTier(tt.query, config.TierSmart)
			if got != config.TierFast {
				t.Errorf("SelectTier(%q) = %q, want %q", tt.query, got, config.TierFast)
			}
		})
	}
}

func TestSelectTier_DefaultsToSmart(t *testing.T) {
	ts := NewTierSelector()

	// A query with no recognized triggers should return TierSmart
	queries := []string{
		"hello world",
		"thanks",
		"make it better",
		"do something",
	}

	for _, q := range queries {
		got := ts.SelectTier(q, config.TierFast)
		if got != config.TierSmart {
			t.Errorf("SelectTier(%q) = %q, want %q (default)", q, got, config.TierSmart)
		}
	}
}

func TestSelectTier_CaseInsensitive(t *testing.T) {
	ts := NewTierSelector()

	// Complex triggers should work regardless of case
	queries := []string{
		"REVIEW the code",
		"Review The Code",
		"review the code",
	}

	for _, q := range queries {
		got := ts.SelectTier(q, config.TierSmart)
		if got != config.TierGenius {
			t.Errorf("SelectTier(%q) = %q, want %q (case insensitive)", q, got, config.TierGenius)
		}
	}
}

func TestSelectTier_ComplexTakesPriority(t *testing.T) {
	ts := NewTierSelector()

	// A query containing both complex and simple triggers should prefer complex (genius)
	query := "review and find all security issues"
	got := ts.SelectTier(query, config.TierSmart)
	if got != config.TierGenius {
		t.Errorf("SelectTier(%q) = %q, want %q (complex takes priority)", query, got, config.TierGenius)
	}
}

func TestGetTierReason_ComplexQuery(t *testing.T) {
	ts := NewTierSelector()

	reason := ts.GetTierReason("review the authentication code")
	if !contains(reason, "complex query detected") {
		t.Errorf("expected complex reason, got %q", reason)
	}
	if !contains(reason, "review") {
		t.Errorf("expected reason to mention trigger, got %q", reason)
	}
}

func TestGetTierReason_SimpleQuery(t *testing.T) {
	ts := NewTierSelector()

	reason := ts.GetTierReason("find the main.go file")
	if !contains(reason, "simple query detected") {
		t.Errorf("expected simple reason, got %q", reason)
	}
	if !contains(reason, "find") {
		t.Errorf("expected reason to mention trigger, got %q", reason)
	}
}

func TestGetTierReason_DefaultQuery(t *testing.T) {
	ts := NewTierSelector()

	reason := ts.GetTierReason("hello there")
	if reason != "default tier selection" {
		t.Errorf("expected default reason, got %q", reason)
	}
}

func TestSelectTier_DefaultTierNotUsed(t *testing.T) {
	ts := NewTierSelector()

	// The defaultTier parameter is accepted but not used in the current implementation.
	// When no triggers match, TierSmart is always returned regardless of defaultTier.
	got := ts.SelectTier("hello", config.TierGenius)
	if got != config.TierSmart {
		t.Errorf("SelectTier with no triggers = %q, want %q", got, config.TierSmart)
	}
}
