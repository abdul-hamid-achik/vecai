package agent

import (
	"strings"

	"github.com/abdul-hamid-achik/vecai/internal/config"
)

// TierSelector automatically selects the appropriate model tier based on query complexity
type TierSelector struct{}

// NewTierSelector creates a new tier selector
func NewTierSelector() *TierSelector {
	return &TierSelector{}
}

// simpleTriggers are patterns that indicate a simple query suitable for fast tier
var simpleTriggers = []string{
	"where is",
	"find",
	"show me",
	"list",
	"what file",
	"which file",
	"locate",
	"search for",
	"look for",
	"open",
	"read",
	"get",
	"what is the path",
	"where can i find",
}

// complexTriggers are patterns that indicate a complex query needing genius tier
var complexTriggers = []string{
	"review",
	"analyze",
	"architecture",
	"refactor",
	"redesign",
	"optimize",
	"explain how",
	"why does",
	"what's wrong with",
	"debug",
	"fix the bug",
	"implement",
	"design",
	"plan",
	"strategy",
	"security",
	"performance",
	"compare",
	"tradeoff",
	"trade-off",
	"best practice",
	"code review",
	"comprehensive",
	"thorough",
	"detailed analysis",
}

// SelectTier chooses the appropriate model tier based on query content
func (ts *TierSelector) SelectTier(query string, defaultTier config.ModelTier) config.ModelTier {
	queryLower := strings.ToLower(query)

	// Check for complex triggers first (higher priority)
	for _, trigger := range complexTriggers {
		if strings.Contains(queryLower, trigger) {
			return config.TierGenius
		}
	}

	// Check for simple triggers
	for _, trigger := range simpleTriggers {
		if strings.Contains(queryLower, trigger) {
			return config.TierFast
		}
	}

	// Default to smart tier for balanced performance
	return config.TierSmart
}

// GetTierReason returns a human-readable reason for the tier selection
func (ts *TierSelector) GetTierReason(query string) string {
	queryLower := strings.ToLower(query)

	for _, trigger := range complexTriggers {
		if strings.Contains(queryLower, trigger) {
			return "complex query detected (\"" + trigger + "\")"
		}
	}

	for _, trigger := range simpleTriggers {
		if strings.Contains(queryLower, trigger) {
			return "simple query detected (\"" + trigger + "\")"
		}
	}

	return "default tier selection"
}
