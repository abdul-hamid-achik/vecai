package agent

import (
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/skills"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
	"github.com/abdul-hamid-achik/vecai/internal/ui"
)

// newTestAgent creates an Agent wired with mock dependencies for testing.
func newTestAgent(t *testing.T) (*Agent, *llm.MockLLMClient) {
	t.Helper()

	mock := llm.NewMockLLMClient()
	cfg := config.DefaultConfig()
	// Disable memory to avoid filesystem side effects in tests.
	cfg.Memory.Enabled = false

	registry := tools.NewRegistry(&cfg.Tools)
	output := ui.NewOutputHandler()
	input := ui.NewInputHandler()
	policy := permissions.NewPolicy(permissions.ModeAuto, input, output)
	skillLoader := skills.NewLoader()

	a := New(Config{
		LLM:         mock,
		Tools:       registry,
		Permissions: policy,
		Skills:      skillLoader,
		Output:      output,
		Input:       input,
		Config:      cfg,
	})

	return a, mock
}
