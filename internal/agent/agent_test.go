package agent

import (
	"context"
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/permissions"
	"github.com/abdul-hamid-achik/vecai/internal/tui"
)

func TestNew(t *testing.T) {
	a, mock := newTestAgent(t)
	if a == nil {
		t.Fatal("expected non-nil agent")
	}
	if mock == nil {
		t.Fatal("expected non-nil mock client")
	}
	if a.llm == nil {
		t.Error("agent LLM client should not be nil")
	}
}

func TestRunQuick(t *testing.T) {
	a, mock := newTestAgent(t)

	called := false
	mock.ChatFunc = func(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition, systemPrompt string) (*llm.Response, error) {
		called = true
		if len(tools) != 0 {
			t.Errorf("RunQuick should pass nil/empty tools, got %d", len(tools))
		}
		if len(messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(messages))
		}
		if messages[0].Role != "user" {
			t.Errorf("expected user role, got %q", messages[0].Role)
		}
		if messages[0].Content != "hello" {
			t.Errorf("expected content %q, got %q", "hello", messages[0].Content)
		}
		return &llm.Response{Content: "quick reply"}, nil
	}

	err := a.RunQuick("hello")
	if err != nil {
		t.Fatalf("RunQuick returned error: %v", err)
	}
	if !called {
		t.Error("Chat was not called")
	}
}

func TestGetSystemPrompt(t *testing.T) {
	a, _ := newTestAgent(t)

	prompt := a.getSystemPrompt()
	if prompt == "" {
		t.Error("system prompt should not be empty")
	}

	// The default (non-analysis) prompt should contain "vecai"
	if !contains(prompt, "vecai") {
		t.Error("system prompt should mention vecai")
	}
}

func TestGetSystemPromptAnalysisMode(t *testing.T) {
	a, _ := newTestAgent(t)
	a.analysisMode = true

	prompt := a.getSystemPrompt()
	if prompt == "" {
		t.Error("analysis system prompt should not be empty")
	}

	// Analysis prompt should contain "analysis"
	if !contains(prompt, "analysis") {
		t.Errorf("analysis prompt should mention analysis, got: %s", prompt[:80])
	}
}

func TestMockLLMClientDefaults(t *testing.T) {
	mock := llm.NewMockLLMClient()

	// Default Chat
	resp, err := mock.Chat(context.Background(), nil, nil, "")
	if err != nil {
		t.Fatalf("default Chat returned error: %v", err)
	}
	if resp.Content != "mock response" {
		t.Errorf("expected %q, got %q", "mock response", resp.Content)
	}

	// Default ChatStream
	ch := mock.ChatStream(context.Background(), nil, nil, "")
	var chunks []llm.StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Type != "text" || chunks[0].Text != "mock response" {
		t.Errorf("unexpected first chunk: %+v", chunks[0])
	}
	if chunks[1].Type != "done" {
		t.Errorf("unexpected second chunk: %+v", chunks[1])
	}

	// Call recording
	if len(mock.ChatCalls) != 1 {
		t.Errorf("expected 1 ChatCall recorded, got %d", len(mock.ChatCalls))
	}
	if len(mock.ChatStreamCalls) != 1 {
		t.Errorf("expected 1 ChatStreamCall recorded, got %d", len(mock.ChatStreamCalls))
	}
}

func TestMockLLMClientModel(t *testing.T) {
	mock := llm.NewMockLLMClient()

	if mock.GetModel() != "mock-model" {
		t.Errorf("expected default model %q, got %q", "mock-model", mock.GetModel())
	}

	mock.SetModel("custom-model")
	if mock.GetModel() != "custom-model" {
		t.Errorf("expected %q, got %q", "custom-model", mock.GetModel())
	}
}

