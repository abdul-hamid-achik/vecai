package llm

import (
	"context"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/config"
	"github.com/abdul-hamid-achik/vecai/internal/logger"
	"golang.org/x/time/rate"
)

// TokenEstimator estimates token counts for rate limiting
type TokenEstimator struct{}

// NewTokenEstimator creates a new token estimator
func NewTokenEstimator() *TokenEstimator {
	return &TokenEstimator{}
}

// EstimateTokens estimates the number of tokens in a string
// Uses a rough approximation: chars/4 + 20% buffer
func (e *TokenEstimator) EstimateTokens(text string) int {
	// Rough estimation: ~4 characters per token on average
	baseEstimate := len(text) / 4
	// Add 20% buffer for safety
	return int(float64(baseEstimate) * 1.2)
}

// EstimateMessages estimates tokens for a slice of messages
func (e *TokenEstimator) EstimateMessages(messages []Message) int {
	total := 0
	for _, msg := range messages {
		// Add overhead for message structure (~4 tokens per message)
		total += 4
		total += e.EstimateTokens(msg.Content)
	}
	return total
}

// WaitInfo contains information about a rate limit wait
type WaitInfo struct {
	Duration    time.Duration // How long to wait
	Reason      string        // Why we're waiting (e.g., "token bucket cooldown" or "API returned 429")
	Attempt     int           // Current attempt number (1-based, 0 if not a retry)
	MaxAttempts int           // Maximum number of attempts (0 if not a retry)
}

// WaitCallback is called when the client needs to wait due to rate limiting.
// It should block for the specified duration or until context is cancelled.
// If nil, the default time.After behavior is used.
type WaitCallback func(ctx context.Context, info WaitInfo) error

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	limiter  *rate.Limiter
	mu       sync.Mutex
	onWait   WaitCallback
}

// NewTokenBucket creates a new token bucket rate limiter
// tokensPerMinute is converted to tokens per second for the limiter
func NewTokenBucket(tokensPerMinute int) *TokenBucket {
	// Convert tokens/minute to tokens/second
	tokensPerSecond := float64(tokensPerMinute) / 60.0
	// Burst size allows for some flexibility (10 seconds worth of tokens)
	burstSize := tokensPerMinute / 6
	if burstSize < 1000 {
		burstSize = 1000
	}

	return &TokenBucket{
		limiter: rate.NewLimiter(rate.Limit(tokensPerSecond), burstSize),
	}
}

// SetWaitCallback sets a callback to be invoked when waiting for tokens
func (tb *TokenBucket) SetWaitCallback(cb WaitCallback) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.onWait = cb
}

// Wait blocks until the specified number of tokens are available
func (tb *TokenBucket) Wait(ctx context.Context, tokens int) error {
	tb.mu.Lock()
	onWait := tb.onWait
	tb.mu.Unlock()

	// Reserve tokens
	reservation := tb.limiter.ReserveN(time.Now(), tokens)
	if !reservation.OK() {
		// If we can't reserve, wait for the full delay
		logger.Debug("Rate limit: tokens exceed burst size, waiting for availability")
	}

	delay := reservation.Delay()
	if delay > 0 {
		logger.Debug("Rate limit: waiting %v for %d tokens", delay, tokens)

		// Use callback if available
		if onWait != nil {
			err := onWait(ctx, WaitInfo{
				Duration: delay,
				Reason:   "token bucket cooldown",
			})
			if err != nil {
				reservation.Cancel()
				return err
			}
			return nil
		}

		// Default behavior: simple time.After
		select {
		case <-time.After(delay):
			return nil
		case <-ctx.Done():
			reservation.Cancel()
			return ctx.Err()
		}
	}

	return nil
}

// RateLimitedClient wraps a Client with rate limiting
type RateLimitedClient struct {
	*Client
	tokenBucket *TokenBucket
	estimator   *TokenEstimator
	cfg         *config.RateLimitConfig
	onWait      WaitCallback
}

// NewRateLimitedClient creates a new rate-limited client wrapper
func NewRateLimitedClient(client *Client, cfg *config.RateLimitConfig) *RateLimitedClient {
	return &RateLimitedClient{
		Client:      client,
		tokenBucket: NewTokenBucket(cfg.TokensPerMinute),
		estimator:   NewTokenEstimator(),
		cfg:         cfg,
	}
}

// SetWaitCallback sets a callback to be invoked when waiting due to rate limiting.
// The callback is called both for token bucket waits and API 429 retry waits.
func (c *RateLimitedClient) SetWaitCallback(cb WaitCallback) {
	c.onWait = cb
	c.tokenBucket.SetWaitCallback(cb)
}

