package ui

import (
	"context"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"zero", 0, "0s"},
		{"seconds only", 45 * time.Second, "45s"},
		{"one minute", 60 * time.Second, "1m00s"},
		{"one minute thirty", 90 * time.Second, "1m30s"},
		{"five minutes", 5 * time.Minute, "5m00s"},
		{"five minutes thirty", 5*time.Minute + 30*time.Second, "5m30s"},
		{"negative rounds to zero", -5 * time.Second, "0s"},
		{"rounds down", 45*time.Second + 400*time.Millisecond, "45s"},
		{"rounds up", 45*time.Second + 600*time.Millisecond, "46s"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatDuration(tc.duration)
			if result != tc.expected {
				t.Errorf("formatDuration(%v) = %q, expected %q", tc.duration, result, tc.expected)
			}
		})
	}
}

func TestSpinnerContextCancellation(t *testing.T) {
	output := NewOutputHandler()
	spinner := NewSpinner(output)

	ctx, cancel := context.WithCancel(context.Background())

	// Start spinner in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- spinner.Start(ctx, SpinnerConfig{
			Message:  "Test",
			Duration: 10 * time.Second, // Long duration
		})
	}()

	// Give spinner time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Should complete quickly with context error
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Spinner did not respond to context cancellation")
	}
}

func TestSpinnerShortWaitSkipsAnimation(t *testing.T) {
	output := NewOutputHandler()
	spinner := NewSpinner(output)

	ctx := context.Background()
	start := time.Now()

	// Short wait should not animate
	err := spinner.Start(ctx, SpinnerConfig{
		Message:  "Test",
		Duration: 100 * time.Millisecond, // Less than 500ms threshold
	})

	elapsed := time.Since(start)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Should still wait the full duration
	if elapsed < 100*time.Millisecond {
		t.Errorf("Spinner returned too quickly: %v", elapsed)
	}
}

func TestSpinnerConfig(t *testing.T) {
	// Test that SpinnerConfig fields are properly accessible
	cfg := SpinnerConfig{
		Message:     "Rate limited",
		Reason:      "API returned 429",
		Duration:    45 * time.Second,
		Attempt:     2,
		MaxAttempts: 5,
	}

	if cfg.Message != "Rate limited" {
		t.Errorf("Message = %q, expected %q", cfg.Message, "Rate limited")
	}
	if cfg.Reason != "API returned 429" {
		t.Errorf("Reason = %q, expected %q", cfg.Reason, "API returned 429")
	}
	if cfg.Duration != 45*time.Second {
		t.Errorf("Duration = %v, expected %v", cfg.Duration, 45*time.Second)
	}
	if cfg.Attempt != 2 {
		t.Errorf("Attempt = %d, expected %d", cfg.Attempt, 2)
	}
	if cfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, expected %d", cfg.MaxAttempts, 5)
	}
}

func TestSpinnerBuildStatusLine(t *testing.T) {
	// Create output handler with colors disabled for predictable output
	output := &OutputHandler{useColors: false}
	spinner := NewSpinner(output)

	tests := []struct {
		name        string
		frame       string
		message     string
		reason      string
		remaining   time.Duration
		attempt     int
		maxAttempts int
		expected    string
	}{
		{
			name:      "basic message",
			frame:     "⠋",
			message:   "Rate limited",
			remaining: 45 * time.Second,
			expected:  "⠋ Rate limited | 45s remaining",
		},
		{
			name:        "with retry",
			frame:       "⠹",
			message:     "Rate limited",
			attempt:     2,
			maxAttempts: 5,
			remaining:   30 * time.Second,
			expected:    "⠹ Rate limited | Retry 2/5 | 30s remaining",
		},
		{
			name:      "with reason",
			frame:     "⠸",
			message:   "Rate limited",
			reason:    "API returned 429",
			remaining: 1 * time.Minute,
			expected:  "⠸ Rate limited | API returned 429 | 1m00s remaining",
		},
		{
			name:        "full info",
			frame:       "⠼",
			message:     "Rate limited",
			reason:      "API returned 429",
			attempt:     3,
			maxAttempts: 5,
			remaining:   2*time.Minute + 15*time.Second,
			expected:    "⠼ Rate limited | Retry 3/5 | API returned 429 | 2m15s remaining",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := spinner.buildStatusLine(
				tc.frame,
				tc.message,
				tc.reason,
				tc.remaining,
				tc.attempt,
				tc.maxAttempts,
			)
			if result != tc.expected {
				t.Errorf("buildStatusLine() = %q, expected %q", result, tc.expected)
			}
		})
	}
}

func TestNewSpinner(t *testing.T) {
	output := NewOutputHandler()
	spinner := NewSpinner(output)

	if spinner == nil {
		t.Fatal("NewSpinner returned nil")
	}
	if spinner.output != output {
		t.Error("Spinner output handler not set correctly")
	}
}
