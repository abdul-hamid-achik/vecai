package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/config"
)

func TestTokenEstimator_EstimateTokens(t *testing.T) {
	estimator := NewTokenEstimator()

	tests := []struct {
		name     string
		text     string
		wantMin  int
		wantMax  int
	}{
		{
			name:    "empty string",
			text:    "",
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "short text",
			text:    "Hello, world!",
			wantMin: 1,
			wantMax: 10,
		},
		{
			name:    "longer text",
			text:    "This is a longer piece of text that should result in a reasonable token estimate.",
			wantMin: 15,
			wantMax: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimator.EstimateTokens(tt.text)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("EstimateTokens() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestTokenEstimator_EstimateMessages(t *testing.T) {
	estimator := NewTokenEstimator()

	messages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there, how can I help you?"},
		{Role: "user", Content: "Tell me about Go programming."},
	}

	estimate := estimator.EstimateMessages(messages)

	// Should include overhead per message (4 tokens each = 12) plus content estimates
	if estimate < 15 {
		t.Errorf("EstimateMessages() = %v, expected at least 15 (overhead + content)", estimate)
	}
}

func TestTokenBucket_Wait(t *testing.T) {
	// Create a bucket with 1000 tokens/minute (about 16.67 tokens/second)
	bucket := NewTokenBucket(1000)

	ctx := context.Background()

	// First request should not block
	start := time.Now()
	err := bucket.Wait(ctx, 100)
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("First Wait() took too long: %v", elapsed)
	}
}

func TestTokenBucket_Wait_Context_Cancelled(t *testing.T) {
	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Create a very restrictive bucket that would require waiting
	bucket := NewTokenBucket(1) // 1 token/minute

	// First exhaust the burst allowance
	_ = bucket.Wait(context.Background(), 1000)

	// Now try with cancelled context - should fail
	err := bucket.Wait(ctx, 1000)
	if err == nil {
		t.Error("Wait() should return error when context is cancelled")
	}
	if err != context.Canceled {
		t.Errorf("Wait() error = %v, want context.Canceled", err)
	}
}

func TestRateLimitedClient_calculateBackoff(t *testing.T) {
	cfg := &config.RateLimitConfig{
		BaseDelay: 1 * time.Second,
		MaxDelay:  60 * time.Second,
	}

	client := &RateLimitedClient{cfg: cfg}

	tests := []struct {
		attempt int
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			attempt: 1,
			wantMin: 1 * time.Second,
			wantMax: 2 * time.Second, // base + 25% jitter max
		},
		{
			attempt: 2,
			wantMin: 2 * time.Second,
			wantMax: 3 * time.Second,
		},
		{
			attempt: 3,
			wantMin: 4 * time.Second,
			wantMax: 6 * time.Second,
		},
		{
			attempt: 10,
			wantMin: 50 * time.Second, // Would be >512s without cap, capped at 60s
			wantMax: 75 * time.Second, // 60s + 25% jitter
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := client.calculateBackoff(tt.attempt)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calculateBackoff(%d) = %v, want between %v and %v", tt.attempt, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "429 error",
			err:  errors.New("status code 429"),
			want: true,
		},
		{
			name: "rate limit error",
			err:  errors.New("rate limit exceeded"),
			want: true,
		},
		{
			name: "Rate limit error (capitalized)",
			err:  errors.New("Rate limit reached"),
			want: true,
		},
		{
			name: "too many requests",
			err:  errors.New("too many requests"),
			want: true,
		},
		{
			name: "Too Many Requests (HTTP status text)",
			err:  errors.New("429 Too Many Requests"),
			want: true,
		},
		{
			name: "other error",
			err:  errors.New("connection refused"),
			want: false,
		},
		{
			name: "timeout error",
			err:  errors.New("request timeout"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRateLimitError(tt.err); got != tt.want {
				t.Errorf("isRateLimitError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewTokenBucket(t *testing.T) {
	// Test that bucket is created with reasonable parameters
	bucket := NewTokenBucket(30000) // 30k tokens/minute

	if bucket.limiter == nil {
		t.Error("NewTokenBucket() created bucket with nil limiter")
	}
}

func TestRateLimitConfig_Defaults(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg.RateLimit.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.RateLimit.MaxRetries)
	}
	if cfg.RateLimit.BaseDelay != 1*time.Second {
		t.Errorf("BaseDelay = %v, want 1s", cfg.RateLimit.BaseDelay)
	}
	if cfg.RateLimit.MaxDelay != 60*time.Second {
		t.Errorf("MaxDelay = %v, want 60s", cfg.RateLimit.MaxDelay)
	}
	if cfg.RateLimit.TokensPerMinute != 30000 {
		t.Errorf("TokensPerMinute = %d, want 30000", cfg.RateLimit.TokensPerMinute)
	}
	if !cfg.RateLimit.EnableRateLimiting {
		t.Error("EnableRateLimiting = false, want true")
	}
}
