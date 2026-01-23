package ui

import (
	"context"
	"fmt"
	"os"
	"time"
)

// Braille spinner animation frames
var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// SpinnerConfig holds configuration for a spinner display
type SpinnerConfig struct {
	Message     string        // Main message (e.g., "Rate limited")
	Reason      string        // Reason for waiting (e.g., "API returned 429")
	Duration    time.Duration // Total wait duration
	Attempt     int           // Current attempt number (1-based)
	MaxAttempts int           // Maximum number of attempts
}

// Spinner provides animated terminal feedback
type Spinner struct {
	output *OutputHandler
}

// NewSpinner creates a new spinner attached to an output handler
func NewSpinner(output *OutputHandler) *Spinner {
	return &Spinner{
		output: output,
	}
}

// Start displays a spinner with countdown until duration elapses or context is cancelled.
// It blocks until complete.
func (s *Spinner) Start(ctx context.Context, cfg SpinnerConfig) error {
	// Skip spinner for very short waits to avoid flicker
	if cfg.Duration < 500*time.Millisecond {
		select {
		case <-time.After(cfg.Duration):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Use static output for non-TTY mode
	if !s.output.IsTTY() {
		return s.staticWait(ctx, cfg)
	}

	return s.animatedWait(ctx, cfg)
}

// staticWait displays a single line and waits (for non-TTY/piped output)
func (s *Spinner) staticWait(ctx context.Context, cfg SpinnerConfig) error {
	// Format: ℹ Rate limited: waiting 45s (retry 2/5, API returned 429)
	msg := fmt.Sprintf("ℹ %s: waiting %s", cfg.Message, formatDuration(cfg.Duration))
	if cfg.MaxAttempts > 0 {
		msg += fmt.Sprintf(" (retry %d/%d", cfg.Attempt, cfg.MaxAttempts)
		if cfg.Reason != "" {
			msg += ", " + cfg.Reason
		}
		msg += ")"
	} else if cfg.Reason != "" {
		msg += fmt.Sprintf(" (%s)", cfg.Reason)
	}
	fmt.Fprintln(os.Stderr, msg)

	select {
	case <-time.After(cfg.Duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// animatedWait displays an animated spinner with countdown (for TTY mode)
func (s *Spinner) animatedWait(ctx context.Context, cfg SpinnerConfig) error {
	startTime := time.Now()
	frameIndex := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Cleanup function to clear the line when done
	defer s.cleanup()

	for {
		elapsed := time.Since(startTime)
		remaining := max(cfg.Duration-elapsed, 0)

		// Build the status line
		frame := string(spinnerFrames[frameIndex])
		line := s.buildStatusLine(frame, cfg.Message, cfg.Reason, remaining, cfg.Attempt, cfg.MaxAttempts)

		// Write to stderr (clear line first, then write)
		fmt.Fprint(os.Stderr, ClearLine+CursorStart+line)

		// Check if we're done
		if remaining == 0 {
			return nil
		}

		// Wait for next tick or cancellation
		select {
		case <-ticker.C:
			frameIndex = (frameIndex + 1) % len(spinnerFrames)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// buildStatusLine constructs the animated status line
func (s *Spinner) buildStatusLine(frame, message, reason string, remaining time.Duration, attempt, maxAttempts int) string {
	// Format: ⠹ Rate limited | Retry 2/5 | API returned 429 | 45s remaining
	var line string

	if s.output.UseColors() {
		line = fmt.Sprintf("%s%s%s %s%s%s",
			Cyan, frame, Reset,
			Yellow, message, Reset)
	} else {
		line = fmt.Sprintf("%s %s", frame, message)
	}

	if maxAttempts > 0 {
		if s.output.UseColors() {
			line += fmt.Sprintf(" %s|%s Retry %d/%d", Dim, Reset, attempt, maxAttempts)
		} else {
			line += fmt.Sprintf(" | Retry %d/%d", attempt, maxAttempts)
		}
	}

	if reason != "" {
		if s.output.UseColors() {
			line += fmt.Sprintf(" %s|%s %s", Dim, Reset, reason)
		} else {
			line += fmt.Sprintf(" | %s", reason)
		}
	}

	remainingStr := formatDuration(remaining)
	if s.output.UseColors() {
		line += fmt.Sprintf(" %s|%s %s%s remaining%s", Dim, Reset, Bold, remainingStr, Reset)
	} else {
		line += fmt.Sprintf(" | %s remaining", remainingStr)
	}

	return line
}

// cleanup clears the spinner line completely
func (s *Spinner) cleanup() {
	if s.output.IsTTY() {
		fmt.Fprint(os.Stderr, ClearLine+CursorStart)
	}
}

// formatDuration formats a duration for display (45s, 1m30s, 5m00s)
func formatDuration(d time.Duration) string {
	d = max(d.Round(time.Second), 0)

	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60

	if minutes == 0 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dm%02ds", minutes, seconds)
}