// Chat sends a message with rate limiting and returns the response
func (c *RateLimitedClient) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (*Response, error) {
	// Estimate tokens for rate limiting
	estimatedTokens := c.estimator.EstimateMessages(messages)
	estimatedTokens += c.estimator.EstimateTokens(systemPrompt)
	// Add overhead for tool definitions (~100 tokens per tool)
	estimatedTokens += len(tools) * 100

	logger.Debug("Rate limit: estimated %d tokens for request", estimatedTokens)

	// Wait for rate limit
	if err := c.tokenBucket.Wait(ctx, estimatedTokens); err != nil {
		return nil, err
	}

	// Call the underlying client with retry logic
	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateBackoff(attempt)
			logger.Debug("Rate limit: retry %d/%d, waiting %v", attempt, c.cfg.MaxRetries, delay)

			// Use callback if available
			if c.onWait != nil {
				err := c.onWait(ctx, WaitInfo{
					Duration:    delay,
					Reason:      "API returned 429",
					Attempt:     attempt,
					MaxAttempts: c.cfg.MaxRetries,
				})
				if err != nil {
					return nil, err
				}
			} else {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}

		resp, err := c.Client.Chat(ctx, messages, tools, systemPrompt)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if this is a rate limit error (429)
		if !isRateLimitError(err) {
			return nil, err
		}

		logger.Warn("Rate limit hit (attempt %d/%d): %v", attempt+1, c.cfg.MaxRetries+1, err)
	}

	return nil, lastErr
}

// ChatStream sends a message with rate limiting and streams the response
func (c *RateLimitedClient) ChatStream(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) <-chan StreamChunk {
	ch := make(chan StreamChunk, 100)

	go func() {
		defer close(ch)

		// Estimate tokens for rate limiting
		estimatedTokens := c.estimator.EstimateMessages(messages)
		estimatedTokens += c.estimator.EstimateTokens(systemPrompt)
		estimatedTokens += len(tools) * 100

		logger.Debug("Rate limit: estimated %d tokens for stream request", estimatedTokens)

		// Wait for rate limit
		if err := c.tokenBucket.Wait(ctx, estimatedTokens); err != nil {
			ch <- StreamChunk{Type: "error", Error: err}
			return
		}

		// Stream with retry logic
		var lastErr error
		for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
			if attempt > 0 {
				delay := c.calculateBackoff(attempt)
				logger.Debug("Rate limit: stream retry %d/%d, waiting %v", attempt, c.cfg.MaxRetries, delay)

				// Use callback if available
				if c.onWait != nil {
					err := c.onWait(ctx, WaitInfo{
						Duration:    delay,
						Reason:      "API returned 429",
						Attempt:     attempt,
						MaxAttempts: c.cfg.MaxRetries,
					})
					if err != nil {
						ch <- StreamChunk{Type: "error", Error: err}
						return
					}
				} else {
					select {
					case <-time.After(delay):
					case <-ctx.Done():
						ch <- StreamChunk{Type: "error", Error: ctx.Err()}
						return
					}
				}
			}

			// Get stream from underlying client
			stream := c.Client.ChatStream(ctx, messages, tools, systemPrompt)

			// Forward all chunks, checking for rate limit errors
			gotError := false
			for chunk := range stream {
				if chunk.Type == "error" && chunk.Error != nil {
					if isRateLimitError(chunk.Error) {
						lastErr = chunk.Error
						gotError = true
						logger.Warn("Rate limit hit on stream (attempt %d/%d): %v", attempt+1, c.cfg.MaxRetries+1, chunk.Error)
						break
					}
					// Non-rate-limit error, forward it
					ch <- chunk
					return
				}
				ch <- chunk
			}

			if !gotError {
				// Stream completed successfully
				return
			}
		}

		// Max retries exceeded
		if lastErr != nil {
			ch <- StreamChunk{Type: "error", Error: lastErr}
		}
	}()

	return ch
}

// calculateBackoff calculates the backoff delay for a retry attempt
// Uses exponential backoff with jitter
func (c *RateLimitedClient) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	backoff := float64(c.cfg.BaseDelay) * math.Pow(2, float64(attempt-1))

	// Add jitter (0-25% of backoff)
	jitter := backoff * 0.25 * rand.Float64()
	backoff += jitter

	// Cap at maxDelay
	if backoff > float64(c.cfg.MaxDelay) {
		backoff = float64(c.cfg.MaxDelay)
	}

	return time.Duration(backoff)
}

// isRateLimitError checks if an error is a rate limit (429) error
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "Rate limit") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "Too Many Requests")
}
