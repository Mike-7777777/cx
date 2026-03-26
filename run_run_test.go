package main

import (
	"testing"
	"time"
)

func TestSmartScore_PrefersLowUsage(t *testing.T) {
	// Same reset time, different usage. Lower usage = lower score.
	reset := 3 * time.Hour
	scoreA := smartScore(20, reset, 1.0)
	scoreB := smartScore(80, reset, 1.0)
	if scoreA >= scoreB {
		t.Errorf("lower usage should score better: 20%%=%f, 80%%=%f", scoreA, scoreB)
	}
}

func TestSmartScore_PrefersNearReset(t *testing.T) {
	// Same usage, different reset times. Near-reset = lower score (use it up before reset).
	scoreA := smartScore(50, 30*time.Minute, 1.0) // resets in 30min
	scoreB := smartScore(50, 4*time.Hour, 1.0)    // resets in 4h
	if scoreA >= scoreB {
		t.Errorf("near-reset should score better: 30m=%f, 4h=%f", scoreA, scoreB)
	}
}

func TestSmartScore_HighUsagePenalized(t *testing.T) {
	// >90% usage gets penalty score regardless of reset time.
	score := smartScore(95, 10*time.Minute, 1.0)
	if score < 1000 {
		t.Errorf("95%% usage should be penalized (>1000), got %f", score)
	}
}

func TestSmartScore_ZeroUsageIsZero(t *testing.T) {
	// 0% usage * anything = 0 score (best possible).
	score := smartScore(0, 5*time.Hour, 1.0)
	if score != 0 {
		t.Errorf("0%% usage should score 0, got %f", score)
	}
}

func TestSmartScore_JustResetLowUsage(t *testing.T) {
	// Just reset (timeToReset=0) with any usage = 0 score (about to waste capacity).
	score := smartScore(50, 0, 1.0)
	if score != 0 {
		t.Errorf("zero time-to-reset should score 0 (resetFraction=0), got %f", score)
	}
}

func TestSmartScore_7dHeadroomTiebreaker(t *testing.T) {
	// Same 5h scores, but different 7d headroom. More headroom = lower score (preferred).
	scoreA := smartScore(40, 2*time.Hour, 2.0) // lots of 7d room
	scoreB := smartScore(40, 2*time.Hour, 0.3) // tight on 7d
	if scoreA >= scoreB {
		t.Errorf("higher 7d headroom should score better: 2.0=%f, 0.3=%f", scoreA, scoreB)
	}
}

func TestSevenDayHeadroom(t *testing.T) {
	// 91% used, 1.5 days left → (9/1.5)/14.3 ≈ 0.42
	h := sevenDayHeadroom(91, 1.5)
	if h < 0.40 || h > 0.44 {
		t.Errorf("expected ~0.42, got %.2f", h)
	}

	// 50% used, 4 days left → (50/4)/14.3 ≈ 0.87
	h2 := sevenDayHeadroom(50, 4)
	if h2 < 0.85 || h2 > 0.90 {
		t.Errorf("expected ~0.87, got %.2f", h2)
	}

	// 0% used, 7 days left → (100/7)/14.3 ≈ 1.0
	h3 := sevenDayHeadroom(0, 7)
	if h3 < 0.98 || h3 > 1.02 {
		t.Errorf("expected ~1.0, got %.2f", h3)
	}

	// 0 days left → 1.0 (neutral)
	h4 := sevenDayHeadroom(99, 0)
	if h4 != 1.0 {
		t.Errorf("expected 1.0 when 0 days left, got %.2f", h4)
	}
}

// TestSmartScore_7dHeadroomDoesNotOverride5h verifies that a large difference
// in 7d headroom cannot flip rankings when the 5h signal is decisive.
// Account A: 20% usage (low)  — headroom=0.1 (tight 7d).
// Account B: 80% usage (high) — headroom=3.0 (generous 7d).
// A must still win even though B has far better 7d headroom.
func TestSmartScore_7dHeadroomDoesNotOverride5h(t *testing.T) {
	reset := 3 * time.Hour
	scoreA := smartScore(20, reset, 0.1) // low 5h usage, tight 7d
	scoreB := smartScore(80, reset, 3.0) // high 5h usage, generous 7d
	if scoreA >= scoreB {
		t.Errorf("5h signal must dominate: score(20%%, h=0.1)=%f should be less than score(80%%, h=3.0)=%f", scoreA, scoreB)
	}
}

// TestSmartScore_7dHeadroomBreaksTie verifies that when two accounts have
// identical 5h parameters, the account with higher 7d headroom wins (lower score).
func TestSmartScore_7dHeadroomBreaksTie(t *testing.T) {
	reset := 2 * time.Hour
	scoreLowHeadroom := smartScore(50, reset, 0.5)  // same 5h, tight 7d
	scoreHighHeadroom := smartScore(50, reset, 2.0) // same 5h, generous 7d
	if scoreHighHeadroom >= scoreLowHeadroom {
		t.Errorf("higher 7d headroom should win the tie: h=2.0 score=%f should be less than h=0.5 score=%f",
			scoreHighHeadroom, scoreLowHeadroom)
	}
}

// TestSmartScore_7dBonusCapped verifies that the 7d bonus is capped at 5 points.
// headroom=10.0 must yield the same score as headroom=2.0 (both hit the cap),
// given identical 5h parameters.
func TestSmartScore_7dBonusCapped(t *testing.T) {
	reset := 3 * time.Hour
	scoreAtCap := smartScore(50, reset, 2.0)     // bonus = (2.0-1)*5 = 5, exactly at cap
	scoreAboveCap := smartScore(50, reset, 10.0) // bonus = (10.0-1)*5 = 45, clamped to 5
	if scoreAtCap != scoreAboveCap {
		t.Errorf("7d bonus should be capped: h=2.0 score=%f, h=10.0 score=%f (expected equal)", scoreAtCap, scoreAboveCap)
	}
}

// TestSevenDayHeadroom_HighUsageLowDaysLeft verifies headroom when the weekly
// budget is nearly exhausted but only half a day remains.
// 95% used, 0.5 days left → remaining=5, dailyAvailable=10, dailyBudget≈14.3 → headroom≈0.7
// The account still has enough for half a day, so headroom > 0.5.
func TestSevenDayHeadroom_HighUsageLowDaysLeft(t *testing.T) {
	h := sevenDayHeadroom(95, 0.5)
	if h <= 0.5 {
		t.Errorf("expected headroom > 0.5 (enough for the remaining half-day), got %.4f", h)
	}
}

// TestSevenDayHeadroom_LowUsageHighDaysLeft verifies headroom for an account
// that is well ahead of schedule.
// 10% used, 6 days left → remaining=90, dailyAvailable=15, dailyBudget≈14.3 → headroom≈1.05
func TestSevenDayHeadroom_LowUsageHighDaysLeft(t *testing.T) {
	h := sevenDayHeadroom(10, 6)
	if h <= 0.9 || h >= 1.1 {
		t.Errorf("expected headroom in (0.9, 1.1) (on track), got %.4f", h)
	}
}
