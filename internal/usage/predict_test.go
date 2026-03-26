package usage

import (
	"testing"
	"time"
)

func TestEstimateExhaustion_LinearGrowth(t *testing.T) {
	// 40% used, 2h elapsed (windowSize=5h so timeToReset=3h).
	// velocity = 40% / 2h = 20%/h
	// remaining = 60%, hoursToExhaust = 60/20 = 3h
	// timeToExhaust (3h) == timeToReset (3h) → boundary case: 3h <= 3h → exhausts.
	now := time.Now()
	timeToReset := 3 * time.Hour

	est := EstimateExhaustion(40.0, timeToReset, now)

	if est.Exhausted {
		t.Error("40% used should not be exhausted")
	}
	if est.TimeToExhaust == 0 {
		t.Error("expected non-zero TimeToExhaust")
	}
	// Should exhaust in roughly 3h (±5min tolerance).
	want := 3 * time.Hour
	diff := est.TimeToExhaust - want
	if diff < 0 {
		diff = -diff
	}
	if diff > 5*time.Minute {
		t.Errorf("TimeToExhaust = %v, want ~%v", est.TimeToExhaust, want)
	}
}

func TestEstimateExhaustion_AlreadyExhausted(t *testing.T) {
	now := time.Now()
	est := EstimateExhaustion(100.0, 2*time.Hour, now)
	if !est.Exhausted {
		t.Error("100% should be exhausted")
	}
}

func TestEstimateExhaustion_ZeroUsage(t *testing.T) {
	now := time.Now()
	est := EstimateExhaustion(0.0, 3*time.Hour, now)
	if est.Exhausted {
		t.Error("0% should not be exhausted")
	}
	if est.TimeToExhaust != 0 {
		t.Errorf("0%% usage: TimeToExhaust should be 0 (never), got %v", est.TimeToExhaust)
	}
}

func TestCalculateVelocity_FromBlocks(t *testing.T) {
	// 4 entries spread across 4 hours → 1 msg/h
	now := time.Now()
	entries := []Entry{
		{
			Model:     "claude-sonnet-4-6",
			Timestamp: now.Add(-3*time.Hour - 30*time.Minute),
			Usage:     TokenUsage{InputTokens: 100, OutputTokens: 50},
		},
		{
			Model:     "claude-sonnet-4-6",
			Timestamp: now.Add(-2*time.Hour - 30*time.Minute),
			Usage:     TokenUsage{InputTokens: 100, OutputTokens: 50},
		},
		{
			Model:     "claude-sonnet-4-6",
			Timestamp: now.Add(-1*time.Hour - 30*time.Minute),
			Usage:     TokenUsage{InputTokens: 100, OutputTokens: 50},
		},
		{
			Model:     "claude-sonnet-4-6",
			Timestamp: now.Add(-30 * time.Minute),
			Usage:     TokenUsage{InputTokens: 100, OutputTokens: 50},
		},
	}

	v := CalculateVelocity(entries, 4*time.Hour)

	// 4 msgs / 4h = 1 msg/h
	if v.MsgsPerHour < 0.9 || v.MsgsPerHour > 1.1 {
		t.Errorf("MsgsPerHour = %.2f, want ~1.0", v.MsgsPerHour)
	}
	if v.TokensPerHour <= 0 {
		t.Error("TokensPerHour should be positive")
	}
	if v.CostPerHour <= 0 {
		t.Error("CostPerHour should be positive")
	}
}
