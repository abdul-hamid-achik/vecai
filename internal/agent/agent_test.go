package agent

import (
	"context"
	"testing"

	"github.com/abdul-hamid-achik/vecai/internal/llm"
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

// contains is a simple helper to avoid importing strings in tests.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
