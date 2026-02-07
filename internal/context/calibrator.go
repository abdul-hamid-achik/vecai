package context

import "sync"

// TokenCalibrator learns the ratio between estimated and actual token counts
// by recording samples and adjusting future estimates accordingly.
type TokenCalibrator struct {
	mu         sync.Mutex
	samples    []calibrationSample
	ratio      float64 // Current learned ratio (estimated/actual)
	maxSamples int
}

type calibrationSample struct {
	estimated int
	actual    int
}

// NewTokenCalibrator creates a new calibrator that tracks up to maxSamples
func NewTokenCalibrator(maxSamples int) *TokenCalibrator {
	if maxSamples <= 0 {
		maxSamples = 100
	}
	return &TokenCalibrator{
		ratio:      1.0,
		maxSamples: maxSamples,
	}
}

// Record records a calibration sample of estimated vs actual token count
func (c *TokenCalibrator) Record(estimated, actual int) {
	if actual <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.samples = append(c.samples, calibrationSample{
		estimated: estimated,
		actual:    actual,
	})

	// Evict oldest samples if over capacity
	if len(c.samples) > c.maxSamples {
		c.samples = c.samples[len(c.samples)-c.maxSamples:]
	}

	// Recalculate ratio from all samples
	c.recalculate()
}

// Adjust adjusts an estimated token count using the learned ratio
func (c *TokenCalibrator) Adjust(estimated int) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ratio <= 0 || len(c.samples) == 0 {
		return estimated
	}

	return int(float64(estimated) / c.ratio)
}

// Ratio returns the current learned ratio (estimated/actual)
func (c *TokenCalibrator) Ratio() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ratio
}

// SampleCount returns how many samples have been recorded
func (c *TokenCalibrator) SampleCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.samples)
}

// recalculate updates the ratio from all current samples.
// Must be called while holding c.mu.
func (c *TokenCalibrator) recalculate() {
	if len(c.samples) == 0 {
		c.ratio = 1.0
		return
	}

	var totalEstimated, totalActual int
	for _, s := range c.samples {
		totalEstimated += s.estimated
		totalActual += s.actual
	}

	if totalActual > 0 {
		c.ratio = float64(totalEstimated) / float64(totalActual)
	} else {
		c.ratio = 1.0
	}
}
