package main

import (
	"strings"
	"testing"
	"time"

	"github.com/Mike-7777777/cx/internal/config"
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

// --- Tests for selectPreferred, selectBalanced, ensureAllAccounts, replaceOrAppendEnv ---

func TestSelectPreferred_UnderThreshold(t *testing.T) {
	scores := []accountScore{
		{name: "alpha", fiveHPct: 10},
		{name: "beta", fiveHPct: 50},
	}
	selected, reason := selectPreferred(scores, "beta")
	if selected.name != "beta" {
		t.Errorf("expected beta, got %s", selected.name)
	}
	if !strings.Contains(reason, "preferred") {
		t.Errorf("reason should mention preferred: %q", reason)
	}
}

func TestSelectPreferred_OverThreshold(t *testing.T) {
	scores := []accountScore{
		{name: "alpha", fiveHPct: 10},
		{name: "beta", fiveHPct: 85},
	}
	selected, reason := selectPreferred(scores, "beta")
	if selected.name != "alpha" {
		t.Errorf("should fall back to best account: got %s", selected.name)
	}
	if !strings.Contains(reason, "fell back") {
		t.Errorf("reason should mention fallback: %q", reason)
	}
}

func TestSelectPreferred_NotFound(t *testing.T) {
	scores := []accountScore{
		{name: "alpha", fiveHPct: 10},
	}
	selected, reason := selectPreferred(scores, "nonexistent")
	if selected.name != "alpha" {
		t.Errorf("should fall back to best: got %s", selected.name)
	}
	if !strings.Contains(reason, "not found") {
		t.Errorf("reason should say not found: %q", reason)
	}
}

func TestSelectBalanced_RoundRobin(t *testing.T) {
	scores := []accountScore{
		{name: "a", fiveHPct: 20},
		{name: "b", fiveHPct: 30},
		{name: "c", fiveHPct: 40},
	}
	// Reset the counter so test is deterministic.
	writeRunCounter(0)
	s1, _ := selectBalanced(scores)
	s2, _ := selectBalanced(scores)
	// Subsequent calls should not always return the same account.
	// (They may or may not differ depending on counter mod 3.)
	if s1.name == "" || s2.name == "" {
		t.Errorf("selectBalanced should return valid accounts")
	}
}

func TestSelectBalanced_SkipsOverloaded(t *testing.T) {
	scores := []accountScore{
		{name: "hot", fiveHPct: 95},  // over balanceThreshold (90)
		{name: "cool", fiveHPct: 40}, // under threshold
	}
	writeRunCounter(0) // counter=1 → idx=1%2=1 → "hot" but skip → try idx=0 → "cool"... actually logic is (counter+attempt)%n
	selected, _ := selectBalanced(scores)
	if selected.name == "hot" {
		t.Errorf("should skip overloaded account, got %s", selected.name)
	}
}

func TestSelectBalanced_AllOverThreshold(t *testing.T) {
	scores := []accountScore{
		{name: "a", fiveHPct: 95},
		{name: "b", fiveHPct: 92},
	}
	writeRunCounter(0)
	selected, reason := selectBalanced(scores)
	if selected.name != "a" {
		t.Errorf("when all over threshold, should pick best scored (first): got %s", selected.name)
	}
	if !strings.Contains(reason, "all above threshold") {
		t.Errorf("reason should say all above threshold: %q", reason)
	}
}

func TestEnsureAllAccounts_AddsMissing(t *testing.T) {
	reg := &config.Registry{
		Main: "alpha",
		Accounts: map[string]config.Account{
			"alpha": {ConfigDir: "/a"},
			"beta":  {ConfigDir: "/b"},
		},
	}
	scores := []accountScore{
		{name: "alpha", fiveHPct: 30, dir: "/a"},
	}
	result := ensureAllAccounts(reg, scores)
	if len(result) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(result))
	}
	found := false
	for _, s := range result {
		if s.name == "beta" {
			found = true
			if s.fiveHPct != 0 {
				t.Errorf("missing account should have 0%% usage, got %.0f%%", s.fiveHPct)
			}
		}
	}
	if !found {
		t.Error("beta should be added to scores")
	}
}

