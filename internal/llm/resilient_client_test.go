package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	vecerr "github.com/abdul-hamid-achik/vecai/internal/errors"
)

func TestResilientClient_SuccessfulChat(t *testing.T) {
	mock := NewMockLLMClient()
	rc := NewResilientClient(mock, config.RateLimitConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
	})

	resp, err := rc.Chat(context.Background(), nil, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "mock response" {
		t.Errorf("expected 'mock response', got %q", resp.Content)
	}
	if len(mock.ChatCalls) != 1 {
		t.Errorf("expected 1 call, got %d", len(mock.ChatCalls))
	}
}

func TestResilientClient_RetriesOnRetryableError(t *testing.T) {
	mock := NewMockLLMClient()
	callCount := 0
	mock.ChatFunc = func(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error) {
		callCount++
		if callCount < 3 {
			return nil, vecerr.LLMRequestFailed(errors.New("temporary error"))
		}
		return &Response{Content: "recovered"}, nil
	}

	rc := NewResilientClient(mock, config.RateLimitConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
	})

	resp, err := rc.Chat(context.Background(), nil, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "recovered" {
		t.Errorf("expected 'recovered', got %q", resp.Content)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestResilientClient_DoesNotRetryNonRetryable(t *testing.T) {
	mock := NewMockLLMClient()
	callCount := 0
	mock.ChatFunc = func(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error) {
		callCount++
		return nil, vecerr.LLMModelNotFound("nonexistent-model")
	}

	rc := NewResilientClient(mock, config.RateLimitConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
	})

	_, err := rc.Chat(context.Background(), nil, nil, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (no retry), got %d", callCount)
	}
}

func TestResilientClient_CircuitBreakerOpens(t *testing.T) {
	mock := NewMockLLMClient()
	mock.ChatFunc = func(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error) {
		return nil, vecerr.LLMRequestFailed(errors.New("always fail"))
	}

	rc := NewResilientClient(mock, config.RateLimitConfig{
		MaxRetries: 0, // No retries, just record failures
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
	})

	// Exhaust the circuit breaker (5 failures to open)
	for i := 0; i < 5; i++ {
		_, _ = rc.Chat(context.Background(), nil, nil, "")
	}

	// Next call should be rejected by circuit breaker
	_, err := rc.Chat(context.Background(), nil, nil, "")
	if err == nil {
		t.Fatal("expected error from open circuit")
	}
}

func TestResilientClient_ContextCancellation(t *testing.T) {
	mock := NewMockLLMClient()
	mock.ChatFunc = func(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error) {
		return nil, vecerr.LLMRequestFailed(errors.New("fail"))
	}

	rc := NewResilientClient(mock, config.RateLimitConfig{
		MaxRetries: 5,
		BaseDelay:  1 * time.Second,
		MaxDelay:   5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := rc.Chat(ctx, nil, nil, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResilientClient_ChatStream(t *testing.T) {
	mock := NewMockLLMClient()
	rc := NewResilientClient(mock, config.RateLimitConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
	})

	ch := rc.ChatStream(context.Background(), nil, nil, "")
	var chunks []StreamChunk
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
}

func TestResilientClient_DelegatesModelOps(t *testing.T) {
	mock := NewMockLLMClient()
	rc := NewResilientClient(mock, config.RateLimitConfig{})

	rc.SetModel("test-model")
	if rc.GetModel() != "test-model" {
		t.Errorf("expected 'test-model', got %q", rc.GetModel())
	}

	if err := rc.Close(); err != nil {
		t.Errorf("unexpected close error: %v", err)
	}
}

func TestResilientClient_DefaultConfig(t *testing.T) {
	mock := NewMockLLMClient()
	rc := NewResilientClient(mock, config.RateLimitConfig{})
	if rc.maxRetries != 3 {
		t.Errorf("expected default maxRetries 3, got %d", rc.maxRetries)
	}
	if rc.baseDelay != 1*time.Second {
		t.Errorf("expected default baseDelay 1s, got %v", rc.baseDelay)
	}
	if rc.maxDelay != 30*time.Second {
		t.Errorf("expected default maxDelay 30s, got %v", rc.maxDelay)
	}
}

func TestResilientClient_BackoffCalculation(t *testing.T) {
	rc := &ResilientClient{
		baseDelay: 100 * time.Millisecond,
		maxDelay:  1 * time.Second,
	}

	// Attempt 0: base delay (50-100ms with jitter)
	d0 := rc.backoff(0)
	if d0 < 50*time.Millisecond || d0 > 100*time.Millisecond {
		t.Errorf("attempt 0 backoff out of range: %v", d0)
	}

	// Attempt 3: should be capped at maxDelay
	d3 := rc.backoff(3)
	if d3 > 1*time.Second {
		t.Errorf("attempt 3 backoff should be capped at 1s, got %v", d3)
	}
}
