package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/usage"
)

func TestShouldSwitch_BelowThreshold(t *testing.T) {
	est := usage.ExhaustionEstimate{CurrentPct: 50}
	if shouldSwitch(50, est, 80) {
		t.Error("expected false: 50% is below threshold 80%")
	}
}

func TestShouldSwitch_AboveThreshold(t *testing.T) {
	est := usage.ExhaustionEstimate{CurrentPct: 85}
	if !shouldSwitch(85, est, 80) {
		t.Error("expected true: 85% exceeds threshold 80%")
	}
}

func TestShouldSwitch_Exhausted(t *testing.T) {
	est := usage.ExhaustionEstimate{Exhausted: true, CurrentPct: 100}
	if !shouldSwitch(100, est, 80) {
		t.Error("expected true: account is exhausted")
	}
}

func TestShouldSwitch_PredictedExhaustSoon(t *testing.T) {
	est := usage.ExhaustionEstimate{
		CurrentPct:    70,
		TimeToExhaust: 10 * time.Minute,
	}
	if !shouldSwitch(70, est, 80) {
		t.Error("expected true: predicted to exhaust within 15m window")
	}
}

func TestDaemonCmd_OnceMode(t *testing.T) {
	dir := t.TempDir()

	resetsAt := time.Now().Add(2 * time.Hour).Unix()
	rc := &cache.RateCache{
		Account:   "testaccount",
		UpdatedAt: time.Now().Unix(),
		RateLimits: &cache.RateLimits{
			FiveHour: &cache.Window{
				UsedPercentage: 50.0,
				ResetsAt:       resetsAt,
			},
		},
	}
	if err := cache.WriteRateCache(filepath.Join(dir, "rate-cache.json"), rc); err != nil {
		t.Fatalf("WriteRateCache: %v", err)
	}

	var stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "testaccount",
			Accounts: map[string]config.Account{
				"testaccount": {ConfigDir: dir},
			},
		},
		Stdout:   &bytes.Buffer{},
		Stderr:   &stderr,
		UseColor: false,
	}

	cmd := &daemonCmd{}
	err := cmd.Run(context.Background(), app, []string{"--once"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := stderr.String()
	if !strings.Contains(out, "daemon") {
		t.Errorf("stderr missing 'daemon': %q", out)
	}
	if !strings.Contains(out, "Monitoring") {
		t.Errorf("stderr missing 'Monitoring': %q", out)
	}
}

func TestDaemonCmd_NoSwitchFlag(t *testing.T) {
	dir := t.TempDir()

	resetsAt := time.Now().Add(2 * time.Hour).Unix()
	rc := &cache.RateCache{
		Account:   "testaccount",
		UpdatedAt: time.Now().Unix(),
		RateLimits: &cache.RateLimits{
			FiveHour: &cache.Window{
				UsedPercentage: 50.0,
				ResetsAt:       resetsAt,
			},
		},
	}
	_ = cache.WriteRateCache(filepath.Join(dir, "rate-cache.json"), rc)

	var stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "testaccount",
			Accounts: map[string]config.Account{
				"testaccount": {ConfigDir: dir},
			},
		},
		Stdout:   &bytes.Buffer{},
		Stderr:   &stderr,
		UseColor: false,
	}

	cmd := &daemonCmd{}
	err := cmd.Run(context.Background(), app, []string{"--once", "--no-switch"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := stderr.String()
	if strings.Contains(out, "Best:") || strings.Contains(out, "SWITCH") {
		t.Errorf("--no-switch should suppress auto-switch output: %q", out)
	}
}

func TestDaemonCmd_Help(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Accounts: map[string]config.Account{}},
		Stdout:   &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &daemonCmd{}
	err := cmd.Run(context.Background(), app, []string{"--help"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "daemon") {
		t.Errorf("help text missing 'daemon': %q", buf.String())
	}
	if !strings.Contains(buf.String(), "--no-sync") {
		t.Errorf("help text missing '--no-sync': %q", buf.String())
	}
}