func TestReplaceOrAppendEnv_ReplacesExisting(t *testing.T) {
	env := []string{"HOME=/home/user", "PATH=/usr/bin", "CLAUDE_CONFIG_DIR=/old"}
	result := replaceOrAppendEnv(env, "CLAUDE_CONFIG_DIR", "/new")
	found := false
	for _, e := range result {
		if e == "CLAUDE_CONFIG_DIR=/new" {
			found = true
		}
		if e == "CLAUDE_CONFIG_DIR=/old" {
			t.Error("old value should be replaced")
		}
	}
	if !found {
		t.Error("new value should be present")
	}
	if len(result) != 3 {
		t.Errorf("slice length should not change: got %d", len(result))
	}
}

func TestReplaceOrAppendEnv_AppendsNew(t *testing.T) {
	env := []string{"HOME=/home/user"}
	result := replaceOrAppendEnv(env, "CLAUDE_CONFIG_DIR", "/new")
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	if result[1] != "CLAUDE_CONFIG_DIR=/new" {
		t.Errorf("expected appended value, got %q", result[1])
	}
}

func TestReplaceOrAppendEnv_EmptySlice(t *testing.T) {
	result := replaceOrAppendEnv(nil, "KEY", "value")
	if len(result) != 1 || result[0] != "KEY=value" {
		t.Errorf("expected [KEY=value], got %v", result)
	}
}

// --- Tests for parseRunArgs (flag parsing + pass-through) ---

func TestParseRunArgs_YoloShorthand(t *testing.T) {
	prefer, balance, claudeArgs, showHelp, err := parseRunArgs([]string{"-y"})
	if err != nil {
		t.Fatal(err)
	}
	if showHelp || balance || prefer != "" {
		t.Errorf("unexpected cx flags: prefer=%q balance=%v help=%v", prefer, balance, showHelp)
	}
	if len(claudeArgs) != 1 || claudeArgs[0] != "--dangerously-skip-permissions" {
		t.Errorf("claudeArgs=%v, want [--dangerously-skip-permissions]", claudeArgs)
	}
}