func TestApplyModeChange(t *testing.T) {
	a, _ := newTestAgent(t)

	// Agent starts with zero-value agentMode (ModeAsk).
	// Set to Build so we can test switching away from it.
	a.agentMode = tui.ModeBuild

	// Switch to Ask — should save permissions and switch to ModeAnalysis
	a.applyModeChange(tui.ModeAsk, false)
	if a.agentMode != tui.ModeAsk {
		t.Errorf("expected ModeAsk, got %v", a.agentMode)
	}
	if a.permissions.GetMode() != permissions.ModeAnalysis {
		t.Errorf("expected ModeAnalysis permission, got %v", a.permissions.GetMode())
	}
	if a.previousPermMode == 0 {
		t.Error("previousPermMode should be saved when switching to Ask")
	}

	// Switch to Plan
	a.applyModeChange(tui.ModePlan, false)
	if a.agentMode != tui.ModePlan {
		t.Errorf("expected ModePlan, got %v", a.agentMode)
	}
	if a.permissions.GetMode() != permissions.ModeAsk {
		t.Errorf("expected ModeAsk permission for Plan mode, got %v", a.permissions.GetMode())
	}

	// Switch to Build — should restore original permissions
	a.applyModeChange(tui.ModeBuild, false)
	if a.agentMode != tui.ModeBuild {
		t.Errorf("expected ModeBuild, got %v", a.agentMode)
	}
	if a.previousPermMode != 0 {
		t.Error("previousPermMode should be cleared after restoring Build mode")
	}

	// No-op: switching to same mode shouldn't change anything
	a.agentMode = tui.ModeAsk
	a.applyModeChange(tui.ModeAsk, false)
	// Should remain ModeAsk with no side effects
	if a.agentMode != tui.ModeAsk {
		t.Errorf("expected no change, got %v", a.agentMode)
	}
}

func TestApplyModeChange_WithTierUpdate(t *testing.T) {
	a, mock := newTestAgent(t)
	a.autoTier = true
	a.quickMode = false
	a.agentMode = tui.ModeBuild // Start from Build

	// Switch to Ask with tier update
	a.applyModeChange(tui.ModeAsk, true)
	if mock.CurrentTier != config.TierFast {
		t.Errorf("expected TierFast for Ask mode, got %v", mock.CurrentTier)
	}

	// Switch to Plan with tier update
	a.applyModeChange(tui.ModePlan, true)
	if mock.CurrentTier != config.TierSmart {
		t.Errorf("expected TierSmart for Plan mode, got %v", mock.CurrentTier)
	}

	// Switch to Build with tier update
	a.applyModeChange(tui.ModeBuild, true)
	if mock.CurrentTier != config.TierSmart {
		t.Errorf("expected TierSmart for Build mode, got %v", mock.CurrentTier)
	}
}

func TestSelectTierForMode(t *testing.T) {
	a, _ := newTestAgent(t)

	// In Ask mode, simple queries should get Fast tier
	a.agentMode = tui.ModeAsk
	tier := a.selectTierForMode("where is the router?")
	if tier != config.TierFast {
		t.Errorf("Ask mode simple query: expected TierFast, got %v", tier)
	}

	// In Plan mode, simple queries should be floored to Smart
	a.agentMode = tui.ModePlan
	tier = a.selectTierForMode("where is the router?")
	if tier != config.TierSmart {
		t.Errorf("Plan mode simple query: expected TierSmart (floor), got %v", tier)
	}

	// In Build mode, simple queries should be floored to Smart
	a.agentMode = tui.ModeBuild
	tier = a.selectTierForMode("where is the router?")
	if tier != config.TierSmart {
		t.Errorf("Build mode simple query: expected TierSmart (floor), got %v", tier)
	}

	// In Ask mode, complex queries should still get Genius tier
	a.agentMode = tui.ModeAsk
	tier = a.selectTierForMode("review the architecture for security issues")
	if tier != config.TierGenius {
		t.Errorf("Ask mode complex query: expected TierGenius, got %v", tier)
	}

	// In Plan mode, complex queries should still get Genius tier (above floor)
	a.agentMode = tui.ModePlan
	tier = a.selectTierForMode("review the architecture for security issues")
	if tier != config.TierGenius {
		t.Errorf("Plan mode complex query: expected TierGenius, got %v", tier)
	}
}

// contains is a simple helper to avoid importing strings in tests.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
