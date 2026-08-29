package retry

import (
	"math"
	"math/rand"
	"time"
)

// Config holds retry policy parameters.
type Config struct {
	InitialDelay time.Duration
	Multiplier   float64
	MaxRetries   int
	MaxDelay     time.Duration
	Jitter       bool
}

// DefaultConfig returns the default retry policy from the PRD.
func DefaultConfig() Config {
	return Config{
		InitialDelay: 1 * time.Second,
		Multiplier:   2.0,
		MaxRetries:   3,
		MaxDelay:     30 * time.Second,
		Jitter:       true,
	}
}

// Calculate returns the delay for the given retry attempt.
func (c Config) Calculate(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	delay := float64(c.InitialDelay) * math.Pow(c.Multiplier, float64(attempt-1))
	if delay > float64(c.MaxDelay) {
		delay = float64(c.MaxDelay)
	}
	if c.Jitter {
		// Add 0-30% jitter to avoid thundering herd
		jitter := delay * 0.3 * rand.Float64()
		delay += jitter
	}
	return time.Duration(delay)
}

// ShouldRetry returns true if another retry is allowed.
func (c Config) ShouldRetry(attempt int) bool {
	return attempt < c.MaxRetries
}
