package llm

import (
	"context"
	"math/rand"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	vecerr "github.com/abdul-hamid-achik/vecai/internal/errors"
)

// ResilientClient wraps an LLMClient with retry logic and circuit breaking.
type ResilientClient struct {
	inner      LLMClient
	cb         *CircuitBreaker
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// NewResilientClient wraps the given client with resilience features.
func NewResilientClient(inner LLMClient, cfg config.RateLimitConfig) *ResilientClient {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	baseDelay := cfg.BaseDelay
	if baseDelay <= 0 {
		baseDelay = 1 * time.Second
	}
	maxDelay := cfg.MaxDelay
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}
	return &ResilientClient{
		inner:      inner,
		cb:         NewCircuitBreaker(5, 30*time.Second),
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		maxDelay:   maxDelay,
	}
}

// Chat sends a request with retry and circuit breaker protection.
func (rc *ResilientClient) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error) {
	var lastErr error
	for attempt := 0; attempt <= rc.maxRetries; attempt++ {
		if !rc.cb.Allow() {
			return nil, vecerr.LLMUnavailable(ErrOllamaUnavailable)
		}

		resp, err := rc.inner.Chat(ctx, messages, tools, systemPrompt)
		if err == nil {
			rc.cb.RecordSuccess()
			return resp, nil
		}

		lastErr = err

		// Don't retry non-retryable errors
		if !vecerr.IsRetryable(err) {
			rc.cb.RecordFailure()
			return nil, err
		}

		rc.cb.RecordFailure()

		// Don't retry on last attempt or context cancellation
		if attempt == rc.maxRetries || ctx.Err() != nil {
			break
		}

		// Wait with exponential backoff + jitter
		delay := rc.backoff(attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

// ChatStream streams with circuit breaker protection (no retry for streams).
func (rc *ResilientClient) ChatStream(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) <-chan StreamChunk {
	if !rc.cb.Allow() {
		ch := make(chan StreamChunk, 1)
		go func() {
			defer close(ch)
			ch <- StreamChunk{Type: "error", Error: vecerr.LLMUnavailable(ErrOllamaUnavailable)}
		}()
		return ch
	}

	innerCh := rc.inner.ChatStream(ctx, messages, tools, systemPrompt)

	// Wrap to track success/failure for circuit breaker
	outCh := make(chan StreamChunk, 100)
	go func() {
		defer close(outCh)
		hadError := false
		for chunk := range innerCh {
			if chunk.Type == "error" {
				hadError = true
			}
			if chunk.Type == "done" && !hadError {
				rc.cb.RecordSuccess()
			}
			outCh <- chunk
		}
		if hadError {
			rc.cb.RecordFailure()
		}
	}()
	return outCh
}

// SetModel delegates to the inner client.
func (rc *ResilientClient) SetModel(model string) {
	rc.inner.SetModel(model)
}

// SetTier delegates to the inner client.
func (rc *ResilientClient) SetTier(tier config.ModelTier) {
	rc.inner.SetTier(tier)
}

// GetModel delegates to the inner client.
func (rc *ResilientClient) GetModel() string {
	return rc.inner.GetModel()
}

// Close delegates to the inner client.
func (rc *ResilientClient) Close() error {
	return rc.inner.Close()
}

// backoff calculates the delay for the given attempt using exponential backoff with jitter.
func (rc *ResilientClient) backoff(attempt int) time.Duration {
	delay := rc.baseDelay * (1 << uint(attempt))
	if delay > rc.maxDelay {
		delay = rc.maxDelay
	}
	// Add jitter: 50-100% of calculated delay
	jitter := time.Duration(rand.Int63n(int64(delay / 2)))
	return delay/2 + jitter
}
