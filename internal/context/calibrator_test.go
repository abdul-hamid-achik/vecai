package context

import (
	"math"
	"testing"
)

func TestNewTokenCalibrator(t *testing.T) {
	c := NewTokenCalibrator(50)
	if c == nil {
		t.Fatal("NewTokenCalibrator returned nil")
	}
	if c.Ratio() != 1.0 {
		t.Errorf("expected initial ratio 1.0, got %f", c.Ratio())
	}
	if c.SampleCount() != 0 {
		t.Errorf("expected 0 samples, got %d", c.SampleCount())
	}
}

func TestNewTokenCalibratorDefaultMaxSamples(t *testing.T) {
	c := NewTokenCalibrator(0)
	if c.maxSamples != 100 {
		t.Errorf("expected default maxSamples 100, got %d", c.maxSamples)
	}
}

func TestCalibratorRecord(t *testing.T) {
	c := NewTokenCalibrator(100)

	c.Record(120, 100) // estimated 20% too high
	if c.SampleCount() != 1 {
		t.Errorf("expected 1 sample, got %d", c.SampleCount())
	}

	expectedRatio := 1.2 // 120/100
	if math.Abs(c.Ratio()-expectedRatio) > 0.01 {
		t.Errorf("expected ratio ~%.2f, got %.2f", expectedRatio, c.Ratio())
	}
}

func TestCalibratorRecordIgnoresZeroActual(t *testing.T) {
	c := NewTokenCalibrator(100)
	c.Record(100, 0)

	if c.SampleCount() != 0 {
		t.Errorf("expected 0 samples after recording with actual=0, got %d", c.SampleCount())
	}
}

func TestCalibratorAdjust(t *testing.T) {
	c := NewTokenCalibrator(100)

	// No samples: should return input unchanged
	adjusted := c.Adjust(100)
	if adjusted != 100 {
		t.Errorf("expected 100 with no samples, got %d", adjusted)
	}

	// Record that estimates are consistently 20% too high
	c.Record(120, 100)
	c.Record(240, 200)
	c.Record(60, 50)

	// Ratio should be ~1.2 (estimated/actual)
	// Adjust(120) should return 120/1.2 = 100
	adjusted = c.Adjust(120)
	if adjusted != 100 {
		t.Errorf("expected adjusted value 100, got %d", adjusted)
	}
}

func TestCalibratorMaxSamples(t *testing.T) {
	c := NewTokenCalibrator(3)

	c.Record(100, 100) // ratio 1.0
	c.Record(100, 100) // ratio 1.0
	c.Record(100, 100) // ratio 1.0
	c.Record(200, 100) // ratio 2.0 (only this and previous two are kept)

	if c.SampleCount() != 3 {
		t.Errorf("expected 3 samples (maxSamples), got %d", c.SampleCount())
	}

	// After eviction, samples are: (100,100), (100,100), (200,100)
	// Total estimated: 400, total actual: 300 -> ratio ~1.33
	expectedRatio := 400.0 / 300.0
	if math.Abs(c.Ratio()-expectedRatio) > 0.01 {
		t.Errorf("expected ratio ~%.2f, got %.2f", expectedRatio, c.Ratio())
	}
}

func TestCalibratorMultipleSamples(t *testing.T) {
	c := NewTokenCalibrator(100)

	// Simulate estimates being 10% too high for code
	c.Record(110, 100)
	c.Record(55, 50)
	c.Record(220, 200)

	// Total estimated: 385, total actual: 350 -> ratio 1.1
	expectedRatio := 385.0 / 350.0
	if math.Abs(c.Ratio()-expectedRatio) > 0.01 {
		t.Errorf("expected ratio ~%.2f, got %.2f", expectedRatio, c.Ratio())
	}

	// Adjusting 110 should give ~100
	adjusted := c.Adjust(110)
	expected := int(110.0 / expectedRatio)
	if adjusted != expected {
		t.Errorf("expected adjusted value %d, got %d", expected, adjusted)
	}
}
