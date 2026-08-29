package retry

import (
	"testing"
	"time"
)

func TestBackoffCalculation(t *testing.T) {
	cfg := Config{
		InitialDelay: 1 * time.Second,
		Multiplier:   2.0,
		MaxRetries:   3,
		MaxDelay:     30 * time.Second,
		Jitter:       false,
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 0},
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 30 * time.Second}, // capped at max
		{7, 30 * time.Second},
	}

	for _, tt := range tests {
		got := cfg.Calculate(tt.attempt)
		if got != tt.want {
			t.Errorf("Calculate(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestShouldRetry(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.ShouldRetry(1) {
		t.Error("ShouldRetry(1) should be true")
	}
	if !cfg.ShouldRetry(3) {
		t.Error("ShouldRetry(3) should be true")
	}
	if cfg.ShouldRetry(4) {
		t.Error("ShouldRetry(4) should be false")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.InitialDelay != 1*time.Second {
		t.Errorf("InitialDelay = %v, want 1s", cfg.InitialDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want 2.0", cfg.Multiplier)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay = %v, want 30s", cfg.MaxDelay)
	}
}
