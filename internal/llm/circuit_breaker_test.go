package llm

import (
	"testing"
	"time"
)

func TestCircuitBreaker_StartsClosedAndAllows(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed, got %d", cb.State())
	}
	if !cb.Allow() {
		t.Error("closed circuit should allow requests")
	}
}

func TestCircuitBreaker_OpensAfterMaxFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.State() != CircuitOpen {
		t.Errorf("expected open after 3 failures, got %d", cb.State())
	}
	if cb.Allow() {
		t.Error("open circuit should reject requests")
	}
}

func TestCircuitBreaker_TransitionsToHalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatal("expected open")
	}

	time.Sleep(60 * time.Millisecond)

	if !cb.Allow() {
		t.Error("should allow after timeout (half-open)")
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected half-open, got %d", cb.State())
	}
}

func TestCircuitBreaker_ClosesAfterSuccessesInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // Transition to half-open

	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after successes in half-open, got %d", cb.State())
	}
}

func TestCircuitBreaker_ReOpensOnFailureInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // Transition to half-open

	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("expected open after failure in half-open, got %d", cb.State())
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // Resets count
	cb.RecordFailure()
	// Should still be closed (1 failure, not 3)
	if cb.State() != CircuitClosed {
		t.Error("expected closed after success reset")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Second)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatal("expected open")
	}
	cb.Reset()
	if cb.State() != CircuitClosed {
		t.Error("expected closed after reset")
	}
	if !cb.Allow() {
		t.Error("should allow after reset")
	}
}

func TestCircuitBreaker_DefaultValues(t *testing.T) {
	cb := NewCircuitBreaker(0, 0)
	if cb.maxFailures != 5 {
		t.Errorf("expected default maxFailures 5, got %d", cb.maxFailures)
	}
	if cb.timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", cb.timeout)
	}
}
