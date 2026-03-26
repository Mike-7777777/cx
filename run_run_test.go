package main

import (
	"testing"
	"time"
)

func TestSmartScore_PrefersLowUsage(t *testing.T) {
	// Same reset time, different usage. Lower usage = lower score.
	reset := 3 * time.Hour
	scoreA := smartScore(20, reset)
	scoreB := smartScore(80, reset)
	if scoreA >= scoreB {
		t.Errorf("lower usage should score better: 20%%=%f, 80%%=%f", scoreA, scoreB)
	}
}

func TestSmartScore_PrefersNearReset(t *testing.T) {
	// Same usage, different reset times. Near-reset = lower score (use it up before reset).
	scoreA := smartScore(50, 30*time.Minute) // resets in 30min
	scoreB := smartScore(50, 4*time.Hour)    // resets in 4h
	if scoreA >= scoreB {
		t.Errorf("near-reset should score better: 30m=%f, 4h=%f", scoreA, scoreB)
	}
}

func TestSmartScore_HighUsagePenalized(t *testing.T) {
	// >90% usage gets penalty score regardless of reset time.
	score := smartScore(95, 10*time.Minute)
	if score < 1000 {
		t.Errorf("95%% usage should be penalized (>1000), got %f", score)
	}
}

func TestSmartScore_ZeroUsageIsZero(t *testing.T) {
	// 0% usage * anything = 0 score (best possible).
	score := smartScore(0, 5*time.Hour)
	if score != 0 {
		t.Errorf("0%% usage should score 0, got %f", score)
	}
}

func TestSmartScore_JustResetLowUsage(t *testing.T) {
	// Just reset (timeToReset=0) with any usage = 0 score (about to waste capacity).
	score := smartScore(50, 0)
	if score != 0 {
		t.Errorf("zero time-to-reset should score 0 (resetFraction=0), got %f", score)
	}
}
