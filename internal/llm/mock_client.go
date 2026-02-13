package llm

import (
	"context"
	"sync"

	"github.com/abdul-hamid-achik/vecai/internal/config"
)

// MockLLMClient implements LLMClient for testing.
type MockLLMClient struct {
	// Injectable behavior
	ChatFunc       func(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error)
	ChatStreamFunc func(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) <-chan StreamChunk

	// State
	model string
	mu    sync.Mutex

	// CurrentTier records the last tier set via SetTier (for test assertions).
	CurrentTier config.ModelTier

	// Call recording
	ChatCalls       []ChatCall
	ChatStreamCalls []ChatStreamCall
}

// ChatCall records the arguments of a Chat invocation.
type ChatCall struct {
	Messages     []Message
	Tools        []ToolDefinition
	SystemPrompt string
}

// ChatStreamCall records the arguments of a ChatStream invocation.
type ChatStreamCall struct {
	Messages     []Message
	Tools        []ToolDefinition
	SystemPrompt string
}

// NewMockLLMClient creates a mock client with sensible defaults.
func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{
		model: "mock-model",
	}
}

// Chat calls the injected ChatFunc or returns a default response.
func (m *MockLLMClient) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error) {
	m.mu.Lock()
	m.ChatCalls = append(m.ChatCalls, ChatCall{
		Messages:     messages,
		Tools:        tools,
		SystemPrompt: systemPrompt,
	})
	m.mu.Unlock()

	if m.ChatFunc != nil {
		return m.ChatFunc(ctx, messages, tools, systemPrompt)
	}
	return &Response{
		Content:    "mock response",
		StopReason: "end_turn",
	}, nil
}

// ChatStream calls the injected ChatStreamFunc or returns a default stream.
func (m *MockLLMClient) ChatStream(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) <-chan StreamChunk {
	m.mu.Lock()
	m.ChatStreamCalls = append(m.ChatStreamCalls, ChatStreamCall{
		Messages:     messages,
		Tools:        tools,
		SystemPrompt: systemPrompt,
	})
	m.mu.Unlock()

	if m.ChatStreamFunc != nil {
		return m.ChatStreamFunc(ctx, messages, tools, systemPrompt)
	}

	ch := make(chan StreamChunk, 2)
	go func() {
		defer close(ch)
		ch <- StreamChunk{Type: "text", Text: "mock response"}
		ch <- StreamChunk{Type: "done"}
	}()
	return ch
}

// SetModel sets the model name.
func (m *MockLLMClient) SetModel(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.model = model
}

// SetTier records the tier for test assertions.
func (m *MockLLMClient) SetTier(tier config.ModelTier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CurrentTier = tier
}

// GetModel returns the current model name.
func (m *MockLLMClient) GetModel() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.model
}

// Fork returns a new MockLLMClient with the same injectable functions.
func (m *MockLLMClient) Fork() LLMClient {
	return &MockLLMClient{
		ChatFunc:       m.ChatFunc,
		ChatStreamFunc: m.ChatStreamFunc,
		model:          m.GetModel(),
	}
}

// Close is a no-op for the mock client.
func (m *MockLLMClient) Close() error {
	return nil
}