func TestParseRunArgs_YoloLonghand(t *testing.T) {
	_, _, claudeArgs, _, err := parseRunArgs([]string{"--yolo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(claudeArgs) != 1 || claudeArgs[0] != "--dangerously-skip-permissions" {
		t.Errorf("claudeArgs=%v, want [--dangerously-skip-permissions]", claudeArgs)
	}
}

func TestParseRunArgs_UnknownFlagsPassThrough(t *testing.T) {
	_, _, claudeArgs, _, err := parseRunArgs([]string{"--remote-control", "--verbose"})
	if err != nil {
		t.Fatal(err)
	}
	if len(claudeArgs) != 2 || claudeArgs[0] != "--remote-control" || claudeArgs[1] != "--verbose" {
		t.Errorf("claudeArgs=%v, want [--remote-control --verbose]", claudeArgs)
	}
}

func TestParseRunArgs_MixedCxAndClaudeFlags(t *testing.T) {
	prefer, balance, claudeArgs, _, err := parseRunArgs([]string{"--prefer", "work", "-y", "--verbose"})
	if err != nil {
		t.Fatal(err)
	}
	if prefer != "work" {
		t.Errorf("prefer=%q, want work", prefer)
	}
	if balance {
		t.Error("balance should be false")
	}
	if len(claudeArgs) != 2 || claudeArgs[0] != "--dangerously-skip-permissions" || claudeArgs[1] != "--verbose" {
		t.Errorf("claudeArgs=%v, want [--dangerously-skip-permissions --verbose]", claudeArgs)
	}
}

func TestParseRunArgs_DoubleDashSeparator(t *testing.T) {
	_, _, claudeArgs, _, err := parseRunArgs([]string{"--", "-p", "fix bug"})
	if err != nil {
		t.Fatal(err)
	}
	if len(claudeArgs) != 2 || claudeArgs[0] != "-p" || claudeArgs[1] != "fix bug" {
		t.Errorf("claudeArgs=%v, want [-p fix bug]", claudeArgs)
	}
}

func TestParseRunArgs_PreferWithoutValue(t *testing.T) {
	_, _, _, _, err := parseRunArgs([]string{"--prefer"})
	if err == nil {
		t.Error("expected error for --prefer without value")
	}
}

func TestParseRunArgs_PreferEqualsForm(t *testing.T) {
	prefer, _, _, _, err := parseRunArgs([]string{"--prefer=work"})
	if err != nil {
		t.Fatal(err)
	}
	if prefer != "work" {
		t.Errorf("prefer=%q, want work", prefer)
	}
}

func TestParseRunArgs_BalanceWithYolo(t *testing.T) {
	_, balance, claudeArgs, _, err := parseRunArgs([]string{"-y", "--balance"})
	if err != nil {
		t.Fatal(err)
	}
	if !balance {
		t.Error("balance should be true")
	}
	if len(claudeArgs) != 1 || claudeArgs[0] != "--dangerously-skip-permissions" {
		t.Errorf("claudeArgs=%v, want [--dangerously-skip-permissions]", claudeArgs)
	}
}

func TestParseRunArgs_HelpFlag(t *testing.T) {
	_, _, _, showHelp, err := parseRunArgs([]string{"-h"})
	if err != nil {
		t.Fatal(err)
	}
	if !showHelp {
		t.Error("showHelp should be true")
	}
}

func TestParseRunArgs_EmptyArgs(t *testing.T) {
	prefer, balance, claudeArgs, showHelp, err := parseRunArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if prefer != "" || balance || showHelp || len(claudeArgs) != 0 {
		t.Errorf("empty args should produce zero state: prefer=%q balance=%v help=%v claude=%v",
			prefer, balance, showHelp, claudeArgs)
	}
}

func TestParseRunArgs_PreferShorthand(t *testing.T) {
	prefer, _, _, _, err := parseRunArgs([]string{"-pf", "work"})
	if err != nil {
		t.Fatal(err)
	}
	if prefer != "work" {
		t.Errorf("prefer=%q, want work", prefer)
	}
}

func TestParseRunArgs_PreferShorthandWithoutValue(t *testing.T) {
	_, _, _, _, err := parseRunArgs([]string{"-pf"})
	if err == nil {
		t.Error("expected error for -pf without value")
	}
}

func TestParseRunArgs_ClaudePromptFlag(t *testing.T) {
	_, _, claudeArgs, _, err := parseRunArgs([]string{"-p", "fix the bug"})
	if err != nil {
		t.Fatal(err)
	}
	if len(claudeArgs) != 2 || claudeArgs[0] != "-p" || claudeArgs[1] != "fix the bug" {
		t.Errorf("claudeArgs=%v, want [-p fix the bug]", claudeArgs)
	}
}

// TestParseRunArgs_DashRcYoloPreferQM verifies the symmetric behaviour with
// cx resume: `cx run -rc --yolo --prefer QM` routes to QM with -rc and
// --dangerously-skip-permissions forwarded to claude.
func TestParseRunArgs_DashRcYoloPreferQM(t *testing.T) {
	prefer, balance, claudeArgs, showHelp, err := parseRunArgs(
		[]string{"-rc", "--yolo", "--prefer", "QM"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if showHelp || balance {
		t.Errorf("unexpected cx state: showHelp=%v balance=%v", showHelp, balance)
	}
	if prefer != "QM" {
		t.Errorf("prefer=%q, want QM", prefer)
	}
	want := []string{"-rc", "--dangerously-skip-permissions"}
	if len(claudeArgs) != len(want) {
		t.Fatalf("claudeArgs=%v, want %v", claudeArgs, want)
	}
	for i, v := range want {
		if claudeArgs[i] != v {
			t.Errorf("claudeArgs[%d]=%q, want %q", i, claudeArgs[i], v)
		}
	}
}
